package oidc

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/logout"
)

// drillBudget is how long the whole deactivation-to-ejection round trip may
// take before the phase-2 exit criterion counts as missed.
const drillBudget = 5 * time.Second

// drillGrant is what the human's login opened: a refresh grant and an access
// token, the doors the drill must later close.
type drillGrant struct {
	refresh     string
	accessToken string
}

// drillLogin performs a real authorization_code login with offline_access
// against the in-process provider and returns what the login granted.
func drillLogin(t *testing.T, fixture *fixture, secret string) drillGrant {
	t.Helper()
	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	granted := signInAndExchange(t, fixture, client, "openid offline_access")
	dg := drillGrant{
		refresh:     granted["refresh_token"].(string),
		accessToken: granted["access_token"].(string),
	}
	if dg.refresh == "" {
		t.Fatal("offline_access was requested but no refresh token arrived")
	}
	return dg
}

// assertGrantAlive proves the doors exist before the drill kicks them.
func assertGrantAlive(t *testing.T, fixture *fixture, secret string, dg drillGrant) {
	t.Helper()
	if _, status := fixture.post("/oauth/token", "sablier", secret, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {dg.refresh},
	}); status != http.StatusOK {
		t.Fatal("the refresh grant did not work before the drill ran")
	}
	before, _ := fixture.introspect("sablier", secret, dg.accessToken)
	if before["active"] != true {
		t.Fatal("the access token did not introspect as active before the drill ran")
	}
}

// disableAndNotify walks `registre user-disable`'s exact code path:
// identity.Disable, then logout.Fire over the apps it reports.
func disableAndNotify(t *testing.T, fixture *fixture) (string, []string, time.Time) {
	t.Helper()
	user, err := fixture.users.ByEmail(context.Background(), "yann@facile.studio")
	if err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	started := time.Now()
	reached, err := fixture.users.Disable(context.Background(), user.ID, started)
	if err != nil {
		t.Fatalf("disabling the user: %v", err)
	}
	fire := logout.New(fixture.keys, fixture.register, fixture.issuer, slog.New(slog.DiscardHandler))
	if err := fire.Fire(context.Background(), user.ID, reached); err != nil {
		t.Fatalf("firing back-channel logout: %v", err)
	}
	return user.ID, reached, started
}

// awaitLogoutToken waits for the app's notification inside the budget and
// checks it against everything back-channel logout requires.
func awaitLogoutToken(t *testing.T, inbox *logoutInbox, fixture *fixture, userID string, started time.Time) {
	t.Helper()
	bodies := inbox.await(t, 1)
	if len(bodies) != 1 {
		t.Fatalf("the app received %d logout notifications within %s, want 1", len(bodies), drillBudget)
	}
	elapsed := inbox.arrivals()[0].Sub(started)
	if elapsed > drillBudget {
		t.Errorf("logout token arrived after %s, want under %s", elapsed, drillBudget)
	}
	values, err := url.ParseQuery(string(bodies[0]))
	if err != nil {
		t.Fatalf("unreadable form body %q: %v", bodies[0], err)
	}
	assertLogoutToken(t, fixture, values.Get("logout_token"), userID)
	t.Logf("drill: logout token arrived %d ms after user-disable", elapsed.Milliseconds())
}

// assertFreshSignInRefused proves a disabled account cannot start over. A 200
// from /api/login would be a lockout failure; anything else is a refusal.
func assertFreshSignInRefused(t *testing.T, fixture *fixture, clientID, redirect string) {
	t.Helper()
	jar := &singleCookie{}
	params := flowParams("", url.Values{"scope": {"openid"}})
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirect)
	if got := jar.get(t, fixture.issuer+"/authorize?"+params.Encode()); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d", got.StatusCode)
	}
	payload := `{"auth_request":"` + extractAuthRequest(jar.lastBody.String()) +
		`","email":"yann@facile.studio","password":"correct horse battery staple"}`
	response := jar.postJSONNoFollow(t, fixture.issuer+"/api/login", payload)
	if response.StatusCode == http.StatusOK {
		t.Errorf("a disabled account signed in again via /api/login: %s", jar.lastBody.String())
	}
	t.Logf("a disabled account's fresh sign-in answered %d (refusal expected)", response.StatusCode)
}

// TestDisablingAUserDropsThemFromEveryAppWithinSeconds is the phase-2 exit
// criterion, run rather than promised. A fake relying party registers a
// backchannel_logout_uri, a human performs a real authorization_code login
// with offline_access, then `registre user-disable`'s exact code path —
// identity.Disable followed by logout.Fire — fires. The app must receive a
// valid signed logout token within seconds, and every door the login opened
// must be shut afterwards: the refresh grant dies as invalid_grant and a
// fresh sign-in is refused.
//
// An unexpired access token is deliberately not asserted dead: access tokens
// verify offline, so no server-side check can close them, and the logout
// token itself is what tells the app to drop its session — that residual
// window is the short access-token TTL's job.
func TestDisablingAUserDropsThemFromEveryAppWithinSeconds(t *testing.T) {
	inbox := newLogoutInbox(t)
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", redirectOf("sablier"), "offline_access")
	registerLogoutURI(t, fixture, "sablier", inbox.URL())
	fixture.human("yann@facile.studio", "correct horse battery staple")

	dg := drillLogin(t, fixture, secret)
	assertGrantAlive(t, fixture, secret, dg)

	userID, reached, started := disableAndNotify(t, fixture)
	if len(reached) != 1 || reached[0] != "sablier" {
		t.Fatalf("Disable returned %v, want [sablier]", reached)
	}

	awaitLogoutToken(t, inbox, fixture, userID, started)

	refused, status := fixture.post("/oauth/token", "sablier", secret, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {dg.refresh},
	})
	if status != http.StatusBadRequest || refused["error"] != "invalid_grant" {
		t.Errorf("after disable the refresh grant answered %d %v, want 400 invalid_grant", status, refused["error"])
	}

	assertFreshSignInRefused(t, fixture, "sablier", redirectOf("sablier"))
}
