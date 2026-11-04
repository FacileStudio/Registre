package oidc

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// startFlow opens a flow at /authorize in jar's browser and returns its id.
func startFlow(t *testing.T, f *fixture, jar *singleCookie, challenge string) string {
	t.Helper()
	extra := url.Values{"scope": {"openid profile email"}, "state": {"att4cker"}}
	if got := jar.get(t, f.issuer+"/authorize?"+flowParams(challenge, extra).Encode()); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d: %s", got.StatusCode, jar.lastBody.String())
	}
	id := extractAuthRequest(jar.lastBody.String())
	if id == "" {
		t.Fatal("the authorize page carries no auth_request parameter")
	}
	return id
}

// codeFrom calls the callback for id in jar's browser and returns the code it
// redirected with, or the empty string when it refused.
func codeFrom(t *testing.T, f *fixture, jar *singleCookie, id string) string {
	t.Helper()
	response := jar.getNoFollow(t, f.issuer+"/authorize/callback?id="+id)
	return parseCallback(response.Header.Get("Location")).code
}

// TestCallbackRefusesAFlowFinishedByAnotherBrowser is the account takeover
// the binding exists for. The attacker starts a flow with their own state and
// PKCE, a signed-in victim finishes it, and the attacker polls the callback.
// Neither the attacker's browser nor the victim's may collect a code: the
// attacker lacks the victim's session, the victim lacks the attacker's flow
// cookie.
func TestCallbackRefusesAFlowFinishedByAnotherBrowser(t *testing.T) {
	f := newFixture(t, defaultKeyConfig())
	f.application("sablier", "http://127.0.0.1:9999/callback")
	f.human("yann@facile.studio", "correct horse battery staple")

	victim := &singleCookie{}
	_, challenge := pkcePair(t)
	if got := driveToTheCode(t, f, victim, "", challenge); got.code == "" {
		t.Fatal("the victim's own sign-in produced no code")
	}

	attacker := &singleCookie{}
	_, attackerChallenge := pkcePair(t)
	flow := startFlow(t, f, attacker, attackerChallenge)

	if got := victim.resumeSession(t, f.issuer, flow); got.StatusCode != http.StatusOK {
		t.Fatalf("victim finishing the flow = %d: %s", got.StatusCode, victim.lastBody.String())
	}
	if code := codeFrom(t, f, attacker, flow); code != "" {
		t.Fatal("the attacker's browser collected a code for a flow the victim finished")
	}
	if code := codeFrom(t, f, victim, flow); code != "" {
		t.Fatal("a browser that did not start the flow collected its code")
	}
}

// TestCallbackIssuesOneCodePerFlow checks that a second callback for a flow
// that already produced a code gets nothing, even from the browser that owns
// it, and that SaveAuthCode refuses to overwrite a code.
func TestCallbackIssuesOneCodePerFlow(t *testing.T) {
	f := newFixture(t, defaultKeyConfig())
	f.application("sablier", "http://127.0.0.1:9999/callback")
	f.human("yann@facile.studio", "correct horse battery staple")

	jar := &singleCookie{}
	_, challenge := pkcePair(t)
	flow := startFlow(t, f, jar, challenge)
	flowCookies := map[string]*http.Cookie{}
	for name, cookie := range jar.cookies {
		flowCookies[name] = cookie
	}
	first := f.submitLogin(jar, "yann@facile.studio", "correct horse battery staple")
	if first.code == "" {
		t.Fatal("the first callback produced no code")
	}
	for name, cookie := range flowCookies {
		jar.cookies[name] = cookie
	}
	if code := codeFrom(t, f, jar, flow); code != "" {
		t.Fatal("a second callback minted a fresh code")
	}
	if err := f.storage.SaveAuthCode(context.Background(), flow, "another"); err == nil {
		t.Fatal("SaveAuthCode overwrote an existing code")
	}
}
