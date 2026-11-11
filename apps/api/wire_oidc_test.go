package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const (
	wireSecret   = "golden-secret-0123456789abcdefghij"
	wirePassword = "correct horse battery staple"
	wireRedirect = "http://127.0.0.1:9999/callback"
	wireVerifier = "golden-verifier-0123456789abcdefghijklmnopqrstuvwxyz"
)

// TestWireOIDC pins the protocol answers every relying party reads:
// discovery, the key set, the token endpoint for a machine and for a human,
// userinfo and introspection, including their refusals.
func TestWireOIDC(t *testing.T) {
	w := newWire(t)
	w.seedClients()
	w.get("login page", "/login", "")
	w.get("discovery", "/.well-known/openid-configuration", "")
	w.get("authorization server metadata", "/.well-known/oauth-authorization-server", "")
	w.hideKeys()
	w.get("jwks", "/keys", "")
	w.form("token client_credentials", "/oauth/token", "golden-agent", url.Values{"grant_type": {"client_credentials"}})
	w.form("token wrong secret", "/oauth/token", "golden-agent:wrong", url.Values{"grant_type": {"client_credentials"}})
	w.form("token unsupported grant", "/oauth/token", "golden-app", url.Values{"grant_type": {"password"}})
	access := w.humanTokens()
	w.get("userinfo", "/userinfo", access)
	w.get("userinfo without token", "/userinfo", "")
	w.get("userinfo bad token", "/userinfo", "not-a-token")
	w.form("introspect", "/oauth/introspect", "golden-app", url.Values{"token": {access}})
	w.form("introspect unknown token", "/oauth/introspect", "golden-app", url.Values{"token": {"nope"}})
	w.form("introspect without client", "/oauth/introspect", "", url.Values{"token": {access}})
	w.golden("oidc")
}

// seedClients registers one application and one service account, both with
// fixed ids, subjects and secret, and a fixed human.
func (w *wire) seedClients() {
	w.t.Helper()
	ctx := context.Background()
	verifier := credentials.New(2)
	hash, err := verifier.Hash(ctx, wireSecret)
	if err != nil {
		w.t.Fatal(err)
	}
	rows := []schemas.Client{{
		ClientID: "golden-app", Subject: "golden-app-subject", Name: "Golden", Kind: schemas.KindApplication,
		SecretHash: hash, GrantTypes: schemas.StringList{"authorization_code", "refresh_token"},
		Scopes: schemas.StringList{"openid", "profile", "email", "offline_access"}, RedirectURIs: schemas.StringList{wireRedirect},
	}, {
		ClientID: "golden-agent", Subject: "golden-agent-subject", Name: "Agent", Kind: schemas.KindServiceAccount,
		SecretHash: hash, GrantTypes: schemas.StringList{"client_credentials"},
		Audiences: schemas.StringList{"golden-app"}, Managed: true,
	}}
	for _, row := range rows {
		if err := w.db.Create(&row).Error; err != nil {
			w.t.Fatalf("seeding %s: %v", row.ClientID, err)
		}
	}
	user, err := identity.New(w.db, verifier).Create(ctx, identity.NewUser{
		Email: "golden@facile.studio", Password: wirePassword, Name: "Golden Person"})
	if err != nil {
		w.t.Fatal(err)
	}
	w.hide(user.ID, "{user}")
}

// hideKeys masks the per-run signing key's id and modulus.
func (w *wire) hideKeys() {
	w.t.Helper()
	request, _ := http.NewRequest(http.MethodGet, w.url+"/keys", nil)
	response, err := w.client.Do(request)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var set struct {
		Keys []struct{ Kid, N string } `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&set); err != nil || len(set.Keys) == 0 {
		w.t.Fatalf("reading the key set: %v", err)
	}
	for _, key := range set.Keys {
		w.hide(key.Kid, "{kid}")
		w.hide(key.N, "{modulus}")
	}
}

// get sends a GET, with a bearer when one is given.
func (w *wire) get(label, path, bearer string) (int, []byte) {
	request, _ := http.NewRequest(http.MethodGet, w.url+path, nil)
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	return w.do(label, request)
}

// form posts a form body, authenticated as client (or client:secret) with
// HTTP Basic when client is set.
func (w *wire) form(label, path, client string, values url.Values) (int, []byte) {
	request, _ := http.NewRequest(http.MethodPost, w.url+path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if client != "" {
		id, secret, found := strings.Cut(client, ":")
		if !found {
			secret = wireSecret
		}
		request.SetBasicAuth(id, secret)
	}
	return w.do(label, request)
}

// humanTokens signs the fixed human in through the browser half of the code
// flow and redeems the code, returning the access token.
func (w *wire) humanTokens() string {
	w.t.Helper()
	sum := sha256.Sum256([]byte(wireVerifier))
	query := url.Values{"client_id": {"golden-app"}, "redirect_uri": {wireRedirect}, "response_type": {"code"},
		"scope": {"openid profile email offline_access"}, "state": {"st4te"}, "nonce": {"n0nce"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	request, _ := http.NewRequest(http.MethodGet, w.url+"/authorize?"+query.Encode(), nil)
	w.do("authorize", request)
	code := w.signIn()
	_, body := w.form("token authorization_code", "/oauth/token", "golden-app", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {wireRedirect},
		"code_verifier": {wireVerifier}})
	var tokens struct{ AccessToken, RefreshToken string }
	tokens.AccessToken, tokens.RefreshToken = field(body, "access_token"), field(body, "refresh_token")
	w.hide(tokens.RefreshToken, "{refresh_token}")
	_, refreshed := w.form("token refresh_token", "/oauth/token", "golden-app", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken}})
	w.hide(field(refreshed, "refresh_token"), "{refresh_token}")
	return tokens.AccessToken
}

// signIn posts the password for the pending request and follows the
// callback to the code the relying party receives.
func (w *wire) signIn() string {
	w.t.Helper()
	body, _ := json.Marshal(map[string]string{"auth_request": w.lastLocation.Query().Get("auth_request"),
		"email": "golden@facile.studio", "password": wirePassword})
	login, _ := http.NewRequest(http.MethodPost, w.url+"/api/login", strings.NewReader(string(body)))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("X-Facile-CSRF", "1")
	_, answer := w.do("login", login)
	callback, _ := http.NewRequest(http.MethodGet, w.url+field(answer, "redirect"), nil)
	w.do("authorize callback", callback)
	return w.lastLocation.Query().Get("code")
}

// field reads one top-level string out of a JSON body, empty when absent.
func field(body []byte, key string) string {
	var object map[string]any
	_ = json.Unmarshal(body, &object)
	value, _ := object[key].(string)
	return value
}
