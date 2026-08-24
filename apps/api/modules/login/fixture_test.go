package login

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/challenges"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
	otp "github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

// rootKey is the throwaway key the tests seal TOTP seeds under.
var rootKey = []byte("0123456789abcdef0123456789abcdef")

// fixture is the login endpoints over a real Postgres and a real chi router.
// Nothing is mocked: a test that stubbed the factor would prove only that the
// stub agrees with itself, and the bug this slice guards against is a
// redirect handed out on a password alone.
type fixture struct {
	t      *testing.T
	db     *gorm.DB
	users  *identity.Service
	codes  *totp.Service
	server *httptest.Server
}

// newFixture wires the whole login path with a budget wide enough that the
// rate limiter never fires. The budget is tested next door in
// ratelimit_test.go; here it would only make the attempt tests flaky.
func newFixture(t *testing.T) *fixture {
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
	users := identity.New(db, credentials.New(4))
	codes := totp.New(db, rootKey)
	service := New(users, slog.New(slog.DiscardHandler)).WithSecondFactor(SecondFactor{
		Challenges: challenges.New(db), Codes: codes,
		Accounts: accounts.New(db), Cookies: accounts.NewCookies(false),
	})
	router := chi.NewRouter()
	MountWithLimits(router, service, 1000, 1000)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &fixture{t: t, db: db, users: users, codes: codes, server: server}
}

// user creates an account with a password and returns it.
func (f *fixture) user(email, password string) schemas.User {
	f.t.Helper()
	user, err := f.users.Create(context.Background(), identity.NewUser{Email: email, Password: password})
	if err != nil {
		f.t.Fatalf("creating %s: %v", email, err)
	}
	return user
}

// enrol gives user a confirmed second factor and returns the seed and the
// recovery codes. It clears the burned time step afterwards: Confirm records
// the step it just proved, so the code that enrolled would otherwise be
// refused as a replay for the rest of its window.
func (f *fixture) enrol(user schemas.User) (string, []string) {
	f.t.Helper()
	ctx := context.Background()
	enrolment, err := f.codes.Begin(ctx, user, "registre-test")
	if err != nil {
		f.t.Fatalf("beginning enrolment: %v", err)
	}
	pending := f.reload(user.ID)
	recovery, err := f.codes.Confirm(ctx, pending, f.code(enrolment.Secret))
	if err != nil {
		f.t.Fatalf("confirming enrolment: %v", err)
	}
	err = f.db.Model(&schemas.User{}).Where("id = ?", user.ID).Update("totp_last_step", nil).Error
	if err != nil {
		f.t.Fatalf("clearing the burned step: %v", err)
	}
	return enrolment.Secret, recovery
}

// code is the six digits an authenticator app would be showing now.
func (f *fixture) code(secret string) string {
	f.t.Helper()
	code, err := otp.GenerateCode(secret, time.Now())
	if err != nil {
		f.t.Fatalf("generating a code: %v", err)
	}
	return code
}

// reload re-reads a user, which the TOTP service needs after every write to
// the columns it owns.
func (f *fixture) reload(id string) schemas.User {
	f.t.Helper()
	user, err := f.users.ByID(context.Background(), id)
	if err != nil {
		f.t.Fatalf("reading the user: %v", err)
	}
	return user
}

// authRequest inserts a pending browser flow and returns its id.
func (f *fixture) authRequest() string {
	f.t.Helper()
	row := schemas.AuthRequest{
		ID: "flow-" + time.Now().Format("150405.000000000"), ClientID: "app",
		RedirectURI: "https://app.test/callback", Scopes: schemas.StringList{"openid"},
		ResponseType: "code", ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}
	if err := f.db.Create(&row).Error; err != nil {
		f.t.Fatalf("creating the auth request: %v", err)
	}
	return row.ID
}

// post sends one JSON body and returns the decoded answer and the response.
func (f *fixture) post(path string, body map[string]string) (map[string]string, *http.Response) {
	f.t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		f.t.Fatalf("encoding the body: %v", err)
	}
	response, err := http.Post(f.server.URL+path, "application/json", bytes.NewReader(encoded))
	if err != nil {
		f.t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		f.t.Fatalf("reading the body: %v", err)
	}
	answer := map[string]string{}
	if err := json.Unmarshal(raw, &answer); err != nil {
		f.t.Fatalf("POST %s returned %d and unparseable body %q", path, response.StatusCode, raw)
	}
	return answer, response
}

// login submits the password step.
func (f *fixture) login(id, email, password string) (map[string]string, *http.Response) {
	f.t.Helper()
	return f.post("/api/login", map[string]string{"auth_request": id, "email": email, "password": password})
}

// second submits the code step.
func (f *fixture) second(challenge, code string) (map[string]string, *http.Response) {
	f.t.Helper()
	return f.post("/api/login/totp", map[string]string{"challenge": challenge, "code": code})
}

// done reports whether the browser flow has been authenticated.
func (f *fixture) done(id string) bool {
	f.t.Helper()
	var row schemas.AuthRequest
	if err := f.db.Where("id = ?", id).Take(&row).Error; err != nil {
		f.t.Fatalf("reading the auth request: %v", err)
	}
	return row.Done()
}

// cookie returns the account session cookie the response set, or nil.
func cookie(response *http.Response) *http.Cookie {
	for _, set := range response.Cookies() {
		if set.Name == accounts.DevCookieName && set.Value != "" {
			return set
		}
	}
	return nil
}
