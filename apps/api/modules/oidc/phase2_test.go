package oidc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// The second phase-1 slice and phase 2, end to end over real HTTP: a refresh
// grant that rotates, roles filtered per client, back-channel logout reaching
// the apps a session reached, and an email_verified claim that tells the
// truth.

// testClient names an application under test and its registered callback.
type testClient struct {
	id       string
	secret   string
	redirect string
}

// signInAndExchange drives the browser half of the flow for client and
// exchanges the code, returning what the token endpoint answered.
func signInAndExchange(t *testing.T, fixture *fixture, client testClient, scope string) map[string]any {
	t.Helper()
	verifier, challenge := pkcePair(t)
	jar := &singleCookie{}
	params := flowParams(challenge, url.Values{"scope": {scope}, "state": {"st4te"}, "nonce": {"n0nce"}})
	params.Set("client_id", client.id)
	params.Set("redirect_uri", client.redirect)
	authorizeURL := fixture.issuer + "/authorize?" + params.Encode()
	if got := jar.get(t, authorizeURL); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d: %s", got.StatusCode, jar.lastBody.String())
	}
	back := fixture.submitLogin(jar, "yann@facile.studio", "correct horse battery staple")
	granted, status := fixture.post("/oauth/token", client.id, client.secret, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {back.code},
		"redirect_uri":  {client.redirect},
		"code_verifier": {verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s", status, granted["error"])
	}
	return granted
}

func TestARefreshTokenRotatesAndTheOldOneDies(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback", "offline_access")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	granted := signInAndExchange(t, fixture, client, "openid offline_access")
	first, _ := granted["refresh_token"].(string)
	if first == "" {
		t.Fatal("offline_access was requested but no refresh token arrived")
	}

	redeem := func(token string) (map[string]any, int) {
		return fixture.post("/oauth/token", "sablier", secret, url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {token},
		})
	}
	again, status := redeem(first)
	if status != http.StatusOK {
		t.Fatalf("refresh grant = %d %s: %s", status, again["error"], again["error_description"])
	}
	renewed, _ := again["refresh_token"].(string)
	if renewed == "" || renewed == first {
		t.Fatalf("the grant did not rotate: %q", renewed)
	}
	if _, err := fixture.verify(again["access_token"].(string)); err != nil {
		t.Fatalf("the refreshed access token does not verify against the JWKS: %v", err)
	}

	spent, status := redeem(first)
	if status != http.StatusBadRequest || spent["error"] != "invalid_grant" {
		t.Errorf("a rotated-out grant answered %d %v, want 400 invalid_grant", status, spent["error"])
	}
	if latest, status := redeem(renewed); status != http.StatusOK {
		t.Fatalf("the successor grant was collateral damage: %d %s", status, latest["error"])
	}
}

func TestRolesArriveFilteredPerClient(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	sablierSecret := fixture.application("sablier", "http://127.0.0.1:9999/callback", "roles")
	courrierSecret := fixture.application("courrier", "http://127.0.0.1:4020/callback", "roles")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	sablier, err := fixture.register.ByClientID(context.Background(), "sablier")
	if err != nil {
		t.Fatalf("reading sablier: %v", err)
	}
	sablier.VisibleRoles = schemas.StringList{"admin"}
	if _, err := fixture.register.Upsert(context.Background(), sablier); err != nil {
		t.Fatalf("whitelisting roles for sablier: %v", err)
	}
	user, err := fixture.users.ByEmail(context.Background(), "yann@facile.studio")
	if err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	user.Roles = schemas.StringList{"admin", "editor"}
	err = fixture.db.Model(&schemas.User{}).Where("id = ?", user.ID).Update("roles", user.Roles).Error
	if err != nil {
		t.Fatalf("giving the user roles: %v", err)
	}

	for name, tc := range map[string]struct {
		secret string
		want   []any
	}{
		"sablier":  {sablierSecret, []any{"admin"}},
		"courrier": {courrierSecret, nil},
	} {
		t.Run(name, func(t *testing.T) {
			client := testClient{id: name, secret: tc.secret, redirect: redirectOf(name)}
			granted := signInAndExchange(t, fixture, client, "openid roles")
			claims, err := fixture.verify(granted["id_token"].(string))
			if err != nil {
				t.Fatalf("id_token does not verify: %v", err)
			}
			got := claims["roles"]
			if tc.want == nil {
				if got != nil {
					t.Errorf("roles = %v, want none", got)
				}
				return
			}
			roles, _ := got.([]any)
			if len(roles) != len(tc.want) || roles[0] != tc.want[0] {
				t.Errorf("roles = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBackChannelLogoutNotifiesTheAppsTheSessionReached(t *testing.T) {
	inbox := newLogoutInbox(t)
	defer inbox.close()

	fixture := newFixture(t, defaultKeyConfig())
	fixture.application("sablier", "http://127.0.0.1:9999/callback")
	registerLogoutURI(t, fixture, "sablier", inbox.URL())
	fixture.human("yann@facile.studio", "correct horse battery staple")
	signInOnly(t, fixture, "openid")

	user, err := fixture.users.ByEmail(context.Background(), "yann@facile.studio")
	if err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	if err := fixture.storage.TerminateSession(context.Background(), user.ID, ""); err != nil {
		t.Fatalf("ending the sessions: %v", err)
	}

	bodies := inbox.received()
	if len(bodies) != 1 {
		t.Fatalf("the app received %d logout notifications, want 1", len(bodies))
	}
	values, err := url.ParseQuery(string(bodies[0]))
	if err != nil {
		t.Fatalf("unreadable form body %q: %v", bodies[0], err)
	}
	assertLogoutToken(t, fixture, values.Get("logout_token"), user.ID)
}

func TestUserinfoSaysEmailVerifiedTruthfully(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	granted := signInAndExchange(t, fixture, client, "openid email")
	request, err := http.NewRequest(http.MethodGet, fixture.issuer+"/userinfo", nil)
	if err != nil {
		t.Fatalf("building the userinfo request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+granted["access_token"].(string))
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatalf("GET /userinfo: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatalf("reading the userinfo response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /userinfo = %d: %s", response.StatusCode, body)
	}
	var info struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("decoding the userinfo response: %v", err)
	}
	if info.Email != "yann@facile.studio" {
		t.Errorf("email = %q", info.Email)
	}
	if !info.EmailVerified {
		t.Error("an operator-minted account must say its address is verified; " +
			"a false here re-freezes the adoption Authentik hardcoded shut")
	}
}
