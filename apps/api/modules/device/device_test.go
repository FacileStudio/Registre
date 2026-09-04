package device

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

// fixture is the verification API over a real database, a real account
// session and a real cookie, because every one of those is part of what the
// routes assert. A mocked session would prove the mock lets people in.
type fixture struct {
	t        *testing.T
	server   *httptest.Server
	flows    *devices.Service
	users    *identity.Service
	sessions *accounts.Service
	cookies  accounts.Cookies
	cookie   string
}

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
	verifier := credentials.New(2)
	flows := devices.New(db)
	sessions := accounts.New(db)
	cookies := accounts.NewCookies(false)
	router := chi.NewRouter()
	Mount(router, New(Services{
		Devices: flows, Sessions: sessions, Register: clients.New(db, verifier), Cookies: cookies,
	}, slog.New(slog.DiscardHandler)))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &fixture{
		t: t, server: server, flows: flows, sessions: sessions,
		users: identity.New(db, verifier), cookies: cookies,
	}
}

// signIn creates a human, opens the provider session login would have opened,
// and keeps the account cookie that hangs off it.
func (f *fixture) signIn() schemas.User {
	f.t.Helper()
	user, err := f.users.Create(context.Background(), identity.NewUser{
		Email: "yann@facile.studio", Password: "correct horse battery staple", Name: "Yann",
	})
	if err != nil {
		f.t.Fatalf("creating the human: %v", err)
	}
	sessionID, err := f.users.StartSession(context.Background(), user.ID, "sablier", time.Now().UTC())
	if err != nil {
		f.t.Fatalf("opening the session: %v", err)
	}
	token, err := f.sessions.Issue(context.Background(), sessionID)
	if err != nil {
		f.t.Fatalf("issuing the account cookie: %v", err)
	}
	f.cookie = token
	return user
}

// start records a device flow to answer.
func (f *fixture) start(userCode string) {
	f.t.Helper()
	err := f.flows.Store(
		context.Background(), devices.StoreParams{
			ClientID:   "facile-cli",
			DeviceCode: "device-" + userCode,
			UserCode:   userCode,
			ExpiresAt:  time.Now().UTC().Add(devices.Lifetime),
			Scopes:     []string{"openid", "offline_access"},
		})
	if err != nil {
		f.t.Fatalf("recording the flow: %v", err)
	}
}

// call runs one request against the verification API, with the cookie and the
// CSRF header when the fixture is signed in.
func (f *fixture) call(method, path, body string) (map[string]any, int) {
	f.t.Helper()
	request, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		f.t.Fatalf("building %s %s: %v", method, path, err)
	}
	request.Header.Set("Content-Type", "application/json")
	if method != http.MethodGet {
		request.Header.Set(accounts.CSRFHeader, "1")
	}
	if f.cookie != "" {
		request.AddCookie(&http.Cookie{Name: f.cookies.Name(), Value: f.cookie})
	}
	return f.do(request)
}

func (f *fixture) do(request *http.Request) (map[string]any, int) {
	f.t.Helper()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		f.t.Fatalf("%s %s: %v", request.Method, request.URL, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		f.t.Fatalf("reading the body: %v", err)
	}
	decoded := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return decoded, response.StatusCode
}

// TestApprovingAFlowNeedsAnAccountSession is the whole security model of this
// module in one assertion. A device flow is approved by a named human, and the
// token that follows carries their subject and their roles; an anonymous
// approval would mint a credential nobody is accountable for.
func TestApprovingAFlowNeedsAnAccountSession(t *testing.T) {
	fixture := newFixture(t)
	fixture.start("BCDFGHJK")

	if _, status := fixture.call(http.MethodGet, "/api/device?user_code=BCDF-GHJK", ""); status != http.StatusUnauthorized {
		t.Errorf("reading a pending flow without a cookie = %d, want 401", status)
	}
	body := `{"user_code":"BCDF-GHJK"}`
	if _, status := fixture.call(http.MethodPost, "/api/device/approve", body); status != http.StatusUnauthorized {
		t.Errorf("approving without a cookie = %d, want 401", status)
	}

	grant, err := fixture.flows.Poll(context.Background(), "facile-cli", "device-BCDFGHJK")
	if err != nil {
		t.Fatalf("polling: %v", err)
	}
	if grant.Approved {
		t.Fatal("the flow was approved by a request that carried no session")
	}
}

// TestApprovingWithoutTheCSRFHeaderIsRefused covers the cross-site case the
// cookie alone cannot: the browser sends it whether or not the page that
// triggered the request was ours.
func TestApprovingWithoutTheCSRFHeaderIsRefused(t *testing.T) {
	fixture := newFixture(t)
	fixture.signIn()
	fixture.start("BCDFGHJK")

	request, err := http.NewRequest(
		http.MethodPost, fixture.server.URL+"/api/device/approve",
		strings.NewReader(`{"user_code":"BCDFGHJK"}`),
	)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: fixture.cookies.Name(), Value: fixture.cookie})
	if _, status := fixture.do(request); status != http.StatusForbidden {
		t.Fatalf("approving without X-Facile-CSRF = %d, want 403", status)
	}
}

// TestAHumanSeesWhatTheyAreApprovingAndThenApprovesIt walks the page's whole
// job: read the flow, say yes, and have the CLI's next poll find a grant that
// names the human who said it.
func TestAHumanSeesWhatTheyAreApprovingAndThenApprovesIt(t *testing.T) {
	fixture := newFixture(t)
	user := fixture.signIn()
	fixture.start("BCDFGHJK")

	view, status := fixture.call(http.MethodGet, "/api/device?user_code=bcdf-ghjk", "")
	if status != http.StatusOK {
		t.Fatalf("GET /api/device = %d: %v", status, view)
	}
	if view["client_id"] != "facile-cli" {
		t.Errorf("the page is told the client is %v, want facile-cli", view["client_id"])
	}
	scopes, _ := view["scopes"].([]any)
	if len(scopes) != 2 {
		t.Errorf("the page is shown %v, want the two scopes the flow asked for", view["scopes"])
	}

	answered, status := fixture.call(http.MethodPost, "/api/device/approve", `{"user_code":"bcdf ghjk"}`)
	if status != http.StatusOK {
		t.Fatalf("POST /api/device/approve = %d: %v", status, answered)
	}

	grant, err := fixture.flows.Poll(context.Background(), "facile-cli", "device-BCDFGHJK")
	if err != nil {
		t.Fatalf("polling: %v", err)
	}
	if !grant.Approved || grant.Subject != user.ID {
		t.Fatalf("the poll found %+v, want an approval naming %s", grant, user.ID)
	}
	if grant.AuthTime.IsZero() {
		t.Error("the grant carries no auth_time, so the token would claim the human never signed in")
	}
}

// TestDenyingAFlowAnswersIt covers the button that matters when a code turns
// up unexpectedly.
func TestDenyingAFlowAnswersIt(t *testing.T) {
	fixture := newFixture(t)
	fixture.signIn()
	fixture.start("BCDFGHJK")

	if _, status := fixture.call(http.MethodPost, "/api/device/deny", `{"user_code":"BCDFGHJK"}`); status != http.StatusOK {
		t.Fatalf("POST /api/device/deny = %d", status)
	}
	grant, err := fixture.flows.Poll(context.Background(), "facile-cli", "device-BCDFGHJK")
	if err != nil {
		t.Fatalf("polling: %v", err)
	}
	if !grant.Denied {
		t.Fatalf("the poll found %+v, want denied", grant)
	}
}

// TestAnUnknownCodeAnswersTheSameAsASpentOne keeps the endpoint from being an
// oracle. A guesser who could tell "no such code" from "already used" would
// learn the only thing worth learning from guessing here.
func TestAnUnknownCodeAnswersTheSameAsASpentOne(t *testing.T) {
	fixture := newFixture(t)
	fixture.signIn()
	fixture.start("BCDFGHJK")
	if _, status := fixture.call(http.MethodPost, "/api/device/deny", `{"user_code":"BCDFGHJK"}`); status != http.StatusOK {
		t.Fatalf("answering the flow first: %d", status)
	}

	answered, spent := fixture.call(http.MethodGet, "/api/device?user_code=BCDFGHJK", "")
	invented, unknown := fixture.call(http.MethodGet, "/api/device?user_code=KLMNPQRS", "")
	if spent != http.StatusNotFound || unknown != http.StatusNotFound {
		t.Fatalf("an answered code = %d and an invented one = %d, want 404 for both", spent, unknown)
	}
	if answered["error"] != invented["error"] {
		t.Errorf("the two answers differ: %v vs %v", answered, invented)
	}
}

// TestTheVerificationRoutesRatelimitPerAddress is RFC 8628 §5.2's requirement
// made real. A user code is 34.6 bits, which is only enough because a guesser
// gets a few thousand attempts inside its whole ten-minute life rather than as
// many as their connection allows.
func TestTheVerificationRoutesRatelimitPerAddress(t *testing.T) {
	router := chi.NewRouter()
	MountWithLimits(router, &Service{}, 1, 2)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	for i := 0; i < 2; i++ {
		response, err := http.Get(server.URL + "/api/device?user_code=BCDFGHJK")
		if err != nil {
			t.Fatalf("GET %d: %v", i, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("request %d = %d, want the 401 from the absent cookie", i+1, response.StatusCode)
		}
	}
	response, err := http.Get(server.URL + "/api/device?user_code=BCDFGHJK")
	if err != nil {
		t.Fatalf("the refused request: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request 3 = %d, want 429", response.StatusCode)
	}
	if response.Header.Get("Retry-After") == "" {
		t.Error("a refusal without Retry-After leaves the client guessing when to come back")
	}
}
