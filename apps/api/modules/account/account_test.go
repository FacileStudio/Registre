package account

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
	otptotp "github.com/pquerna/otp/totp"
)

// root is the throwaway key the tests seal TOTP seeds under.
var root = []byte("0123456789abcdef0123456789abcdef")

// holderEmail and holderPassword are the fixture account's credentials.
const (
	holderEmail    = "human@example.com"
	holderPassword = "the holder's password"
)

type fixture struct {
	router  chi.Router
	users   *identity.Service
	factors *totp.Service
	cookies accounts.Cookies
	user    string
	token   string
}

func newFixture(t *testing.T) fixture {
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
	users := identity.New(db, verifier)
	factors := totp.New(db, root)
	sessions := accounts.New(db)
	cookies := accounts.NewCookies(false)
	router := chi.NewRouter()
	Mount(router, New(Services{
		Sessions: sessions, TOTP: factors, Users: users, Passwords: verifier,
		DB: db, Cookies: cookies, Issuer: "registre",
	}, slog.New(slog.DiscardHandler)))

	ctx := context.Background()
	user, err := users.Create(ctx, identity.NewUser{Email: holderEmail, Password: holderPassword})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	sessionID, err := users.StartSession(ctx, user.ID, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("opening the session: %v", err)
	}
	token, err := sessions.Issue(ctx, sessionID)
	if err != nil {
		t.Fatalf("issuing the cookie: %v", err)
	}
	return fixture{router, users, factors, cookies, user.ID, token}
}

// request builds a JSON request for path, carrying no credentials.
func request(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encoding the body: %v", err)
	}
	return httptest.NewRequest(method, path, bytes.NewReader(raw))
}

func (f fixture) cookie(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: f.cookies.Name(), Value: f.token})
	return req
}

// send runs a request the real page would send: the cookie and the CSRF header.
func (f fixture) send(t *testing.T, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	req := f.cookie(request(t, method, path, body))
	req.Header.Set(accounts.CSRFHeader, "1")
	return f.do(t, req)
}

func (f fixture) do(t *testing.T, req *http.Request) (*http.Response, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req)
	response := recorder.Result()
	var decoded map[string]any
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		decoded = nil
	}
	return response, decoded
}

// reload reads the fixture account back, so a test can see what a refusal left.
func (f fixture) reload(t *testing.T) schemas.User {
	t.Helper()
	user, err := f.users.ByID(context.Background(), f.user)
	if err != nil {
		t.Fatalf("reading the account: %v", err)
	}
	return user
}

// enrol runs begin and confirm and returns the codes confirmation showed.
func (f fixture) enrol(t *testing.T) []string {
	t.Helper()
	response, body := f.send(t, http.MethodPost, "/api/account/totp/begin", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("beginning enrolment answered %d: %v", response.StatusCode, body)
	}
	secret, _ := body["secret"].(string)
	if secret == "" || body["qr_png"] == "" || body["uri"] == "" {
		t.Fatalf("enrolment is missing a seed, a URI or a QR: %v", body)
	}
	code, err := otptotp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generating a code: %v", err)
	}
	response, body = f.send(t, http.MethodPost, "/api/account/totp/confirm", map[string]any{"code": code})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("confirming answered %d: %v", response.StatusCode, body)
	}
	return recoveryCodes(t, body)
}

// recoveryCodes reads the set a response showed once and asserts it is full: a
// factor with no codes is one lost phone away from needing an operator.
func recoveryCodes(t *testing.T, body map[string]any) []string {
	t.Helper()
	raw, _ := body["recovery_codes"].([]any)
	if len(raw) != totp.RecoveryCodeCount {
		t.Fatalf("the response carried %d recovery codes, want %d", len(raw), totp.RecoveryCodeCount)
	}
	codes := make([]string, 0, len(raw))
	for _, one := range raw {
		codes = append(codes, one.(string))
	}
	return codes
}

// passwords is a password-change body.
func passwords(current, next string) map[string]any {
	return map[string]any{"current_password": current, "new_password": next}
}

func TestRequestsWithoutACookieOrTheCSRFHeaderAreRefused(t *testing.T) {
	f := newFixture(t)

	response, _ := f.do(t, request(t, http.MethodGet, "/api/account", nil))
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no cookie answered %d, want 401", response.StatusCode)
	}
	response, _ = f.do(t, f.cookie(request(t, http.MethodPost, "/api/account/totp/begin", nil)))
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("a write with no CSRF header answered %d, want 403", response.StatusCode)
	}
	if len(f.reload(t).TOTPSecret) != 0 {
		t.Fatal("the refused write still stored an enrolment seed")
	}
	response, _ = f.do(t, f.cookie(request(t, http.MethodGet, "/api/account", nil)))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("a GET without the CSRF header answered %d, want 200", response.StatusCode)
	}
}

func TestEnrolmentRunsFromBeginToRecoveryCodesAndCannotBeRebegun(t *testing.T) {
	f := newFixture(t)
	f.enrol(t)

	response, body := f.send(t, http.MethodGet, "/api/account", nil)
	if enrolled, _ := body["totp_enrolled"].(bool); !enrolled {
		t.Fatalf("the account view does not show the factor: %v", body)
	}
	if left, _ := body["recovery_codes_remaining"].(float64); int(left) != totp.RecoveryCodeCount {
		t.Fatalf("the account view counts %v recovery codes", body["recovery_codes_remaining"])
	}
	response, _ = f.send(t, http.MethodPost, "/api/account/totp/begin", nil)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("re-beginning a confirmed factor answered %d, want 409", response.StatusCode)
	}
}

func TestLoggingOutTwiceAnswersAndClearsTheCookieBothTimes(t *testing.T) {
	f := newFixture(t)

	for attempt := range 2 {
		response, _ := f.send(t, http.MethodPost, "/api/account/logout", nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("logout %d answered %d, want 200", attempt+1, response.StatusCode)
		}
		cleared := false
		for _, cookie := range response.Cookies() {
			if cookie.Name == f.cookies.Name() && cookie.MaxAge < 0 {
				cleared = true
			}
		}
		if !cleared {
			t.Fatalf("logout %d did not clear the cookie", attempt+1)
		}
	}
	response, _ := f.send(t, http.MethodGet, "/api/account", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("the ended session still answered %d", response.StatusCode)
	}
}

func TestChangingThePasswordWantsTheCurrentOne(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	path := "/api/account/password"

	response, _ := f.send(t, http.MethodPost, path, passwords("the wrong password", "a replacement"))
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a wrong current password answered %d, want 401", response.StatusCode)
	}
	if _, err := f.users.Verify(ctx, holderEmail, holderPassword); err != nil {
		t.Fatalf("the refused change replaced the password anyway: %v", err)
	}
	response, body := f.send(t, http.MethodPost, path, passwords(holderPassword, "short"))
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("a password under the floor answered %d: %v", response.StatusCode, body)
	}
	response, body = f.send(t, http.MethodPost, path, passwords(holderPassword, "a replacement password"))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("changing the password answered %d: %v", response.StatusCode, body)
	}
	if _, err := f.users.Verify(ctx, holderEmail, "a replacement password"); err != nil {
		t.Fatalf("the new password does not verify: %v", err)
	}
}
