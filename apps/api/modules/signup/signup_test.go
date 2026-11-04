package signup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	altcha "github.com/altcha-org/altcha-lib-go"
	"github.com/go-chi/chi/v5"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func call(t *testing.T, router http.Handler, method, path, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "192.0.2.1:1234"
	router.ServeHTTP(rec, request)
	return rec.Code, rec.Body.String()
}

// TestTheKillSwitchHidesEveryRoute: with sign-up off every route is a 404.
func TestTheKillSwitchHidesEveryRoute(t *testing.T) {
	router := chi.NewRouter()
	Mount(router, New(Deps{}, quiet()))
	for _, route := range [][2]string{
		{http.MethodGet, "/api/signup/challenge"}, {http.MethodPost, "/api/signup/start"},
		{http.MethodPost, "/api/signup/verify"}, {http.MethodPost, "/api/signup/passkey"},
		{http.MethodPost, "/api/signup/finish"},
	} {
		if status, _ := call(t, router, route[0], route[1], `{}`); status != http.StatusNotFound {
			t.Errorf("%s %s with sign-up off = %d, want 404", route[0], route[1], status)
		}
	}
}

// open builds the routes over a real flow and the given budgets.
func open(t *testing.T, budget Limits) http.Handler {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	db.Create(&schemas.User{ID: "known", Email: "known@example.com"})
	keys, _ := passkeys.New(db, passkeys.Config{RPID: "localhost", Origins: []string{"http://localhost"}}, quiet())
	verifier := credentials.New(2)
	mailer := recovery.MailerFunc(func(context.Context, string, string, string) error { return nil })
	steps := flow.New(flow.Deps{
		DB: db, Verifier: verifier, Consents: consent.New(db, []byte("k")), Passkeys: keys, Mailer: mailer,
	}, "an-altcha-hmac-key-of-at-least-32-bytes", "https://sso.example.com")
	router := chi.NewRouter()
	MountWithLimits(router, New(Deps{
		Flow: steps, Users: identity.New(db, verifier), Sessions: accounts.New(db), Cookies: accounts.NewCookies(false),
	}, quiet()), budget)
	return router
}

// proof fetches and solves a challenge over HTTP, as the page does.
func proof(t *testing.T, router http.Handler) string {
	t.Helper()
	_, body := call(t, router, http.MethodGet, "/api/signup/challenge", "")
	var challenge altcha.Challenge
	if err := json.Unmarshal([]byte(body), &challenge); err != nil {
		t.Fatalf("challenge: %v", err)
	}
	solution, _ := altcha.SolveChallenge(challenge.Challenge, challenge.Salt, altcha.SHA256, int(challenge.MaxNumber), 0, nil)
	payload, _ := json.Marshal(altcha.Payload{
		Algorithm: challenge.Algorithm, Challenge: challenge.Challenge, Number: int64(solution.Number),
		Salt: challenge.Salt, Signature: challenge.Signature,
	})
	return base64.StdEncoding.EncodeToString(payload)
}

// TestStartAnswersKnownAndUnknownIdentically compares the HTTP answers.
func TestStartAnswersKnownAndUnknownIdentically(t *testing.T) {
	router := open(t, DefaultLimits())
	var answers []string
	for _, email := range []string{"known@example.com", "unknown@example.com"} {
		status, body := call(t, router, http.MethodPost, "/api/signup/start",
			`{"email":"`+email+`","altcha":"`+proof(t, router)+`"}`)
		answers = append(answers, http.StatusText(status)+body)
	}
	if answers[0] != answers[1] || !strings.Contains(answers[0], "sent") {
		t.Fatalf("known %q, unknown %q: the answers must be identical", answers[0], answers[1])
	}
}

// TestStartIsLimitedPerEmailAndPerAddress.
func TestStartIsLimitedPerEmailAndPerAddress(t *testing.T) {
	budget := DefaultLimits()
	budget.EmailLimit, budget.EmailBurst = 0.0001, 1
	router := open(t, budget)
	start := func(email string) int {
		status, _ := call(t, router, http.MethodPost, "/api/signup/start", `{"email":"`+email+`","altcha":"x"}`)
		return status
	}
	if status := start("a@example.com"); status != http.StatusBadRequest {
		t.Fatalf("first start = %d, want 400 for the bad proof", status)
	}
	if status := start("A@example.com "); status != http.StatusTooManyRequests {
		t.Fatalf("second start for the same address = %d, want 429", status)
	}
	if status := start("b@example.com"); status != http.StatusBadRequest {
		t.Fatalf("another address = %d, want 400", status)
	}

	budget = DefaultLimits()
	budget.StartLimit, budget.StartBurst = 0.0001, 2
	router = open(t, budget)
	start("c@example.com")
	start("d@example.com")
	if status := start("e@example.com"); status != http.StatusTooManyRequests {
		t.Fatalf("third start from one address = %d, want 429", status)
	}
}
