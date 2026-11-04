package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
)

// The phase 1 browser flow, end to end: an application client sends a human
// to /authorize with PKCE, the human signs in on the login page, the code
// comes back to the callback, and the token endpoint exchanges it for a JWT
// naming the human as the subject.

func TestAHumanSignsInAndGetsAToken(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	clientSecret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	verifier, challenge := pkcePair(t)
	jar := &singleCookie{}

	back := driveToTheCode(t, fixture, jar, verifier, challenge)
	if back.state != "st4te" {
		t.Errorf("state = %q, want st4te", back.state)
	}
	if back.code == "" {
		t.Fatal("no code came back to the relying party")
	}

	granted, status := fixture.post("/oauth/token", "sablier", clientSecret, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {back.code},
		"redirect_uri":  {"http://127.0.0.1:9999/callback"},
		"code_verifier": {verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s: %s", status, granted["error"], granted["error_description"])
	}
	token, _ := granted["access_token"].(string)
	claims, err := fixture.verify(token)
	if err != nil {
		t.Fatalf("the published key set does not verify the token registre just signed: %v", err)
	}
	users := identity.New(fixture.db, nil)
	if _, err := users.ByID(context.Background(), claims["sub"].(string)); err != nil {
		t.Fatalf("the token's subject is not a registre user: %v", err)
	}
	if claims["iss"] != fixture.issuer {
		t.Errorf("iss = %v, want %s", claims["iss"], fixture.issuer)
	}
	assertIDTokenNonce(t, fixture, granted, "n0nce")
}

func TestAWrongPasswordDoesNotAuthenticateTheFlow(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	_, challenge := pkcePair(t)
	jar := &singleCookie{}
	authorizeURL := fixture.issuer + "/authorize?" + flowParams(challenge, url.Values{"scope": {"openid"}}).Encode()

	if got := jar.get(t, authorizeURL); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize = %d", got.StatusCode)
	}
	payload, err := json.Marshal(map[string]string{
		"auth_request": extractAuthRequest(jar.lastBody.String()),
		"email":        "yann@facile.studio",
		"password":     "wrong password entirely",
	})
	if err != nil {
		t.Fatalf("encoding the login body: %v", err)
	}
	got := jar.postJSONNoFollow(t, fixture.issuer+"/api/login", string(payload))
	if got.StatusCode != http.StatusUnauthorized || !strings.Contains(jar.lastBody.String(), `"error":"invalid"`) {
		t.Fatalf("a wrong password answered %d %q, want the generic JSON refusal",
			got.StatusCode, jar.lastBody.String())
	}
}

func TestAnUnknownClientIsRefusedBeforeAnyRedirect(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	_, challenge := pkcePair(t)
	authorizeURL := fixture.issuer + "/authorize?" + url.Values{
		"client_id":             {"no-such-app"},
		"redirect_uri":          {"http://evil.example/callback"},
		"response_type":         {"code"},
		"scope":                 {"openid"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()
	got := (&singleCookie{}).get(t, authorizeURL)
	if got.StatusCode < 400 {
		t.Fatalf("an unknown client answered %d, want a refusal", got.StatusCode)
	}
}

func driveSSOToCode(t *testing.T, f *fixture, jar *singleCookie, clientID, redirect string) (string, string) {
	t.Helper()
	verifier, challenge := pkcePair(t)
	authorizeURL := f.issuer + "/authorize?" + url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"response_type":         {"code"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile email"},
		"state":                 {"st4te2"},
		"nonce":                 {"n0nce2"},
	}.Encode()
	if got := jar.get(t, authorizeURL); got.StatusCode != http.StatusOK {
		t.Fatalf("GET /authorize for %s = %d", clientID, got.StatusCode)
	}
	authReq := extractAuthRequest(jar.lastBody.String())
	if authReq == "" {
		t.Fatalf("no auth_request for %s", clientID)
	}
	sessionRes := jar.resumeSession(t, f.issuer, authReq)
	if sessionRes.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login/session = %d: %s", sessionRes.StatusCode, jar.lastBody.String())
	}
	var sessionAnswer struct {
		Redirect string `json:"redirect"`
	}
	if err := json.Unmarshal([]byte(jar.lastBody.String()), &sessionAnswer); err != nil || sessionAnswer.Redirect == "" {
		t.Fatalf("session check answered %q", jar.lastBody.String())
	}
	jar.lastBody.Reset()
	codeResponse := jar.getNoFollow(t, f.issuer+sessionAnswer.Redirect)
	if codeResponse.StatusCode != http.StatusFound {
		t.Fatalf("app callback = %d: %s", codeResponse.StatusCode, jar.lastBody.String())
	}
	return verifier, parseCallback(codeResponse.Header.Get("Location")).code
}

func TestSSOReusesSessionAcrossMultipleAppsWithoutAskingForPassword(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	sablierSecret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	courrierSecret := fixture.application("courrier", "http://127.0.0.1:8888/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	verifier1, challenge1 := pkcePair(t)
	jar := &singleCookie{}

	back1 := driveToTheCode(t, fixture, jar, verifier1, challenge1)
	granted1, _ := fixture.post("/oauth/token", "sablier", sablierSecret, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {back1.code},
		"redirect_uri":  {"http://127.0.0.1:9999/callback"},
		"code_verifier": {verifier1},
	})

	verifier2, code2 := driveSSOToCode(t, fixture, jar, "courrier", "http://127.0.0.1:8888/callback")
	granted2, status2 := fixture.post("/oauth/token", "courrier", courrierSecret, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code2},
		"redirect_uri":  {"http://127.0.0.1:8888/callback"},
		"code_verifier": {verifier2},
	})
	if status2 != http.StatusOK {
		t.Fatalf("courrier token endpoint = %d: %v", status2, granted2)
	}
	token1, _ := granted1["access_token"].(string)
	claims1, _ := fixture.verify(token1)
	token2, _ := granted2["access_token"].(string)
	claims2, _ := fixture.verify(token2)
	if claims1["sub"] != claims2["sub"] {
		t.Errorf("subject mismatch: app1=%v, app2=%v", claims1["sub"], claims2["sub"])
	}
}
