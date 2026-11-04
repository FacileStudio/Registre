package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
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

// singleCookie is the smallest possible browser: it keeps the latest cookie of
// each name (the flow cookie /authorize sets and the account cookie login
// sets) and follows nothing automatically, so every hop of the flow is
// asserted by hand.
type singleCookie struct {
	cookies  map[string]*http.Cookie
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
	for _, cookie := range s.cookies {
		request.AddCookie(cookie)
	}
	if s.client == nil {
		s.client = &http.Client{}
	}
	s.client.CheckRedirect = s.follow
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
	s.keep(response.Cookies())
	return response
}

// follow is the redirect policy: keep what the hop set, as a browser does,
// and send it on the next request, unless the test asked to stop.
func (s *singleCookie) follow(next *http.Request, _ []*http.Request) error {
	if s.noFollow {
		return http.ErrUseLastResponse
	}
	if next.Response != nil {
		s.keep(next.Response.Cookies())
	}
	next.Header.Del("Cookie")
	for _, cookie := range s.cookies {
		next.AddCookie(cookie)
	}
	return nil
}

// keep stores each cookie a response set and drops each one it expired.
func (s *singleCookie) keep(cookies []*http.Cookie) {
	if s.cookies == nil {
		s.cookies = map[string]*http.Cookie{}
	}
	for _, cookie := range cookies {
		if cookie.MaxAge < 0 {
			delete(s.cookies, cookie.Name)
			continue
		}
		s.cookies[cookie.Name] = cookie
	}
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

// postJSONNoFollow posts a JSON body with redirects left standing, so the
// test can assert on the Location the way a browser's address bar would show
// it.
func (s *singleCookie) postJSONNoFollow(t *testing.T, rawURL string, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building POST %s: %v", rawURL, err)
	}
	request.Header.Set("Content-Type", "application/json")
	s.noFollow = true
	defer func() { s.noFollow = false }()
	return s.do(t, request)
}

// resumeSession is the login page finishing a flow with the session the
// browser already holds: a POST with the CSRF header and the flow in the body.
func (s *singleCookie) resumeSession(t *testing.T, issuer, authRequest string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, issuer+"/api/login/session",
		strings.NewReader(`{"auth_request":"`+authRequest+`"}`))
	if err != nil {
		t.Fatalf("building POST /api/login/session: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Facile-CSRF", "1")
	s.noFollow = true
	defer func() { s.noFollow = false }()
	return s.do(t, request)
}

// submitLogin signs a human in through the client app's endpoint: pull the
// pending auth request out of the authorize page, POST credentials as JSON,
// follow the returned redirect to the callback.
func (f *fixture) submitLogin(jar *singleCookie, email, password string) callbackResult {
	f.t.Helper()
	id := extractAuthRequest(jar.lastBody.String())
	if id == "" {
		f.t.Fatal("the authorize page carries no auth_request parameter")
	}
	payload, err := json.Marshal(map[string]string{
		"auth_request": id,
		"email":        email,
		"password":     password,
	})
	if err != nil {
		f.t.Fatalf("encoding the login body: %v", err)
	}
	response := jar.postJSONNoFollow(f.t, f.issuer+"/api/login", string(payload))
	if response.StatusCode != http.StatusOK {
		f.t.Fatalf("POST /api/login = %d: %s", response.StatusCode, jar.lastBody.String())
	}
	var body struct {
		Redirect string `json:"redirect"`
	}
	if err := json.Unmarshal([]byte(jar.lastBody.String()), &body); err != nil || body.Redirect == "" {
		f.t.Fatalf("POST /api/login answered %q", jar.lastBody.String())
	}
	jar.lastBody.Reset()
	codeResponse := jar.getNoFollow(f.t, f.issuer+body.Redirect)
	if codeResponse.StatusCode != http.StatusFound {
		f.t.Fatalf("callback = %d: %s", codeResponse.StatusCode, jar.lastBody.String())
	}
	return parseCallback(codeResponse.Header.Get("Location"))
}

func (f *fixture) application(clientID string, redirect string, extraScopes ...string) string {
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
	scopes := schemas.StringList{"openid", "profile", "email"}
	for _, scope := range extraScopes {
		if !scopes.Contains(scope) {
			scopes = append(scopes, scope)
		}
	}
	row := schemas.Client{
		ClientID: clientID, Subject: subject, Name: clientID, Kind: schemas.KindApplication,
		SecretHash:   hash,
		GrantTypes:   schemas.StringList{"authorization_code", "refresh_token"},
		Scopes:       scopes,
		RedirectURIs: schemas.StringList{redirect},
	}
	if _, err := f.register.Upsert(context.Background(), row); err != nil {
		f.t.Fatalf("seeding %s: %v", clientID, err)
	}
	return secret
}

func (f *fixture) human(email, password string) {
	f.t.Helper()
	if _, err := f.users.Create(context.Background(), identity.NewUser{
		Email: email, Password: password, Name: "Yann", Admin: true,
	}); err != nil {
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
) callbackResult {
	t.Helper()
	extra := url.Values{"scope": {"openid profile email"}, "state": {"st4te"}, "nonce": {"n0nce"}}
	authorizeURL := f.issuer + "/authorize?" + flowParams(challenge, extra).Encode()

	if got := jar.get(t, authorizeURL); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d: %s", got.StatusCode, jar.lastBody.String())
	}
	return f.submitLogin(jar, "yann@facile.studio", "correct horse battery staple")
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
