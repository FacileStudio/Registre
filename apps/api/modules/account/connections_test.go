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
