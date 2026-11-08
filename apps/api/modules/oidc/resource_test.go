package oidc

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const (
	courrierMCP = "https://courrier.test/mcp"
	agendaMCP   = "https://agenda.test/mcp"
)

// declareResource gives clientID an MCP server, the way the seed's mcp block
// does.
func (f *fixture) declareResource(clientID, resource string, scopes ...string) {
	f.t.Helper()
	err := f.db.Model(&schemas.Client{}).Where("client_id = ?", clientID).
		Updates(map[string]any{"mcp_resource": resource, "mcp_scopes": schemas.StringList(scopes)}).Error
	if err != nil {
		f.t.Fatalf("declaring %s for %s: %v", resource, clientID, err)
	}
}

// resourceWorld is a registre where courrier runs an MCP server and sablier
// is the client that wants to reach it on a human's behalf.
type resourceWorld struct {
	*fixture
	client testClient
}

func newResourceWorld(t *testing.T) *resourceWorld {
	t.Helper()
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", redirectOf("sablier"), "offline_access")
	fixture.application("courrier", redirectOf("courrier"))
	fixture.application("agenda", redirectOf("agenda"))
	fixture.declareResource("courrier", courrierMCP, "courrier.read", "courrier.write")
	fixture.declareResource("agenda", agendaMCP, "agenda.read")
	fixture.human("yann@facile.studio", "correct horse battery staple")
	return &resourceWorld{fixture: fixture, client: testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}}
}

// authorize starts a flow with extra parameters and returns the response
// without following it, so a refusal redirect can be read.
func (w *resourceWorld) authorize(jar *singleCookie, challenge string, extra url.Values) *http.Response {
	w.t.Helper()
	params := flowParams(challenge, url.Values{"state": {"st4te"}})
	params.Set("client_id", w.client.id)
	params.Set("redirect_uri", w.client.redirect)
	for key, values := range extra {
		params[key] = values
	}
	return jar.getNoFollow(w.t, w.issuer+"/authorize?"+params.Encode())
}

// codeFor drives a whole flow with extra authorize parameters and returns the
// verifier and the callback's Location.
func (w *resourceWorld) codeFor(extra url.Values) (string, *url.URL) {
	w.t.Helper()
	verifier, challenge := pkcePair(w.t)
	jar := &singleCookie{}
	started := w.authorize(jar, challenge, extra)
	if started.StatusCode != http.StatusFound {
		w.t.Fatalf("GET /authorize = %d: %s", started.StatusCode, jar.lastBody.String())
	}
	login := jar.get(w.t, w.issuer+"/login?auth_request="+pendingID(w.t, started))
	if login.StatusCode != http.StatusOK {
		w.t.Fatalf("GET /login = %d", login.StatusCode)
	}
	payload, _ := json.Marshal(map[string]string{
		"auth_request": extractAuthRequest(jar.lastBody.String()),
		"email":        "yann@facile.studio", "password": "correct horse battery staple",
	})
	answer := jar.postJSONNoFollow(w.t, w.issuer+"/api/login", string(payload))
	var body struct {
		Redirect string `json:"redirect"`
	}
	if err := json.Unmarshal([]byte(jar.lastBody.String()), &body); err != nil || answer.StatusCode != http.StatusOK {
		w.t.Fatalf("POST /api/login = %d %s", answer.StatusCode, jar.lastBody.String())
	}
	callback := jar.getNoFollow(w.t, w.issuer+body.Redirect)
	location, err := url.Parse(callback.Header.Get("Location"))
	if err != nil || callback.StatusCode != http.StatusFound {
		w.t.Fatalf("callback = %d %q", callback.StatusCode, callback.Header.Get("Location"))
	}
	return verifier, location
}

// redeem exchanges a code, with extra token parameters.
func (w *resourceWorld) redeem(verifier, code string, extra url.Values) (map[string]any, int) {
	w.t.Helper()
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {w.client.redirect}, "code_verifier": {verifier},
	}
	for key, values := range extra {
		form[key] = values
	}
	return w.post("/oauth/token", w.client.id, w.client.secret, form)
}

// pendingID reads the auth request id out of the redirect to the login page
// that /authorize answered with.
func pendingID(t *testing.T, started *http.Response) string {
	t.Helper()
	location, err := url.Parse(started.Header.Get("Location"))
	if err != nil || location.Query().Get("auth_request") == "" {
		t.Fatalf("/authorize sent the browser to %q, not to the login page", started.Header.Get("Location"))
	}
	return location.Query().Get("auth_request")
}

func TestAResourceBoundFlowMintsATokenForTheResourceAlone(t *testing.T) {
	world := newResourceWorld(t)
	verifier, back := world.codeFor(url.Values{
		"scope": {"openid offline_access courrier.read"}, "resource": {courrierMCP},
	})
	if got := back.Query().Get("iss"); got != world.issuer {
		t.Errorf("the authorization response carries iss %q, want %q (RFC 9207)", got, world.issuer)
	}
	granted, status := world.redeem(verifier, back.Query().Get("code"), url.Values{"resource": {courrierMCP}})
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d: %v", status, granted)
	}
	claims := mustClaims(t, world.fixture, granted["access_token"])
	if audience := only(claims["aud"]); audience != courrierMCP {
		t.Errorf("aud = %v, want only %s", claims["aud"], courrierMCP)
	}
	if claims["scope"] != "courrier.read" {
		t.Errorf("scope = %v, want courrier.read alone", claims["scope"])
	}
	if claims["client_id"] != "sablier" {
		t.Errorf("client_id = %v, want sablier", claims["client_id"])
	}
	idClaims := mustClaims(t, world.fixture, granted["id_token"])
	if !strings.Contains(strings.Join(audiences(idClaims["aud"]), " "), "sablier") {
		t.Errorf("the ID token aud %v lacks the client, so the client cannot validate it", idClaims["aud"])
	}

	refreshed, status := world.post("/oauth/token", world.client.id, world.client.secret, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {granted["refresh_token"].(string)},
	})
	if status != http.StatusOK {
		t.Fatalf("refresh = %d: %v", status, refreshed)
	}
	again := mustClaims(t, world.fixture, refreshed["access_token"])
	if only(again["aud"]) != courrierMCP || again["scope"] != "courrier.read" {
		t.Errorf("a refreshed token lost its binding: aud %v scope %v", again["aud"], again["scope"])
	}
	moved, status := world.post("/oauth/token", world.client.id, world.client.secret, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshed["refresh_token"].(string)},
		"resource": {agendaMCP},
	})
	if status != http.StatusBadRequest || moved["error"] != "invalid_target" {
		t.Errorf("a refresh asking for another resource = %d %v, want invalid_target", status, moved)
	}
}

func TestACodeCannotBeRedeemedForAnotherResource(t *testing.T) {
	world := newResourceWorld(t)
	verifier, back := world.codeFor(url.Values{"scope": {"openid courrier.read"}, "resource": {courrierMCP}})
	granted, status := world.redeem(verifier, back.Query().Get("code"), url.Values{"resource": {agendaMCP}})
	if status != http.StatusBadRequest || granted["error"] != "invalid_target" {
		t.Fatalf("redeeming for another resource = %d %v, want invalid_target", status, granted)
	}
}

func TestAPlainTokenKeepsItsShape(t *testing.T) {
	world := newResourceWorld(t)
	verifier, back := world.codeFor(url.Values{"scope": {"openid"}})
	granted, status := world.redeem(verifier, back.Query().Get("code"), nil)
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d: %v", status, granted)
	}
	claims := mustClaims(t, world.fixture, granted["access_token"])
	if only(claims["aud"]) != "sablier" {
		t.Errorf("aud = %v, want the client itself", claims["aud"])
	}
	if _, has := claims["scope"]; has {
		t.Errorf("a token bound to no resource carries scope %v", claims["scope"])
	}
}

func TestAuthorizeRefusesWhatCannotBeBound(t *testing.T) {
	cases := []struct {
		name  string
		extra url.Values
		want  string
	}{
		{"an unknown resource", url.Values{"scope": {"openid"}, "resource": {"https://nowhere.test/mcp"}}, "invalid_target"},
		{"two resources", url.Values{"scope": {"openid"}, "resource": {courrierMCP, agendaMCP}}, "invalid_target"},
		{"another app's scope", url.Values{"scope": {"openid agenda.read"}, "resource": {courrierMCP}}, "invalid_target"},
		{"a resource scope with no resource", url.Values{"scope": {"openid courrier.read"}}, "invalid_scope"},
		{"plain PKCE", url.Values{"scope": {"openid"}, "code_challenge_method": {"plain"}}, "invalid_request"},
	}
	world := newResourceWorld(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, challenge := pkcePair(t)
			jar := &singleCookie{}
			got := world.authorize(jar, challenge, tc.extra)
			location, err := url.Parse(got.Header.Get("Location"))
			if err != nil || got.StatusCode != http.StatusFound {
				t.Fatalf("GET /authorize = %d %q", got.StatusCode, got.Header.Get("Location"))
			}
			if location.Query().Get("error") != tc.want {
				t.Errorf("error = %q, want %q (%s)", location.Query().Get("error"), tc.want, location)
			}
			if location.Query().Get("iss") != world.issuer {
				t.Errorf("an error response carries iss %q, want %q", location.Query().Get("iss"), world.issuer)
			}
		})
	}
}

func TestAResourceOfARestrictedAppNeedsAccessToThatApp(t *testing.T) {
	world := newResourceWorld(t)
	if err := world.db.Model(&schemas.Client{}).Where("client_id = ?", "courrier").
		Update("access_policy", schemas.AccessAssigned).Error; err != nil {
		t.Fatalf("restricting courrier: %v", err)
	}
	_, challenge := pkcePair(t)
	jar := &singleCookie{}
	started := world.authorize(jar, challenge, url.Values{"scope": {"openid courrier.read"}, "resource": {courrierMCP}})
	jar.get(t, world.issuer+"/login?auth_request="+pendingID(t, started))
	payload, _ := json.Marshal(map[string]string{
		"auth_request": extractAuthRequest(jar.lastBody.String()),
		"email":        "yann@facile.studio", "password": "correct horse battery staple",
	})
	got := jar.postJSONNoFollow(t, world.issuer+"/api/login", string(payload))
	if got.StatusCode != http.StatusForbidden || !strings.Contains(jar.lastBody.String(), "no_access") {
		t.Fatalf("signing in for a restricted app's resource = %d %s, want 403 no_access",
			got.StatusCode, jar.lastBody.String())
	}
}

// mustClaims verifies a token from a token response and returns its claims.
func mustClaims(t *testing.T, f *fixture, token any) map[string]any {
	t.Helper()
	raw, _ := token.(string)
	claims, err := f.verify(raw)
	if err != nil {
		t.Fatalf("the token does not verify: %v", err)
	}
	return claims
}

// audiences reads an aud claim, string or array, as a list.
func audiences(claim any) []string {
	switch typed := claim.(type) {
	case string:
		return []string{typed}
	case []any:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			text, _ := value.(string)
			out = append(out, text)
		}
		return out
	}
	return nil
}
