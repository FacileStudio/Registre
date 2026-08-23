package oidc

import (
	"context"
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

	authRequestID, back := driveToTheCode(t, fixture, jar, verifier, challenge)
	if back.state != "st4te" {
		t.Errorf("state = %q, want st4te", back.state)
	}
	if back.code == "" {
		t.Fatal("no code came back to the relying party")
	}
	_ = authRequestID

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
	form := url.Values{
		"auth_request": {extractAuthRequest(jar.lastBody.String())},
		"email":        {"yann@facile.studio"},
		"password":     {"wrong password entirely"},
	}
	got := jar.postFormNoFollow(t, fixture.issuer+"/login", form)
	if got.StatusCode != http.StatusOK || !strings.Contains(jar.lastBody.String(), "Wrong email or password") {
		t.Fatalf("a wrong password answered %d %q, want the form with the generic refusal",
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
