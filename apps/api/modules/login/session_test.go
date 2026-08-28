package login

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func (f *fixture) sessionGet(cookie *http.Cookie, authRequest string) (map[string]any, *http.Response) {
	f.t.Helper()
	url := f.server.URL + "/api/login/session"
	if authRequest != "" {
		url += "?auth_request=" + authRequest
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		f.t.Fatalf("building request: %v", err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatalf("GET /api/login/session: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		f.t.Fatalf("reading body: %v", err)
	}
	answer := map[string]any{}
	_ = json.Unmarshal(raw, &answer)
	return answer, res
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
		t.Fatalf("GET /api/login/session status = %d: %v", secondRes.StatusCode, answer)
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
