package oidc

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"strings"
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

// TestIDTokenCarriesThePictureUnderProfile is the sync contract. porte reads
// `picture` from the ID token at every sign-in and fetches it into the app's
// own store, so this claim is the only thing that carries a new picture to the
// suite. It must be absolute, and it must stay behind the profile scope.
func TestIDTokenCarriesThePictureUnderProfile(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.application("sablier", "http://127.0.0.1:9999/callback")
	fixture.human("yann@facile.studio", "correct horse battery staple")

	var user schemas.User
	if err := fixture.db.Where("email = ?", "yann@facile.studio").Take(&user).Error; err != nil {
		t.Fatalf("reading the user: %v", err)
	}
	stored, err := avatars.New(fixture.db, fixture.issuer).Put(context.Background(), user.ID, squarePNG(t))
	if err != nil {
		t.Fatalf("storing the picture: %v", err)
	}

	client := testClient{id: "sablier", secret: secret, redirect: redirectOf("sablier")}
	var claims struct {
		Picture string `json:"picture"`
	}
	decodeIDTokenPayload(t, signInAndExchange(t, fixture, client, "openid profile")["id_token"].(string), &claims)
	if claims.Picture != stored || !strings.HasPrefix(claims.Picture, fixture.issuer+avatars.Path) {
		t.Errorf("id_token picture = %q, want the stored %q under the issuer", claims.Picture, stored)
	}

	claims.Picture = ""
	decodeIDTokenPayload(t, signInAndExchange(t, fixture, client, "openid")["id_token"].(string), &claims)
	if claims.Picture != "" {
		t.Errorf("id_token carried picture %q for a request that asked only for openid", claims.Picture)
	}
}
