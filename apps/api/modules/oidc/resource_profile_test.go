package oidc

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func (w *resourceWorld) resourceClaims(t *testing.T) map[string]any {
	t.Helper()
	verifier, back := w.codeFor(url.Values{"scope": {"openid courrier.read"}, "resource": {courrierMCP}})
	granted, status := w.redeem(verifier, back.Query().Get("code"), url.Values{"resource": {courrierMCP}})
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d: %v", status, granted)
	}
	return mustClaims(t, w.fixture, granted["access_token"])
}

func TestAResourceBoundTokenCarriesWhatTheLoginProvisionsFrom(t *testing.T) {
	world := newResourceWorld(t)
	claims := world.resourceClaims(t)
	if claims["email"] != "yann@facile.studio" || claims["email_verified"] != true {
		t.Errorf("email = %v, email_verified = %v; want the verified address", claims["email"], claims["email_verified"])
	}
	if claims["name"] != "Yann" || claims["locale"] != "en" {
		t.Errorf("name = %v, locale = %v; want the profile", claims["name"], claims["locale"])
	}
	if picture, _ := claims["picture"].(string); !strings.HasPrefix(picture, world.issuer) {
		t.Errorf("picture = %v, want registre's stable picture URL", claims["picture"])
	}
	for _, absent := range []string{"preferred_username", "roles", "given_name"} {
		if value, has := claims[absent]; has {
			t.Errorf("%s = %v rides a resource token, which keeps to what an account needs", absent, value)
		}
	}
}

func TestAResourceBoundTokenOmitsAnUnverifiedEmail(t *testing.T) {
	world := newResourceWorld(t)
	if err := world.db.Model(&schemas.User{}).Where("email = ?", "yann@facile.studio").
		Update("email_verified", false).Error; err != nil {
		t.Fatalf("unverifying the address: %v", err)
	}
	claims := world.resourceClaims(t)
	for _, absent := range []string{"email", "email_verified"} {
		if value, has := claims[absent]; has {
			t.Errorf("%s = %v reached a resource token for an unverified address", absent, value)
		}
	}
	if claims["name"] != "Yann" {
		t.Errorf("name = %v, want the profile kept", claims["name"])
	}
}

func TestAPlainTokenCarriesNoProfile(t *testing.T) {
	world := newResourceWorld(t)
	verifier, back := world.codeFor(url.Values{"scope": {"openid email profile"}})
	granted, status := world.redeem(verifier, back.Query().Get("code"), nil)
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d: %v", status, granted)
	}
	claims := mustClaims(t, world.fixture, granted["access_token"])
	for _, absent := range []string{"email", "email_verified", "name", "picture", "locale"} {
		if value, has := claims[absent]; has {
			t.Errorf("a plain access token carries %s = %v", absent, value)
		}
	}
}

func TestAnExchangeForAResourceCarriesTheProfile(t *testing.T) {
	world := newExchangeWorld(t)
	world.declareResource("courrier", courrierMCP, "courrier.read", "courrier.write")
	granted, status := world.exchange(world.access, url.Values{"audience": nil, "resource": {courrierMCP}})
	if status != http.StatusOK {
		t.Fatalf("exchange = %d: %v", status, granted)
	}
	claims := mustClaims(t, world.fixture, granted["access_token"])
	if claims["email"] != "yann@facile.studio" || claims["email_verified"] != true || claims["name"] != "Yann" {
		t.Errorf("claims = %v, want the verified email and the name", claims)
	}
	if _, has := world.exchangedClaims()["email_verified"]; has {
		t.Error("an exchange for an app, not its MCP resource, gained the provisioning claims")
	}
}

func TestADisabledSubjectGetsNoProvisioningClaims(t *testing.T) {
	world := newResourceWorld(t)
	var user schemas.User
	if err := world.db.Where("email = ?", "yann@facile.studio").Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := world.db.Model(&user).Update("disabled_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	claims, err := world.storage.provisioningClaims(context.Background(), user.ID)
	if err != nil || claims != nil {
		t.Fatalf("claims = %v, %v; want none for a disabled account", claims, err)
	}
}
