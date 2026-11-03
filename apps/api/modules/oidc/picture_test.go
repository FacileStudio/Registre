package oidc

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// squarePNG is a blank 32-pixel square, an upload Normalize accepts.
func squarePNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	return out.Bytes()
}

// TestIDTokenCarriesThePictureUnderProfile is the sync contract. Apps read
// `picture` from the ID token at sign-in and store it, so this claim is the
// only thing that carries a picture to the suite. It must be the stable per-user URL, which apps keep and which always
// serves the current picture, and it must stay behind the profile scope.
func TestIDTokenCarriesThePictureUnderProfile(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	var user schemas.User
	if err := fixture.db.Where("email = ?", "yann@facile.studio").Take(&user).Error; err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	if _, err := avatars.New(fixture.db, fixture.issuer).Put(context.Background(), user.ID, squarePNG(t)); err != nil {
		t.Fatalf("storing the picture: %v", err)
	}
	stable := fixture.issuer + avatars.Path + user.ID

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	var claims struct {
		Picture string `json:"picture"`
	}
	decodeIDTokenPayload(t, signInAndExchange(t, fixture, client, "openid profile")["id_token"].(string), &claims)
	if claims.Picture != stable {
		t.Errorf("id_token picture = %q, want the stable %q, which never changes on upload", claims.Picture, stable)
	}

	if err := avatars.New(fixture.db, fixture.issuer).Remove(context.Background(), user.ID); err != nil {
		t.Fatalf("removing the picture: %v", err)
	}
	claims.Picture = ""
	decodeIDTokenPayload(t, signInAndExchange(t, fixture, client, "openid profile")["id_token"].(string), &claims)
	if claims.Picture != stable {
		t.Errorf("without a picture the id_token carried %q, want the stable %q so a first upload is instant",
			claims.Picture, stable)
	}

	claims.Picture = ""
	decodeIDTokenPayload(t, signInAndExchange(t, fixture, client, "openid")["id_token"].(string), &claims)
	if claims.Picture != "" {
		t.Errorf("id_token carried picture %q for a request that asked only for openid", claims.Picture)
	}
}
