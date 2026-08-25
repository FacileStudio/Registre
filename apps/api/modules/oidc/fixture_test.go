package oidc

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/modules/login"

	"github.com/go-chi/chi/v5"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

// rootKey is the throwaway encryption key the tests seal signing keys under.
var rootKey = []byte("0123456789abcdef0123456789abcdef")

// fixture is a whole running provider: real Postgres, real keys, real HTTP.
//
// Nothing here is a mock. The point of these tests is that a token registre
// signs verifies against the key set registre serves, and a fake of either half
// would prove only that the fake agrees with itself.
//
// It calls slog.SetDefault for the same reason main does: zitadel/oidc emits
// its protocol errors through the package-level slog and discards any logger it
// is handed, so without this every refusal a test deliberately provokes prints
// to the test output in slog's own format.
type fixture struct {
	t        *testing.T
	db       *gorm.DB
	keys     *keys.Service
	register *clients.Service
	users    *identity.Service
	devices  *devices.Service
	storage  *Storage
	provider *op.Provider
	server   *httptest.Server
	issuer   string
	clock    *clock
}

// clock is the shared, movable now the fixture hands to every service. It is
// locked because the HTTP handler reads it on the server's goroutine while the
// test writes it on its own.
//
// It starts at the real time rather than a fixed date, which is the opposite of
// what a deterministic fixture usually wants. zitadel/oidc computes the token
// response's expires_in as `exp - time.Now()` and takes no clock, so only half
// the pair can be injected: a fixture pinned to a literal date drifts past it,
// validity goes negative, and uint64(negative) wraps to ~1.8e19. That is a test
// that passes when it is written and fails hours later for no reason anyone
// changed.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
}

func newFixture(t *testing.T, cfg keys.Config) *fixture {
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
	ticking := &clock{at: time.Now().UTC()}

	cfg.Now = ticking.now
	keyring := keys.New(db, rootKey, cfg, logs)
	if err := keyring.Ensure(context.Background()); err != nil {
		t.Fatalf("minting the first key: %v", err)
	}
	register := clients.New(db, credentials.New(4))
	users := identity.New(db, credentials.New(4))
	deviceFlows := devices.New(db).WithClock(ticking.now)
	storage := NewStorage(db, Services{
		Keys: keyring, Register: register, Users: users,
		Logout:  logout.New(keyring, register, issuer, logs),
		Devices: deviceFlows,
	}, Config{
		Issuer: issuer, AccessTokenTTL: time.Hour,
		RefreshTokenTTL: 24 * time.Hour, Now: ticking.now,
	}, logs)

	provider, err := NewProvider(storage, ProviderConfig{Issuer: issuer, RootKey: rootKey, Insecure: true})
	if err != nil {
		t.Fatalf("building the provider: %v", err)
	}
	router := chi.NewRouter()
	Mount(router, provider)
	login.Mount(router, login.New(users, logs))
	router.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		// The real page lives in apps/client; production mounts it through
		// web.Handler. What the flow needs from it is exactly this: render
		// the pending request id so the credentials come back tagged with
		// it. Without it every browser-flow test dies on the redirect to
		// the login page, which is also how those skips hid so long.
		_, _ = w.Write([]byte(
			`<form method="post"><input type="hidden" name="auth_request" value="` +
				r.URL.Query().Get("auth_request") + `"></form>`))
	})
	server.Config.Handler = router
	server.Start()
	t.Cleanup(server.Close)

	return &fixture{
		t: t, db: db, keys: keyring, register: register, users: users, devices: deviceFlows,
		storage: storage, provider: provider, server: server, issuer: issuer, clock: ticking,
	}
}

// defaultKeyConfig is a lifecycle with room to be driven by hand: nothing
// rotates on its own during a test.
func defaultKeyConfig() keys.Config {
	return keys.Config{RotateEvery: time.Hour, PublishDelay: time.Minute, Retention: 2 * time.Hour}
}

// tokenResponse is the successful half of a token endpoint answer.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   uint64 `json:"expires_in"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// token asks the token endpoint for a client_credentials grant.
func (f *fixture) token(clientID, secret, scope string) (tokenResponse, int) {
	f.t.Helper()
	form := url.Values{"grant_type": {"client_credentials"}}
	if scope != "" {
		form.Set("scope", scope)
	}
	request, err := http.NewRequest(http.MethodPost, f.issuer+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		f.t.Fatalf("building the token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(clientID, secret)
	return decode[tokenResponse](f.t, request)
}

// postForm runs a form request with no client authentication at all, which is
// how a public client talks to the token and device authorization endpoints.
func (f *fixture) postForm(path string, form url.Values) (map[string]any, int) {
	f.t.Helper()
	request, err := http.NewRequest(http.MethodPost, f.issuer+path, strings.NewReader(form.Encode()))
	if err != nil {
		f.t.Fatalf("building the %s request: %v", path, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return decode[map[string]any](f.t, request)
}

// jwks fetches the published key set.
func (f *fixture) jwks() jose.JSONWebKeySet {
	f.t.Helper()
	request, err := http.NewRequest(http.MethodGet, f.issuer+"/keys", nil)
	if err != nil {
		f.t.Fatalf("building the JWKS request: %v", err)
	}
	set, status := decode[jose.JSONWebKeySet](f.t, request)
	if status != http.StatusOK {
		f.t.Fatalf("GET /keys = %d", status)
	}
	return set
}

// verify checks a token against the key set the provider is publishing now,
// and returns its claims. This is the assertion the whole repository exists to
// make true, so it goes through the same JWKS a relying party would fetch.
func (f *fixture) verify(token string) (map[string]any, error) {
	f.t.Helper()
	signature, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, err
	}
	set := f.jwks()
	payload, err := signature.Verify(&set)
	if err != nil {
		return nil, err
	}
	claims := map[string]any{}
	return claims, json.Unmarshal(payload, &claims)
}

// rotate moves the clock and drives one step of the key lifecycle.
func (f *fixture) rotate(at time.Time) {
	f.t.Helper()
	f.clock.set(at)
	if err := f.keys.Rotate(context.Background()); err != nil {
		f.t.Fatalf("rotating at %s: %v", at, err)
	}
}

// activeKID is the kid of the key signing right now.
func (f *fixture) activeKID() string {
	f.t.Helper()
	signing, err := f.keys.Signing(context.Background())
	if err != nil {
		f.t.Fatalf("reading the active key: %v", err)
	}
	return signing.ID()
}

// decode runs a request and unmarshals the body, whatever the status.
func decode[T any](t *testing.T, request *http.Request) (T, int) {
	t.Helper()
	var value T
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", request.Method, request.URL, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("%s %s returned %d and unparseable body %q", request.Method, request.URL, response.StatusCode, body)
	}
	return value, response.StatusCode
}

// post runs a form request at path with client credentials in the basic auth
// header, which is how the token, introspection and revocation endpoints all
// authenticate a caller.
func (f *fixture) post(path, clientID, secret string, form url.Values) (map[string]any, int) {
	f.t.Helper()
	request, err := http.NewRequest(http.MethodPost, f.issuer+path, strings.NewReader(form.Encode()))
	if err != nil {
		f.t.Fatalf("building the %s request: %v", path, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(clientID, secret)
	return decode[map[string]any](f.t, request)
}

// introspect asks what a token is, as clientID.
func (f *fixture) introspect(clientID, secret, token string) (map[string]any, int) {
	f.t.Helper()
	return f.post("/oauth/introspect", clientID, secret, url.Values{"token": {token}})
}

// revoke tries to take a token back, as clientID. It reads the status and not
// the body: RFC 7009 §2.2 says a successful revocation answers 200 with no
// content, so there is nothing to parse.
func (f *fixture) revoke(clientID, secret, token string) int {
	f.t.Helper()
	form := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
	request, err := http.NewRequest(http.MethodPost, f.issuer+"/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		f.t.Fatalf("building the revocation request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(clientID, secret)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		f.t.Fatalf("POST /revoke: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
}
