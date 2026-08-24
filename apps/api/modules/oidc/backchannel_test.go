package oidc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// logoutInbox is a stand-in relying party's back-channel endpoint.
type logoutInbox struct {
	server *httptest.Server
	mu     sync.Mutex
	bodies [][]byte
}

func newLogoutInbox(t *testing.T) *logoutInbox {
	t.Helper()
	inbox := &logoutInbox{}
	inbox.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		inbox.mu.Lock()
		inbox.bodies = append(inbox.bodies, body)
		inbox.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(inbox.server.Close)
	return inbox
}

func (i *logoutInbox) close() { i.server.Close() }

func (i *logoutInbox) URL() string { return i.server.URL }

func (i *logoutInbox) received() [][]byte {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.bodies
}

func registerLogoutURI(t *testing.T, fixture *fixture, clientID, uri string) {
	t.Helper()
	row, err := fixture.register.ByClientID(context.Background(), clientID)
	if err != nil {
		t.Fatalf("reading %s: %v", clientID, err)
	}
	row.BackchannelLogoutURI = uri
	if _, err := fixture.register.Upsert(context.Background(), row); err != nil {
		t.Fatalf("registering the logout URI: %v", err)
	}
}

// signInOnly walks the browser half of the flow and stops before the code
// exchange, which is the state a session is in while a human is logged in.
func signInOnly(t *testing.T, fixture *fixture, scope string) {
	t.Helper()
	_, challenge := pkcePair(t)
	jar := &singleCookie{}
	authorizeURL := fixture.issuer + "/authorize?" +
		flowParams(challenge, url.Values{"scope": {scope}}).Encode()
	if got := jar.get(t, authorizeURL); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d", got.StatusCode)
	}
	fixture.submitLogin(jar, "yann@facile.studio", "correct horse battery staple")
}

// assertLogoutToken checks the delivered token against everything the
// back-channel logout specification requires of it, via the same JWKS a
// relying party would use.
func assertLogoutToken(t *testing.T, fixture *fixture, token, wantSubject string) {
	t.Helper()
	if token == "" {
		t.Fatal("no logout_token in the form body")
	}
	claims, err := fixture.verify(token)
	if err != nil {
		t.Fatalf("the logout token does not verify against the JWKS: %v", err)
	}
	if claims["sub"] != wantSubject {
		t.Errorf("logout sub = %v, want %s", claims["sub"], wantSubject)
	}
	events, _ := claims["events"].(map[string]any)
	const event = "http://schemas.openid.net/event/backchannel-logout"
	if _, ok := events[event]; !ok {
		t.Errorf("logout token carries no back-channel logout event: %v", claims["events"])
	}
	if _, ok := claims["nonce"]; ok {
		t.Error("a logout token must not carry a nonce")
	}
	if parts := strings.Count(token, "."); parts != 2 {
		t.Errorf("the logout token has %d dots, want 2 (%q)", parts, token[:20])
	}
}

// redirectOf is each test client's registered exact-match callback.
func redirectOf(clientID string) string {
	switch clientID {
	case "courrier":
		return "http://127.0.0.1:4020/callback"
	default:
		return "http://127.0.0.1:9999/callback"
	}
}
