package admin

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestEditingAClientLeavesEveryColumnTheFormDoesNotOwn is the facile-cli
// regression: saving the dashboard form once turned a public client
// confidential and zeroed its token lifetime, which broke every CLI login.
func TestEditingAClientLeavesEveryColumnTheFormDoesNotOwn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.register.Upsert(ctx, schemas.Client{
		ClientID: "facile-cli", Subject: "cli-sub", Name: "CLI", Kind: schemas.KindApplication, Public: true,
		AccessTokenTTL: 900, Managed: true, GrantTypes: schemas.StringList{"authorization_code", "refresh_token"},
		RedirectURIs:     schemas.StringList{"http://127.0.0.1/callback"},
		DirectorySyncURI: "https://cli.example/api/registre/sync",
		TokenExchange:    schemas.ExchangeGrants{{Audience: "nuage", Scopes: []string{"nuage.read"}}},
	})
	if err != nil {
		t.Fatalf("seeding the client: %v", err)
	}
	response, body := f.call(t, http.MethodPut, "/api/admin/clients/facile-cli", map[string]any{
		"name": "Facile CLI", "kind": schemas.KindServiceAccount,
		"redirect_uris": []string{"http://127.0.0.1/callback"},
		"deletion_uri":  "https://cli.example/api/registre/deleted",
	}, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("updating answered %d: %v", response.StatusCode, body)
	}
	row, err := f.register.ByClientID(ctx, "facile-cli")
	if err != nil {
		t.Fatal(err)
	}
	if !row.Public || row.AccessTokenTTL != 900 || !row.Managed || row.Kind != schemas.KindApplication ||
		row.DirectorySyncURI != "https://cli.example/api/registre/sync" || len(row.TokenExchange) != 1 || row.Subject != "cli-sub" {
		t.Fatalf("the edit changed columns it does not own: %+v", row)
	}
	if row.Name != "Facile CLI" || row.DeletionURI != "https://cli.example/api/registre/deleted" {
		t.Fatalf("the edit did not apply its own columns: %+v", row)
	}
	response, _ = f.call(t, http.MethodPut, "/api/admin/clients/facile-cli", map[string]any{
		"name": "Facile CLI", "redirect_uris": []string{"http://127.0.0.1/callback"},
		"deletion_uri": "https://cli.example/api/registre/sync",
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("a deletion URI equal to the sync URI answered %d, want 400", response.StatusCode)
	}
}

// drawerBody is exactly what ClientEditDrawer.svelte sends for an
// application: no grant_types, every other field it shows.
func drawerBody(row schemas.Client, name string) map[string]any {
	return map[string]any{
		"name": name, "kind": row.Kind, "redirect_uris": []string(row.RedirectURIs),
		"visible_roles": []string(row.VisibleRoles), "backchannel_logout_uri": row.BackchannelLogoutURI,
		"deletion_uri": row.DeletionURI, "scopes": []string(row.Scopes), "audiences": []string(row.Audiences),
	}
}

// TestTheDrawerKeepsTheDeviceGrantAndCustomLists sends the drawer's exact
// body for a device-grant client with custom scopes, audiences and roles:
// the omitted grant types must not fall back to defaults.
func TestTheDrawerKeepsTheDeviceGrantAndCustomLists(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	grants := schemas.StringList{"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:device_code"}
	seeded, err := f.register.Upsert(ctx, schemas.Client{
		ClientID: "facile-cli", Subject: "cli-sub", Name: "CLI", Kind: schemas.KindApplication, Public: true,
		GrantTypes: grants, Scopes: schemas.StringList{"openid", "profile", "email", "offline_access", "roles"},
		Audiences: schemas.StringList{"nuage", "plume"}, VisibleRoles: schemas.StringList{"admin", "editor"},
		RedirectURIs:         schemas.StringList{"http://127.0.0.1/callback"},
		BackchannelLogoutURI: "https://cli.example/logout",
	})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	response, body := f.call(t, http.MethodPut, "/api/admin/clients/facile-cli", drawerBody(seeded, "Facile CLI"), true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("saving the drawer answered %d: %v", response.StatusCode, body)
	}
	row, _ := f.register.ByClientID(ctx, "facile-cli")
	if strings.Join(row.GrantTypes, " ") != strings.Join(grants, " ") {
		t.Fatalf("grant types became %v, want %v kept", row.GrantTypes, grants)
	}
	if len(row.Scopes) != 5 || strings.Join(row.Audiences, " ") != "nuage plume" ||
		strings.Join(row.VisibleRoles, " ") != "admin editor" || !row.Public || row.Name != "Facile CLI" {
		t.Fatalf("the drawer changed what it did not edit: %+v", row)
	}
	cleared := drawerBody(row, "Facile CLI")
	cleared["backchannel_logout_uri"] = ""
	if response, _ := f.call(t, http.MethodPut, "/api/admin/clients/facile-cli", cleared, true); response.StatusCode != http.StatusOK {
		t.Fatalf("clearing the logout URI answered %d", response.StatusCode)
	}
	if row, _ := f.register.ByClientID(ctx, "facile-cli"); row.BackchannelLogoutURI != "" {
		t.Fatalf("a field sent empty was not cleared: %q", row.BackchannelLogoutURI)
	}
}
