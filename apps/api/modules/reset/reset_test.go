package reset

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"

	"github.com/go-chi/chi/v5"
)

// sentMail records what a request would have posted, so the tests can assert
// on delivery without an SMTP server.
type sentMail struct {
	to []string
}

func (m *sentMail) Send(_ context.Context, to, _, _ string) error {
	m.to = append(m.to, to)
	return nil
}

func server(t *testing.T) (*httptest.Server, *identity.Service, *sentMail) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	verifier := credentials.New(2)
	mail := &sentMail{}
	router := chi.NewRouter()
	resets := recovery.New(db, verifier, mail, "https://sso.example.test")
	MountWithLimits(router, New(resets, slog.New(slog.DiscardHandler)), 1000, 1000)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, identity.New(db, verifier), mail
}

// post sends one JSON body and returns the status and the response text.
func post(t *testing.T, srv *httptest.Server, path, body string) (int, string) {
	t.Helper()
	response, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("posting to %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	return response.StatusCode, strings.TrimSpace(string(raw))
}

// TestRequestAnswersIdenticallyForKnownAndUnknownAddresses is the endpoint's
// one security-critical property. SPEC §7: same response whether or not the
// address is known. A difference of a single byte or a single status code
// turns this route into a way to ask registre who has an account.
func TestRequestAnswersIdenticallyForKnownAndUnknownAddresses(t *testing.T) {
	srv, users, mail := server(t)

	_, err := users.Create(context.Background(), identity.NewUser{
		Email: "known@example.test", Password: "the current password",
	})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}

	knownStatus, knownBody := post(t, srv, "/api/recover", `{"email":"known@example.test"}`)
	unknownStatus, unknownBody := post(t, srv, "/api/recover", `{"email":"nobody@example.test"}`)

	if knownStatus != http.StatusOK || unknownStatus != http.StatusOK {
		t.Fatalf("statuses were %d and %d, want 200 and 200", knownStatus, unknownStatus)
	}
	if knownBody != unknownBody {
		t.Errorf("the answers differ, which names who has an account: %q vs %q", knownBody, unknownBody)
	}
	if len(mail.to) != 1 || mail.to[0] != "known@example.test" {
		t.Errorf("mail went to %v, want exactly the known address", mail.to)
	}
}

// TestRequestStillAnswersSentWhenTheBodyIsRubbish keeps the shape uniform for
// a caller probing with malformed input, which is otherwise a cheap way to
// tell a handled request from an unhandled one.
func TestRequestStillAnswersSentWhenTheBodyIsRubbish(t *testing.T) {
	srv, _, mail := server(t)

	status, body := post(t, srv, "/api/recover", `not json`)
	if status != http.StatusOK {
		t.Errorf("status %d, want 200", status)
	}
	if !strings.Contains(body, "sent") {
		t.Errorf("body %q, want the same sent answer every other call gets", body)
	}
	if len(mail.to) != 0 {
		t.Errorf("a malformed request sent %d mails", len(mail.to))
	}
}

// TestResetRefusesAnUnknownToken checks the other half answers one refusal.
func TestResetRefusesAnUnknownToken(t *testing.T) {
	srv, _, _ := server(t)

	status, _ := post(t, srv, "/api/recover/reset",
		`{"token":"0000000000000000","password":"a brand new password"}`)
	if status != http.StatusUnauthorized {
		t.Errorf("status %d, want 401", status)
	}
}

// TestResetRefusesAShortPassword stops the reset path from being the way
// around the floor every other password-setting path enforces.
func TestResetRefusesAShortPassword(t *testing.T) {
	srv, _, _ := server(t)

	status, body := post(t, srv, "/api/recover/reset", `{"token":"whatever","password":"short"}`)
	if status != http.StatusBadRequest {
		t.Errorf("status %d, want 400", status)
	}
	if !strings.Contains(body, "password_too_short") {
		t.Errorf("body %q does not say what was wrong", body)
	}
}
