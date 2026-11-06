package device

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/modules/login"
	"github.com/FacileStudio/Registre/apps/api/modules/oidc"

	"github.com/go-chi/chi/v5"
)

// loop is the whole sign-in round trip standing up: a real provider, the real
// login backend, the real verification API, and registre registered as a
// client of itself.
//
// It is heavier than the other fixture in this package on purpose. The claim
// under test is that a human with no session can get one and come back, and
// every hop of that is somebody else's code — /authorize, /login,
// login.complete — so a fixture that stubbed any of them would be asserting
// the stub.
type loop struct {
	t      *testing.T
	server *httptest.Server
	issuer string
	flows  *devices.Service
	client *http.Client
}

func newLoop(t *testing.T) *loop {
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
	logs := slog.New(slog.DiscardHandler)
	slog.SetDefault(logs)
	server := httptest.NewUnstartedServer(nil)
	issuer := "http://" + server.Listener.Addr().String()

	verifier := credentials.New(2)
	keyring := keys.New(db, []byte("0123456789abcdef0123456789abcdef"), keys.Config{
		RotateEvery: time.Hour, PublishDelay: time.Minute, Retention: 2 * time.Hour,
	}, logs)
	if err := keyring.Ensure(context.Background()); err != nil {
		t.Fatalf("minting the first key: %v", err)
	}
	register := clients.New(db, verifier)
	if err := register.EnsureSelf(context.Background(), issuer+devices.VerificationPath); err != nil {
		t.Fatalf("registering registre with itself: %v", err)
	}
	users := identity.New(db, verifier)
	if _, err := users.Create(context.Background(), identity.NewUser{
		Email: "yann@facile.studio", Password: "correct horse battery staple", Name: "Yann",
	}); err != nil {
		t.Fatalf("creating the human: %v", err)
	}
	flows := devices.New(db)
	sessions := accounts.New(db)
	cookies := accounts.NewCookies(false)

	storage := oidc.NewStorage(db, oidc.Services{
		Keys: keyring, Register: register, Users: users, Devices: flows,
	}, oidc.Config{Issuer: issuer, AccessTokenTTL: time.Hour}, logs)
	provider, err := oidc.NewProvider(storage, oidc.ProviderConfig{
		Issuer: issuer, RootKey: []byte("0123456789abcdef0123456789abcdef"), Insecure: true,
	})
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}
	router := chi.NewRouter()
	oidc.Mount(router, provider, oidc.Binding{Storage: storage, Sessions: sessions, Cookies: cookies})
	login.Mount(router, login.New(users, logs).WithSecondFactor(login.SecondFactor{
		Accounts: sessions, Cookies: cookies,
	}))
	Mount(router, New(Services{
		Devices: flows, Sessions: sessions, Register: register, Cookies: cookies,
		AuthorizeURL: provider.AuthorizationEndpoint().Absolute(issuer),
		RedirectURI:  issuer + devices.VerificationPath,
	}, logs))
	mountStubPages(router)
	server.Config.Handler = router
	server.Start()
	t.Cleanup(server.Close)

	return &loop{
		t: t, server: server, issuer: issuer, flows: flows,
		client: &http.Client{Jar: newJar()},
	}
}

// mountStubPages stands in for apps/client, which is not built during a Go
// test. The login form has to echo the pending request id back, and /device
// has to answer at all so the last redirect of the round trip lands somewhere.
func mountStubPages(router chi.Router) {
	router.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<input name="auth_request" value="` + r.URL.Query().Get("auth_request") + `">`))
	})
	router.Get(devices.VerificationPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("the verification page"))
	})
}

// newJar is a cookie jar that keeps everything, so the test can assert on the
// session the login writes rather than on a redirect chain.
func newJar() http.CookieJar {
	jar := &memoryJar{}
	return jar
}

type memoryJar struct{ held []*http.Cookie }

func (j *memoryJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.held = append(j.held, cookies...)
}
func (j *memoryJar) Cookies(*url.URL) []*http.Cookie { return j.held }

// TestASessionlessHumanCanSignInAndComeBack is the round trip the verification
// page could not make before this route existed.
//
// The CLI prints a URL and a code, the human opens it on a phone, and the phone
// is the device least likely to already hold a registre session. Before this,
// the only answer was "go and sign in to some other Facile app, then come
// back", which is a workaround printed on a screen rather than a login.
func TestASessionlessHumanCanSignInAndComeBack(t *testing.T) {
	fixture := newLoop(t)
	if err := fixture.flows.Store(context.Background(), devices.StoreParams{
		ClientID:   "facile-cli",
		DeviceCode: "device-1",
		UserCode:   "BCDFGHJK",
		ExpiresAt:  time.Now().UTC().Add(devices.Lifetime),
		Scopes:     []string{"openid"},
	}); err != nil {
		t.Fatalf("recording the flow: %v", err)
	}

	if _, status := fixture.get("/api/device?user_code=BCDFGHJK"); status != http.StatusUnauthorized {
		t.Fatalf("the page starts signed in already, so this proves nothing: %d", status)
	}

	landed, status := fixture.get("/api/device/authenticate")
	if status != http.StatusOK {
		t.Fatalf("following the sign-in redirect stopped at %d (%s)", status, landed)
	}
	fixture.signIn(landed)

	body, status := fixture.get("/api/device?user_code=BCDFGHJK")
	if status != http.StatusOK {
		t.Fatalf("after signing in, GET /api/device = %d: %s", status, body)
	}
	if !strings.Contains(body, `"client_id":"facile-cli"`) {
		t.Errorf("the page was shown %q, want the pending flow", body)
	}
}

// signIn posts the credentials the stub login page is holding a request id for,
// then follows the callback the way a browser would.
func (l *loop) signIn(loginPage string) {
	l.t.Helper()
	marker := `value="`
	start := strings.Index(loginPage, marker)
	if start < 0 {
		l.t.Fatalf("the login page carries no auth request: %s", loginPage)
	}
	rest := loginPage[start+len(marker):]
	id := rest[:strings.Index(rest, `"`)]

	payload, err := json.Marshal(map[string]string{
		"auth_request": id, "email": "yann@facile.studio",
		"password": "correct horse battery staple",
	})
	if err != nil {
		l.t.Fatalf("encoding the credentials: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, l.issuer+"/api/login", strings.NewReader(string(payload)))
	if err != nil {
		l.t.Fatalf("building POST /api/login: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Facile-CSRF", "1")
	response, err := l.client.Do(request)
	if err != nil {
		l.t.Fatalf("POST /api/login: %v", err)
	}
	var decision struct {
		Redirect string `json:"redirect"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decision); err != nil {
		l.t.Fatalf("reading the decision: %v", err)
	}
	_ = response.Body.Close()
	if decision.Redirect == "" {
		l.t.Fatal("the login answered no redirect")
	}
	if _, status := l.get(decision.Redirect); status != http.StatusOK {
		l.t.Fatalf("the callback landed on %d", status)
	}
}

// get follows redirects the way a browser does and returns the body it ends on.
func (l *loop) get(path string) (string, int) {
	l.t.Helper()
	response, err := l.client.Get(l.issuer + path)
	if err != nil {
		l.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	body := make([]byte, 4096)
	read, _ := response.Body.Read(body)
	return string(body[:read]), response.StatusCode
}

// TestNothingInTheSignInRedirectComesFromTheRequest is the open-redirect
// assertion, and it is the property that makes this route safe rather than
// merely convenient.
//
// Every part of the destination is process configuration. The moment any of it
// were taken from the caller — a return_to, a next, a redirect_uri — this would
// stop being a redirect and start being an open redirector, and registre is the
// one host in the suite where that is worth the most to an attacker.
func TestNothingInTheSignInRedirectComesFromTheRequest(t *testing.T) {
	fixture := newLoop(t)
	clean := fixture.signInTarget("/api/device/authenticate")
	forged := fixture.signInTarget(
		"/api/device/authenticate?redirect_uri=https://evil.example/&return_to=https://evil.example/" +
			"&next=https://evil.example/&client_id=evil&state=x",
	)

	if clean.Host != forged.Host || clean.Path != forged.Path {
		t.Fatalf("the destination moved from %s to %s", clean, forged)
	}
	for _, key := range []string{"client_id", "redirect_uri", "response_type", "scope"} {
		if clean.Query().Get(key) != forged.Query().Get(key) {
			t.Errorf("%s = %q with a forged request, want %q",
				key, forged.Query().Get(key), clean.Query().Get(key))
		}
	}
	if want := fixture.issuer + devices.VerificationPath; clean.Query().Get("redirect_uri") != want {
		t.Errorf("redirect_uri = %q, want %s", clean.Query().Get("redirect_uri"), want)
	}
	if clean.Query().Get("client_id") != clients.SelfClientID {
		t.Errorf("client_id = %q, want %s", clean.Query().Get("client_id"), clients.SelfClientID)
	}
	if clean.Query().Get("code_challenge") == "" {
		t.Error("no PKCE challenge, so the authorization code this leaves behind is redeemable")
	}
	if clean.Query().Get("code_challenge") == forged.Query().Get("code_challenge") {
		t.Error("the PKCE challenge is reused between requests")
	}
}

// signInTarget reads the Location the sign-in route answers with, without
// following it.
func (l *loop) signInTarget(path string) *url.URL {
	l.t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	response, err := client.Get(l.issuer + path)
	if err != nil {
		l.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusFound {
		l.t.Fatalf("GET %s = %d, want 302", path, response.StatusCode)
	}
	target, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		l.t.Fatalf("the Location is not a URL: %v", err)
	}
	return target
}
