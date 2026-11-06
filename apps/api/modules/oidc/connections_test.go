package oidc

import (
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestIDTokenCarriesConnectionsOnlyWhenTheScopeIsGranted(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback", "connections")
	fixture.human("yann@facile.studio", "correct horse battery staple")
	var user schemas.User
	if err := fixture.db.Where("email = ?", "yann@facile.studio").Take(&user).Error; err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	link := schemas.SocialIdentity{Provider: "discord", Subject: "123456789012345678", UserID: user.ID}
	if err := fixture.db.Create(&link).Error; err != nil {
		t.Fatalf("linking discord: %v", err)
	}
	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}

	with := signInAndExchange(t, fixture, client, "openid connections")
	var granted struct {
		Connections map[string]string `json:"connections"`
	}
	decodeIDTokenPayload(t, with["id_token"].(string), &granted)
	if granted.Connections["discord"] != "123456789012345678" {
		t.Fatalf("connections = %v, want the linked discord id", granted.Connections)
	}

	without := signInAndExchange(t, fixture, client, "openid")
	var absent map[string]any
	decodeIDTokenPayload(t, without["id_token"].(string), &absent)
	if _, present := absent["connections"]; present {
		t.Fatal("the connections claim was emitted without the scope")
	}
}
