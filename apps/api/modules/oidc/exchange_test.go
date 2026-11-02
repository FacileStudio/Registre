package oidc

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const (
	exchangeGrantURN = "urn:ietf:params:oauth:grant-type:token-exchange"
	accessTokenURN   = "urn:ietf:params:oauth:token-type:access_token"
	refreshTokenURN  = "urn:ietf:params:oauth:token-type:refresh_token"
	idTokenURN       = "urn:ietf:params:oauth:token-type:id_token"
)

// exchangeWorld is a registre with a signed-in human, an app allowed to trade
// that human's token (bulle), two apps it may reach (courrier, agenda) and one
// it may not (opus).
type exchangeWorld struct {
	*fixture
	secret  string
	access  string
	idToken string
	userID  string
}

func newExchangeWorld(t *testing.T) *exchangeWorld {
	t.Helper()
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("bulle", redirectOf("bulle"), "offline_access")
	for _, audience := range []string{"courrier", "agenda", "opus"} {
		fixture.application(audience, redirectOf(audience))
	}
	fixture.human("yann@facile.studio", "correct horse battery staple")
	fixture.allowExchange("bulle", schemas.ExchangeGrants{
		{Audience: "courrier", Scopes: []string{"courrier.read"}},
		{Audience: "agenda", Scopes: []string{"agenda.read", "agenda.write"}},
	})
	granted := signInAndExchange(t, fixture, testClient{id: "bulle", secret: secret, redirect: redirectOf("bulle")}, "openid")
	access, _ := granted["access_token"].(string)
	idToken, _ := granted["id_token"].(string)
	claims, err := fixture.verify(access)
	if err != nil {
		t.Fatalf("the subject token does not verify: %v", err)
	}
	userID, _ := claims["sub"].(string)
	return &exchangeWorld{fixture: fixture, secret: secret, access: access, idToken: idToken, userID: userID}
}

func (f *fixture) allowExchange(clientID string, grants schemas.ExchangeGrants) {
	f.t.Helper()
	row, err := f.register.ByClientID(context.Background(), clientID)
	if err != nil {
		f.t.Fatalf("reading %s: %v", clientID, err)
	}
	row.GrantTypes = append(row.GrantTypes, exchangeGrantURN)
	row.TokenExchange = grants
	if _, err := f.register.Upsert(context.Background(), row); err != nil {
		f.t.Fatalf("granting the exchange to %s: %v", clientID, err)
	}
}

func (w *exchangeWorld) exchange(subject string, extra url.Values) (map[string]any, int) {
	w.t.Helper()
	return w.exchangeAs("bulle", w.secret, subject, extra)
}

func (w *exchangeWorld) exchangeAs(clientID, secret, subject string, extra url.Values) (map[string]any, int) {
	w.t.Helper()
	form := url.Values{
		"grant_type":         {exchangeGrantURN},
		"subject_token":      {subject},
		"subject_token_type": {accessTokenURN},
		"audience":           {"courrier"},
	}
	for key, values := range extra {
		form[key] = values
	}
	return w.post("/oauth/token", clientID, secret, form)
}

func (w *exchangeWorld) subjectJTI() string {
	w.t.Helper()
	claims, err := w.verify(w.access)
	if err != nil {
		w.t.Fatalf("verifying the subject token: %v", err)
	}
	jti, _ := claims["jti"].(string)
	return jti
}

func (w *exchangeWorld) updateToken(jti string, column string, value any) {
	w.t.Helper()
	err := w.db.Model(&schemas.AccessToken{}).Where("id = ?", jti).Update(column, value).Error
	if err != nil {
		w.t.Fatalf("updating %s on %s: %v", column, jti, err)
	}
}

func TestAnExchangeIssuesATokenAddressedToOneApp(t *testing.T) {
	world := newExchangeWorld(t)
	granted, status := world.exchange(world.access, nil)
	if status != http.StatusOK {
		t.Fatalf("exchange = %d: %v", status, granted)
	}
	token, _ := granted["access_token"].(string)
	claims, err := world.verify(token)
	if err != nil {
		t.Fatalf("the delegated token does not verify against the JWKS: %v", err)
	}
	assertDelegatedClaims(t, claims, world.userID)
	if granted["issued_token_type"] != accessTokenURN || granted["token_type"] != "Bearer" {
		t.Errorf("response types = %v / %v", granted["issued_token_type"], granted["token_type"])
	}
	if _, has := granted["refresh_token"]; has {
		t.Error("an exchange must not issue a refresh token")
	}
}

func assertDelegatedClaims(t *testing.T, claims map[string]any, userID string) {
	t.Helper()
	audience, _ := claims["aud"].([]any)
	if len(audience) != 1 || audience[0] != "courrier" {
		t.Errorf("aud = %v, want [courrier]", claims["aud"])
	}
	actor, _ := claims["act"].(map[string]any)
	switch {
	case claims["sub"] != userID:
		t.Errorf("sub = %v, want the human %s", claims["sub"], userID)
	case claims["scope"] != "courrier.read":
		t.Errorf("scope = %v, want courrier.read", claims["scope"])
	case actor["sub"] != "bulle" || claims["client_id"] != "bulle":
		t.Errorf("act = %v, client_id = %v, want bulle", claims["act"], claims["client_id"])
	case claims["jti"] == "":
		t.Error("the token carries no jti")
	}
	issued, _ := claims["iat"].(float64)
	expires, _ := claims["exp"].(float64)
	if expires-issued > 300 || expires <= issued {
		t.Errorf("lifetime = %.0fs, want within 300s", expires-issued)
	}
}

func TestADelegatedTokenIsRecordedWithItsActor(t *testing.T) {
	world := newExchangeWorld(t)
	granted, _ := world.exchange(world.access, nil)
	claims, err := world.verify(granted["access_token"].(string))
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	var row schemas.AccessToken
	if err := world.db.Where("id = ?", claims["jti"]).Take(&row).Error; err != nil {
		t.Fatalf("the delegated token has no row: %v", err)
	}
	if row.Actor != "bulle" || row.Subject != world.userID || !row.Audiences.Contains("courrier") {
		t.Errorf("row = %+v", row)
	}
}

func TestAnAskedForScopeIsGrantedAndNothingElse(t *testing.T) {
	world := newExchangeWorld(t)
	granted, status := world.exchange(world.access, url.Values{"audience": {"agenda"}, "scope": {"agenda.read"}})
	if status != http.StatusOK {
		t.Fatalf("exchange = %d: %v", status, granted)
	}
	claims, _ := world.verify(granted["access_token"].(string))
	if claims["scope"] != "agenda.read" {
		t.Errorf("scope = %v, want only what was asked", claims["scope"])
	}
	whole, _ := world.exchange(world.access, url.Values{"audience": {"agenda"}})
	claims, _ = world.verify(whole["access_token"].(string))
	if claims["scope"] != "agenda.read agenda.write" {
		t.Errorf("an empty request scope = %v, want the whole allowance", claims["scope"])
	}
}

func TestADelegationNeverOutlivesTheLoginItCameFrom(t *testing.T) {
	world := newExchangeWorld(t)
	soon := world.clock.now().Add(90 * time.Second)
	world.updateToken(world.subjectJTI(), "expires_at", soon)
	granted, status := world.exchange(world.access, nil)
	if status != http.StatusOK {
		t.Fatalf("exchange = %d: %v", status, granted)
	}
	claims, _ := world.verify(granted["access_token"].(string))
	if expires, _ := claims["exp"].(float64); expires > float64(soon.Unix())+1 {
		t.Errorf("exp = %.0f outlives the subject token at %d", expires, soon.Unix())
	}
}

func TestDiscoveryAdvertisesTheExchangeGrant(t *testing.T) {
	world := newExchangeWorld(t)
	request, err := http.NewRequest(http.MethodGet, world.issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		t.Fatalf("building the discovery request: %v", err)
	}
	document, _ := decode[map[string]any](t, request)
	grants, _ := document["grant_types_supported"].([]any)
	for _, grant := range grants {
		if grant == exchangeGrantURN {
			return
		}
	}
	t.Errorf("grant_types_supported = %v, want the token-exchange grant", grants)
}
