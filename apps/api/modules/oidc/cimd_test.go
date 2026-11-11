package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/metadoc"
)

const claudeCode = "https://client.test/claude-code"

// stubDocuments serves client ID metadata documents from memory, through the
// same validation the real fetcher applies.
type stubDocuments map[string]metadoc.Metadata

func (d stubDocuments) Fetch(_ context.Context, clientID string) (metadoc.Metadata, time.Time, error) {
	document, ok := d[clientID]
	if !ok || document.ClientID != clientID {
		return metadoc.Metadata{}, time.Time{}, errors.New("no such document")
	}
	validated, err := metadoc.Validate(document)
	return validated, time.Now().Add(time.Hour), err
}

// thirdParty is a browser driving a CIMD client's flow against courrier's MCP
// server.
type thirdParty struct {
	*fixture
	jar      *singleCookie
	verifier string
	redirect string
}

func newThirdParty(t *testing.T) *thirdParty {
	t.Helper()
	fixture := newFixture(t, defaultKeyConfig())
	fixture.application("courrier", redirectOf("courrier"))
	fixture.declareResource("courrier", courrierMCP, "courrier.read", "courrier.write")
	fixture.human("yann@facile.studio", "correct horse battery staple")
	fixture.documents[claudeCode] = metadoc.Metadata{
		ClientID: claudeCode, ClientName: "Claude Code", RedirectURIs: []string{"http://localhost/callback"},
	}
	return &thirdParty{fixture: fixture, jar: &singleCookie{}, redirect: "http://localhost:4567/callback"}
}

// start runs /authorize and the password login, and returns the callback's
// answer without following it.
func (p *thirdParty) start(extra url.Values) *http.Response {
	p.t.Helper()
	verifier, challenge := pkcePair(p.t)
	p.verifier = verifier
	params := url.Values{
		"client_id": {claudeCode}, "redirect_uri": {p.redirect}, "response_type": {"code"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"st4te"},
		"scope": {"courrier.read offline_access"}, "resource": {courrierMCP},
	}
	for key, values := range extra {
		params[key] = values
	}
	started := p.jar.getNoFollow(p.t, p.issuer+"/authorize?"+params.Encode())
	if started.StatusCode != http.StatusFound {
		p.t.Fatalf("GET /authorize = %d %q", started.StatusCode, started.Header.Get("Location"))
	}
	location, _ := url.Parse(started.Header.Get("Location"))
	if location.Query().Get("error") != "" {
		return started
	}
	payload, _ := json.Marshal(map[string]string{
		"auth_request": pendingID(p.t, started),
		"email":        "yann@facile.studio", "password": "correct horse battery staple",
	})
	answer := p.jar.postJSONNoFollow(p.t, p.issuer+"/api/login", string(payload))
	var body struct {
		Redirect string `json:"redirect"`
	}
	if err := json.Unmarshal([]byte(p.jar.lastBody.String()), &body); err != nil || answer.StatusCode != http.StatusOK {
		p.t.Fatalf("POST /api/login = %d %s", answer.StatusCode, p.jar.lastBody.String())
	}
	return p.jar.getNoFollow(p.t, p.issuer+body.Redirect)
}

// consent answers the consent screen and returns where it sends the browser.
func (p *thirdParty) consent(id string, allow bool) string {
	p.t.Helper()
	payload, _ := json.Marshal(map[string]any{"auth_request": id, "allow": allow})
	answer := p.jar.postJSONNoFollow(p.t, p.issuer+"/api/connect", string(payload))
	var body struct {
		Redirect string `json:"redirect"`
	}
	if err := json.Unmarshal([]byte(p.jar.lastBody.String()), &body); err != nil || answer.StatusCode != http.StatusOK {
		p.t.Fatalf("POST /api/connect = %d %s", answer.StatusCode, p.jar.lastBody.String())
	}
	return body.Redirect
}

// redeem exchanges a code as the public client: no secret, PKCE only.
func (p *thirdParty) redeem(code string) (map[string]any, int) {
	p.t.Helper()
	return p.postForm("/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {claudeCode},
		"redirect_uri": {p.redirect}, "code_verifier": {p.verifier}, "resource": {courrierMCP},
	})
}

func TestACIMDClientNeedsConsentThenGetsAShortResourceToken(t *testing.T) {
	party := newThirdParty(t)
	callback := party.start(nil)
	consentAt := callback.Header.Get("Location")
	if !strings.HasPrefix(consentAt, ConsentPath+"?auth_request=") {
		t.Fatalf("a third party's callback = %d %q, want the consent page", callback.StatusCode, consentAt)
	}
	id := strings.TrimPrefix(consentAt, ConsentPath+"?auth_request=")
	view := party.jar.get(t, party.issuer+"/api/connect?auth_request="+id)
	shown := party.jar.lastBody.String()
	if view.StatusCode != http.StatusOK || !strings.Contains(shown, `"redirect_host":"localhost:4567"`) ||
		!strings.Contains(shown, `"loopback_only":true`) || !strings.Contains(shown, `"app_name":"courrier"`) {
		t.Fatalf("GET /api/connect = %d %s", view.StatusCode, shown)
	}

	back := party.jar.getNoFollow(t, party.issuer+party.consent(id, true))
	location, _ := url.Parse(back.Header.Get("Location"))
	if location.Query().Get("code") == "" || location.Query().Get("iss") != party.issuer {
		t.Fatalf("after consent the callback sent %q", back.Header.Get("Location"))
	}
	granted, status := party.redeem(location.Query().Get("code"))
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %v", status, granted)
	}
	claims := mustClaims(t, party.fixture, granted["access_token"])
	if only(claims["aud"]) != courrierMCP || claims["client_id"] != claudeCode || claims["scope"] != "courrier.read" {
		t.Errorf("claims = %v", claims)
	}
	if life := claims["exp"].(float64) - claims["iat"].(float64); life < 590 || life > 600 {
		t.Errorf("a third-party access token lives %v s, want 600 (iat and exp come from two clock reads)", life)
	}

	again := party.start(nil)
	if location := again.Header.Get("Location"); strings.HasPrefix(location, ConsentPath) {
		t.Error("a second flow covered by the grant asked for consent again")
	}
}

func TestRevokingAConnectedAppEndsItsRefresh(t *testing.T) {
	party := newThirdParty(t)
	id := strings.TrimPrefix(party.start(nil).Header.Get("Location"), ConsentPath+"?auth_request=")
	back := party.jar.getNoFollow(t, party.issuer+party.consent(id, true))
	location, _ := url.Parse(back.Header.Get("Location"))
	granted, _ := party.redeem(location.Query().Get("code"))

	party.jar.get(t, party.issuer+"/api/account/apps")
	var apps []struct {
		ID         string `json:"id"`
		ClientName string `json:"client_name"`
	}
	if err := json.Unmarshal([]byte(party.jar.lastBody.String()), &apps); err != nil || len(apps) != 1 ||
		apps[0].ClientName != "Claude Code" {
		t.Fatalf("GET /api/account/apps = %s", party.jar.lastBody.String())
	}
	request, _ := http.NewRequest(http.MethodDelete, party.issuer+"/api/account/apps/"+apps[0].ID, nil)
	request.Header.Set("X-Facile-CSRF", "1")
	if gone := party.jar.do(t, request); gone.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE = %d %s", gone.StatusCode, party.jar.lastBody.String())
	}
	refreshed, status := party.postForm("/oauth/token", url.Values{
		"grant_type": {"refresh_token"}, "client_id": {claudeCode}, "refresh_token": {granted["refresh_token"].(string)},
	})
	if status == http.StatusOK || refreshed["error"] != "invalid_grant" {
		t.Errorf("a revoked app refreshed: %d %v, want invalid_grant", status, refreshed)
	}
}

func TestRefusingConsentSendsAccessDeniedToTheClient(t *testing.T) {
	party := newThirdParty(t)
	id := strings.TrimPrefix(party.start(nil).Header.Get("Location"), ConsentPath+"?auth_request=")
	denied, _ := url.Parse(party.consent(id, false))
	if denied.Host != "localhost:4567" || denied.Query().Get("error") != "access_denied" ||
		denied.Query().Get("state") != "st4te" || denied.Query().Get("iss") != party.issuer {
		t.Errorf("refusing sent the browser to %s", denied)
	}
	if replay := party.jar.getNoFollow(t, party.issuer+"/authorize/callback?id="+id); replay.StatusCode == http.StatusFound &&
		strings.Contains(replay.Header.Get("Location"), "code=") {
		t.Error("a refused flow still produced a code")
	}
}

func TestTheConsentScreenAnswersOnlyTheSessionThatSignedIn(t *testing.T) {
	party := newThirdParty(t)
	id := strings.TrimPrefix(party.start(nil).Header.Get("Location"), ConsentPath+"?auth_request=")
	stranger := &singleCookie{}
	if got := stranger.get(t, party.issuer+"/api/connect?auth_request="+id); got.StatusCode != http.StatusUnauthorized {
		t.Errorf("no session = %d, want 401", got.StatusCode)
	}
	party.human("eve@facile.studio", "another long passphrase here")
	other := &thirdParty{fixture: party.fixture, jar: &singleCookie{}, redirect: party.redirect}
	other.start(nil)
	if got := other.jar.get(t, party.issuer+"/api/connect?auth_request="+id); got.StatusCode != http.StatusNotFound {
		t.Errorf("another session read the flow: %d", got.StatusCode)
	}
}

func TestAThirdPartyMustNameARegisteredResource(t *testing.T) {
	party := newThirdParty(t)
	for name, extra := range map[string]url.Values{
		"no resource":       {"resource": nil, "scope": {"openid"}},
		"an unknown one":    {"resource": {"https://nowhere.test/mcp"}},
		"a bad document":    {"client_id": {"https://client.test/missing"}},
		"an unlisted port":  {"redirect_uri": {"http://evil.test:4567/callback"}},
		"plain PKCE method": {"code_challenge_method": {"plain"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, challenge := pkcePair(t)
			params := url.Values{
				"client_id": {claudeCode}, "redirect_uri": {party.redirect}, "response_type": {"code"},
				"code_challenge": {challenge}, "code_challenge_method": {"S256"},
				"scope": {"courrier.read"}, "resource": {courrierMCP},
			}
			for key, values := range extra {
				params[key] = values
			}
			got := (&singleCookie{}).getNoFollow(t, party.issuer+"/authorize?"+params.Encode())
			location, _ := url.Parse(got.Header.Get("Location"))
			if got.StatusCode == http.StatusFound && location.Query().Get("auth_request") != "" {
				t.Errorf("%s reached the login page", name)
			}
		})
	}
}
