package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// extractAuthRequest pulls the hidden auth_request field out of the login
// form's HTML.
func extractAuthRequest(body string) string {
	marker := `name="auth_request" value="`
	start := strings.Index(body, marker)
	if start < 0 {
		return ""
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

type callbackResult struct{ code, state string }

func parseCallback(location string) callbackResult {
	parsed, err := url.Parse(location)
	if err != nil {
		return callbackResult{}
	}
	query := parsed.Query()
	return callbackResult{code: query.Get("code"), state: query.Get("state")}
}

// singleCookie is the smallest possible browser: it keeps one cookie (the
// provider sets none today, but the helper must not assume that) and follows
// nothing automatically, so every hop of the flow is asserted by hand.
type singleCookie struct {
	cookie   *http.Cookie
	lastBody *strings.Builder
	client   *http.Client
	noFollow bool
}

func (s *singleCookie) do(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	if s.lastBody == nil {
		s.lastBody = &strings.Builder{}
	}
	s.lastBody.Reset()
	if s.cookie != nil {
		request.AddCookie(s.cookie)
	}
	if s.client == nil {
		s.client = &http.Client{}
	}
	s.client.CheckRedirect = func(*http.Request, []*http.Request) error {
		if s.noFollow {
			return http.ErrUseLastResponse
		}
		return nil
	}
	response, err := s.client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", request.Method, request.URL, err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	_ = response.Body.Close()
	s.lastBody.Write(body)
	for _, cookie := range response.Cookies() {
		s.cookie = cookie
	}
	return response
}

func (s *singleCookie) get(t *testing.T, rawURL string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("building GET %s: %v", rawURL, err)
	}
	return s.do(t, request)
}

// getNoFollow is get with redirects left standing.
func (s *singleCookie) getNoFollow(t *testing.T, rawURL string) *http.Response {
	t.Helper()
	s.noFollow = true
	defer func() { s.noFollow = false }()
	return s.get(t, rawURL)
}

func (s *singleCookie) postForm(t *testing.T, rawURL string, form url.Values) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("building POST %s: %v", rawURL, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.do(t, request)
}

// postFormNoFollow is postForm with redirects left standing, so the test can
// assert on the Location the way a browser's address bar would show it.
func (s *singleCookie) postFormNoFollow(t *testing.T, rawURL string, form url.Values) *http.Response {
	t.Helper()
	s.noFollow = true
	defer func() { s.noFollow = false }()
	return s.postForm(t, rawURL, form)
}

func (f *fixture) application(clientID string, redirect string) string {
	f.t.Helper()
	secret, err := ids.Secret()
	if err != nil {
		f.t.Fatalf("minting a secret: %v", err)
	}
	hash, err := f.register.HashSecret(context.Background(), secret)
	if err != nil {
		f.t.Fatalf("hashing the secret: %v", err)
	}
	subject, err := ids.New()
	if err != nil {
		f.t.Fatalf("minting a subject: %v", err)
	}
	row := schemas.Client{
		ClientID: clientID, Subject: subject, Name: clientID, Kind: schemas.KindApplication,
		SecretHash:   hash,
		GrantTypes:   schemas.StringList{"authorization_code"},
		Scopes:       schemas.StringList{"openid", "profile", "email"},
		RedirectURIs: schemas.StringList{redirect},
	}
	if _, err := f.register.Upsert(context.Background(), row); err != nil {
		f.t.Fatalf("seeding %s: %v", clientID, err)
	}
	return secret
}

func (f *fixture) human(email, password string) {
	f.t.Helper()
	if _, err := f.users.Create(context.Background(), email, password, "Yann", true); err != nil {
		f.t.Fatalf("creating the user: %v", err)
	}
}

// pkcePair mints a verifier and its S256 challenge.
func pkcePair(t *testing.T) (verifier, challenge string) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("minting a verifier: %v", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// flowParams builds the /authorize query an app would send.
func flowParams(challenge string, extra url.Values) url.Values {
	params := url.Values{
		"client_id":             {"sablier"},
		"redirect_uri":          {"http://127.0.0.1:9999/callback"},
		"response_type":         {"code"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	for key, values := range extra {
		params[key] = values
	}
	return params
}

// driveToTheCode walks the whole browser half of the flow: authorize, sign in,
// callback, and returns the code the relying party receives.
func driveToTheCode(
	t *testing.T, f *fixture, jar *singleCookie, verifier, challenge string,
) (string, callbackResult) {
	t.Helper()
	extra := url.Values{"scope": {"openid profile email"}, "state": {"st4te"}, "nonce": {"n0nce"}}
	authorizeURL := f.issuer + "/authorize?" + flowParams(challenge, extra).Encode()

	if got := jar.get(t, authorizeURL); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d: %s", got.StatusCode, jar.lastBody.String())
	}
	id := extractAuthRequest(jar.lastBody.String())
	if id == "" {
		t.Fatal("the login page carries no auth_request parameter")
	}

	form := url.Values{
		"auth_request": {id},
		"email":        {"yann@facile.studio"},
		"password":     {"correct horse battery staple"},
	}
	loginResponse := jar.postFormNoFollow(t, f.issuer+"/login", form)
	if loginResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /login = %d: %s", loginResponse.StatusCode, jar.lastBody.String())
	}
	callback := loginResponse.Header.Get("Location")
	if !strings.HasPrefix(callback, "/authorize/callback?id=") {
		t.Fatalf("login redirected to %q, want the authorize callback", callback)
	}
	codeResponse := jar.getNoFollow(t, f.issuer+callback)
	if codeResponse.StatusCode != http.StatusFound {
		t.Fatalf("callback = %d: %s", codeResponse.StatusCode, jar.lastBody.String())
	}
	return id, parseCallback(codeResponse.Header.Get("Location"))
}

// assertIDTokenNonce checks that the ID token carries the nonce the flow sent.
func assertIDTokenNonce(t *testing.T, fixture *fixture, granted map[string]any, want string) {
	t.Helper()
	idToken, _ := granted["id_token"].(string)
	if idToken == "" {
		t.Fatal("no id_token came with the code exchange")
	}
	claims, err := fixture.verify(idToken)
	if err != nil {
		t.Fatalf("the id_token does not verify against the JWKS: %v", err)
	}
	if claims["nonce"] != want {
		t.Errorf("id_token nonce = %v, want %s", claims["nonce"], want)
	}
}

// account writes a service account into the register and returns its secret.
func (f *fixture) account(clientID string, scopes, audiences []string) string {
	f.t.Helper()
	secret, err := ids.Secret()
	if err != nil {
		f.t.Fatalf("minting a secret: %v", err)
	}
	hash, err := f.register.HashSecret(context.Background(), secret)
	if err != nil {
		f.t.Fatalf("hashing the secret: %v", err)
	}
	subject, err := ids.New()
	if err != nil {
		f.t.Fatalf("minting a subject: %v", err)
	}
	row := schemas.Client{
		ClientID: clientID, Subject: subject, Name: clientID, Kind: schemas.KindServiceAccount,
		SecretHash: hash, GrantTypes: schemas.StringList{"client_credentials"},
		Scopes: schemas.StringList(scopes), Audiences: schemas.StringList(audiences), Managed: true,
	}
	if _, err := f.register.Upsert(context.Background(), row); err != nil {
		f.t.Fatalf("seeding %s: %v", clientID, err)
	}
	return secret
}
