package oidc

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const discordID = "123456789012345678"

func linkedFixture(t *testing.T) *fixture {
	t.Helper()
	fixture := newFixture(t, defaultKeyConfig())
	fixture.human("yann@facile.studio", "correct horse battery staple")
	var user schemas.User
	if err := fixture.db.Where("email = ?", "yann@facile.studio").Take(&user).Error; err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	link := schemas.SocialIdentity{Provider: "discord", Subject: discordID, UserID: user.ID}
	if err := fixture.db.Create(&link).Error; err != nil {
		t.Fatalf("linking discord: %v", err)
	}
	return fixture
}

func idTokenClaims(t *testing.T, granted map[string]any) map[string]any {
	t.Helper()
	var claims map[string]any
	decodeIDTokenPayload(t, granted["id_token"].(string), &claims)
	return claims
}

func userinfoClaims(t *testing.T, fixture *fixture, granted map[string]any) map[string]any {
	t.Helper()
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
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("GET /userinfo = %d: %s (%v)", response.StatusCode, body, err)
	}
	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil {
		t.Fatalf("decoding userinfo: %v", err)
	}
	return claims
}

func discordOf(claims map[string]any) (string, bool) {
	connections, ok := claims["connections"].(map[string]any)
	if !ok {
		return "", false
	}
	value, _ := connections["discord"].(string)
	return value, true
}

func TestAllowedClientReceivesConnectionsInIDTokenAndUserinfo(t *testing.T) {
	fixture := linkedFixture(t)
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback", "connections")
	fixture.allowProvider("sablier", "discord")
	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}

	granted := signInAndExchange(t, fixture, client, "openid connections")
	if got, _ := discordOf(idTokenClaims(t, granted)); got != discordID {
		t.Fatalf("id token discord = %q, want the linked id", got)
	}
	if got, _ := discordOf(userinfoClaims(t, fixture, granted)); got != discordID {
		t.Fatalf("userinfo discord = %q, want the linked id", got)
	}

	without := signInAndExchange(t, fixture, client, "openid")
	if _, present := idTokenClaims(t, without)["connections"]; present {
		t.Fatal("the connections claim was emitted without the scope")
	}
}

func TestClientThatDidNotOptInReceivesNoConnections(t *testing.T) {
	fixture := linkedFixture(t)
	secret := fixture.application("nuage", "http://127.0.0.1:9999/callback", "connections")
	client := testClient{id: "nuage", secret: secret, redirect: redirectOf("nuage")}

	granted := signInAndExchange(t, fixture, client, "openid connections")
	if _, present := idTokenClaims(t, granted)["connections"]; present {
		t.Fatal("the id token leaked connections to a client that did not opt in")
	}
	if _, present := userinfoClaims(t, fixture, granted)["connections"]; present {
		t.Fatal("userinfo leaked connections to a client that did not opt in")
	}
}

func TestClientOptedIntoAnotherProviderReceivesNoConnections(t *testing.T) {
	fixture := linkedFixture(t)
	secret := fixture.application("plume", "http://127.0.0.1:9999/callback", "connections")
	fixture.allowProvider("plume", "github")
	client := testClient{id: "plume", secret: secret, redirect: redirectOf("plume")}

	granted := signInAndExchange(t, fixture, client, "openid connections")
	if _, present := idTokenClaims(t, granted)["connections"]; present {
		t.Fatal("the id token leaked connections to a client opted into another provider")
	}
}
