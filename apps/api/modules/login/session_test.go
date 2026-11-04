package login

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// sessionCall sends one /api/login/session request the way the login page
// does: a POST with the CSRF header and the flow in the body. method GET or a
// false csrf builds the forged variants the tests refuse.
func (f *fixture) sessionCall(
	method string, csrf bool, cookie *http.Cookie, authRequest string,
) (map[string]any, *http.Response) {
	f.t.Helper()
	url, body := f.server.URL+"/api/login/session", io.Reader(nil)
	if method == http.MethodGet && authRequest != "" {
		url += "?auth_request=" + authRequest
	} else if method == http.MethodPost {
		body = strings.NewReader(`{"auth_request":"` + authRequest + `"}`)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		f.t.Fatalf("building request: %v", err)
	}
	if csrf {
		req.Header.Set("X-Facile-CSRF", "1")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatalf("%s /api/login/session: %v", method, err)
	}
	defer func() { _ = res.Body.Close() }()
	answer := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&answer)
	return answer, res
}

// sessionGet is the login page's call.
func (f *fixture) sessionGet(cookie *http.Cookie, authRequest string) (map[string]any, *http.Response) {
	f.t.Helper()
	return f.sessionCall(http.MethodPost, true, cookie, authRequest)
}

func TestSessionLoginReusesActiveSessionForNewAuthRequest(t *testing.T) {
	f := newFixture(t)
	user := f.user("sso@example.test", "correct horse battery staple")
	firstFlow := f.authRequest()

	_, firstRes := f.login(firstFlow, user.Email, "correct horse battery staple")
	if firstRes.StatusCode != http.StatusOK {
		t.Fatalf("initial login status = %d", firstRes.StatusCode)
	}
	c := cookie(firstRes)
	if c == nil {
		t.Fatal("expected account cookie from initial login")
	}

	secondFlow := f.authRequest()
	answer, secondRes := f.sessionGet(c, secondFlow)
	if secondRes.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login/session status = %d: %v", secondRes.StatusCode, answer)
	}
	if answer["redirect"] != "/authorize/callback?id="+secondFlow {
		t.Fatalf("redirect = %v, want /authorize/callback?id=%s", answer["redirect"], secondFlow)
	}
	if !f.done(secondFlow) {
		t.Fatal("the second flow was not completed by the active session")
	}
}

func TestSessionLoginDirectReturnsRoot(t *testing.T) {
	f := newFixture(t)
	user := f.user("direct-sso@example.test", "correct horse battery staple")
	flow := f.authRequest()

	_, loginRes := f.login(flow, user.Email, "correct horse battery staple")
	c := cookie(loginRes)
	if c == nil {
		t.Fatal("expected account cookie")
	}

	answer, res := f.sessionGet(c, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %v", res.StatusCode, answer)
	}
	if answer["redirect"] != "/" {
		t.Fatalf("redirect = %v, want /", answer["redirect"])
	}
}

func TestSessionLoginWithoutCookieIsRefused(t *testing.T) {
	f := newFixture(t)
	flow := f.authRequest()

	answer, res := f.sessionGet(nil, flow)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %v", res.StatusCode, answer)
	}
	if f.done(flow) {
		t.Fatal("flow should not be completed without a cookie")
	}
}

// TestSessionLoginRefusesForgedCompletion is the login CSRF that turned an
// auth request id into an account takeover: a link that makes a signed-in
// browser finish a flow somebody else started. A GET naming a flow and a POST
// without the CSRF header must both leave the flow unfinished.
func TestSessionLoginRefusesForgedCompletion(t *testing.T) {
	f := newFixture(t)
	user := f.user("victim@example.test", "correct horse battery staple")
	_, loginRes := f.login(f.authRequest(), user.Email, "correct horse battery staple")
	c := cookie(loginRes)
	if c == nil {
		t.Fatal("expected account cookie")
	}

	attackerFlow := f.authRequest()
	if _, res := f.sessionCall(http.MethodGet, false, c, attackerFlow); res.StatusCode != http.StatusForbidden {
		t.Errorf("GET with auth_request = %d, want 403", res.StatusCode)
	}
	if _, res := f.sessionCall(http.MethodPost, false, c, attackerFlow); res.StatusCode != http.StatusForbidden {
		t.Errorf("POST without CSRF header = %d, want 403", res.StatusCode)
	}
	if f.done(attackerFlow) {
		t.Fatal("a forged request finished the flow")
	}
	if answer, res := f.sessionCall(http.MethodGet, false, c, ""); res.StatusCode != http.StatusOK || answer["redirect"] != "/" {
		t.Errorf("GET probe = %d %v, want 200 redirect /", res.StatusCode, answer)
	}
}
