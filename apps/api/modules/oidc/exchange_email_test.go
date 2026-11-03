package oidc

import (
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// exchangedClaims trades the world's subject token for one addressed to
// courrier and returns the verified claims of the delegated token.
func (w *exchangeWorld) exchangedClaims() map[string]any {
	w.t.Helper()
	granted, status := w.exchange(w.access, nil)
	if status != http.StatusOK {
		w.t.Fatalf("exchange = %d: %v", status, granted)
	}
	claims, err := w.verify(granted["access_token"].(string))
	if err != nil {
		w.t.Fatalf("verifying the delegated token: %v", err)
	}
	return claims
}

func TestAnExchangedTokenCarriesTheVerifiedEmailAndAPlainOneDoesNot(t *testing.T) {
	world := newExchangeWorld(t)
	if email := world.exchangedClaims()["email"]; email != "yann@facile.studio" {
		t.Errorf("exchanged email = %v, want yann@facile.studio", email)
	}
	plain, err := world.verify(world.access)
	if err != nil {
		t.Fatalf("verifying the subject token: %v", err)
	}
	if email, has := plain["email"]; has {
		t.Errorf("a plain access token carries email = %v, want none", email)
	}
}

func TestAnExchangedTokenOmitsAnUnverifiedEmail(t *testing.T) {
	world := newExchangeWorld(t)
	err := world.db.Model(&schemas.User{}).Where("id = ?", world.userID).Update("email_verified", false).Error
	if err != nil {
		t.Fatalf("unverifying the address: %v", err)
	}
	if email, has := world.exchangedClaims()["email"]; has {
		t.Errorf("an unverified address reached the token: %v", email)
	}
}
