package admin

import (
	"context"
	"net/http"
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
