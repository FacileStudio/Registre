package account

import (
	"context"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

type stubProvider struct{}

func (stubProvider) Name() string { return "discord" }
func (stubProvider) AuthURL(state, _, _ string) string {
	return "https://discord.test/authorize?state=" + state
}
func (stubProvider) Exchange(context.Context, string, string, string) (social.Profile, error) {
	return social.Profile{}, nil
}

func listed(t *testing.T, f fixture) []any {
	t.Helper()
	response, body := f.send(t, http.MethodGet, "/api/account/connections", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET connections = %d", response.StatusCode)
	}
	items, _ := body["connections"].([]any)
	return items
}

func TestNothingAboutADiscordAppearsUntilAClientOptsIn(t *testing.T) {
	f := newFixture(t)
	if got := listed(t, f); len(got) != 0 {
		t.Fatalf("connections = %v, want none while no client opts in", got)
	}
	row := schemas.Client{
		ClientID: "place", Subject: "place-subject", Name: "Place", Kind: schemas.KindApplication,
		LoginProviders: schemas.StringList{"discord"},
	}
	if err := f.db.Create(&row).Error; err != nil {
		t.Fatalf("creating the client: %v", err)
	}
	got := listed(t, f)
	if len(got) != 1 {
		t.Fatalf("connections = %v, want discord once a client opts in", got)
	}
	if item, _ := got[0].(map[string]any); item["provider"] != "discord" || item["linked"] != false {
		t.Fatalf("connection = %v, want discord, not linked", got[0])
	}
}

func TestLinkingAndUnlinkingNeedAFreshFactor(t *testing.T) {
	f := newFixture(t)
	response, body := f.send(t, http.MethodPost, "/api/account/connections/discord/link", map[string]any{})
	if response.StatusCode != http.StatusUnauthorized || body["error"] != "reauth_required" {
		t.Fatalf("link on a stale session = %d %v, want 401 reauth_required", response.StatusCode, body)
	}
	response, body = f.send(t, http.MethodDelete, "/api/account/connections/discord", nil)
	if response.StatusCode != http.StatusUnauthorized || body["error"] != "reauth_required" {
		t.Fatalf("unlink on a stale session = %d %v, want 401 reauth_required", response.StatusCode, body)
	}
}

func optIn(t *testing.T, f fixture) {
	t.Helper()
	row := schemas.Client{
		ClientID: "place", Subject: "place-subject", Name: "Place", Kind: schemas.KindApplication,
		LoginProviders: schemas.StringList{"discord"},
		RedirectURIs:   schemas.StringList{"https://place.facile.studio/api/auth/callback"},
	}
	if err := f.db.Create(&row).Error; err != nil {
		t.Fatalf("creating the client: %v", err)
	}
	response, body := f.send(t, http.MethodPost, "/api/account/reauth", map[string]string{"password": holderPassword})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reauth = %d %v", response.StatusCode, body)
	}
}

func TestLinkAcceptsARegisteredReturnToAndRefusesTheRest(t *testing.T) {
	f := newFixture(t)
	optIn(t, f)
	path := "/api/account/connections/discord/link"
	response, body := f.send(t, http.MethodPost, path, map[string]string{"return_to": "https://place.facile.studio/settings"})
	if response.StatusCode != http.StatusOK || body["url"] == "" {
		t.Fatalf("link with a registered return_to = %d %v", response.StatusCode, body)
	}
	var stored schemas.SocialState
	if err := f.db.Where("user_id = ?", f.user).Take(&stored).Error; err != nil || stored.ReturnTo != "https://place.facile.studio/settings" {
		t.Fatalf("stored state = %+v, %v; want the return_to kept", stored, err)
	}
	for _, bad := range []string{"https://evil.example/", "javascript:alert(1)", "/account/security", "https://a@place.facile.studio/"} {
		response, body = f.send(t, http.MethodPost, path, map[string]string{"return_to": bad})
		if response.StatusCode != http.StatusBadRequest || body["error"] != "invalid_return_to" {
			t.Errorf("link with %q = %d %v, want 400 invalid_return_to", bad, response.StatusCode, body)
		}
	}
	if response, _ = f.send(t, http.MethodPost, path, map[string]any{}); response.StatusCode != http.StatusOK {
		t.Fatalf("link without a return_to = %d, want 200", response.StatusCode)
	}
}

func TestReturnAnswersOnlyForALinkedProviderAndAValidTarget(t *testing.T) {
	f := newFixture(t)
	optIn(t, f)
	path := "/api/account/connections/discord/return?return_to="
	good := "https%3A%2F%2Fplace.facile.studio%2Fsettings%3Ftab%3D1"
	if response, _ := f.send(t, http.MethodGet, path+good, nil); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("return while not linked = %d, want 400", response.StatusCode)
	}
	if err := f.db.Create(&schemas.SocialIdentity{Provider: "discord", Subject: "9", UserID: f.user}).Error; err != nil {
		t.Fatal(err)
	}
	response, body := f.send(t, http.MethodGet, path+good, nil)
	if response.StatusCode != http.StatusOK || body["url"] != "https://place.facile.studio/settings?connection=linked&tab=1" {
		t.Fatalf("return = %d %v", response.StatusCode, body)
	}
	if response, _ = f.send(t, http.MethodGet, path+"https%3A%2F%2Fevil.example%2F", nil); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("return to a foreign host = %d, want 400", response.StatusCode)
	}
}
