package oidc

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// What a relying party actually reads. porte builds its login claims from the
// ID token and nowhere else, so these assertions are the contract between
// registre and every app in the suite.

// TestIDTokenCarriesTheEmailClaim is the regression test for the outage of
// 2026-08-25, when every browser login in the suite failed with "email claim
// required". porte builds its login claims from the ID token and nowhere else,
// so an ID token without an address is a suite-wide lockout. The userinfo test
// above passed throughout: it covers the endpoint porte's login never calls.
func TestIDTokenCarriesTheEmailClaim(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	granted := signInAndExchange(t, fixture, client, "openid email profile")

	raw, ok := granted["id_token"].(string)
	if !ok || raw == "" {
		t.Fatal("the token response carried no id_token")
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	decodeIDTokenPayload(t, raw, &claims)
	if claims.Email != "yann@facile.studio" {
		t.Errorf("id_token email = %q, want yann@facile.studio", claims.Email)
	}
	if !claims.EmailVerified {
		t.Error("id_token email_verified is false; porte's address fallback " +
			"cannot fire, so every existing account forks into a new empty one")
	}
}

// TestIDTokenOmitsUnrequestedClaims proves the claims stay scope-gated. An app
// that asks for neither profile nor email must receive neither, or the fix for
// the lockout above becomes a standing leak into a token that gets logged.
func TestIDTokenOmitsUnrequestedClaims(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	granted := signInAndExchange(t, fixture, client, "openid")

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	decodeIDTokenPayload(t, granted["id_token"].(string), &claims)
	if claims.Email != "" {
		t.Errorf("id_token carried email %q for a request that asked only for openid", claims.Email)
	}
	if claims.Name != "" {
		t.Errorf("id_token carried name %q for a request that asked only for openid", claims.Name)
	}
}

// decodeIDTokenPayload unmarshals a JWT's claim set into out. The signature is
// checked elsewhere; this reads what a relying party would read.
func decodeIDTokenPayload(t *testing.T, token string, out any) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("id_token has %d segments, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decoding the id_token payload: %v", err)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		t.Fatalf("unmarshalling the id_token payload %s: %v", payload, err)
	}
}

// TestIDTokenEmitsEmptyRolesRatherThanOmitting pins the difference between an
// absent claim and an empty one. porte reads an absent roles claim as a
// misconfigured provider and fails the login outright; an empty list is the
// honest answer for a user whose roles this client may not see. Sablier is the
// app that asks, via OIDC_CLAIMS_SCOPE=roles.
func TestIDTokenEmitsEmptyRolesRatherThanOmitting(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback", "roles")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	granted := signInAndExchange(t, fixture, client, "openid email roles")

	var claims struct {
		Roles []string `json:"roles"`
	}
	decodeIDTokenPayload(t, granted["id_token"].(string), &claims)
	if claims.Roles == nil {
		t.Fatal("the roles claim is absent; porte reads that as a missing scope mapping and refuses the login")
	}
	if len(claims.Roles) != 0 {
		t.Errorf("roles = %v, want empty for a user holding none visible to this client", claims.Roles)
	}
}
