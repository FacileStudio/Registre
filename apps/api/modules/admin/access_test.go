package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestAccessIsManagedThroughTheAPIAndLogged(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.register.Upsert(ctx, schemas.Client{
		ClientID: "ardoise", Subject: "sub-ardoise", Name: "Ardoise", Kind: schemas.KindApplication,
		VisibleRoles: schemas.StringList{"admin"},
	}); err != nil {
		t.Fatalf("registering ardoise: %v", err)
	}
	if response, body := f.call(t, http.MethodPut, "/api/admin/clients/ardoise/access",
		map[string]any{"access_policy": "assigned"}, true); response.StatusCode != http.StatusOK {
		t.Fatalf("restricting answered %d: %v", response.StatusCode, body)
	}
	path := "/api/admin/clients/ardoise/grants/" + f.operator.ID
	if response, body := f.call(t, http.MethodPut, path, map[string]any{"roles": []string{"admin"}}, true); response.StatusCode != http.StatusOK {
		t.Fatalf("granting answered %d: %v", response.StatusCode, body)
	}
	if response, _ := f.call(t, http.MethodPut, path, map[string]any{"roles": []string{"owner"}}, true); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("an undeclared role answered %d, want 400", response.StatusCode)
	}
	if response, _ := f.call(t, http.MethodPut, "/api/admin/clients/registre/access",
		map[string]any{"access_policy": "assigned"}, true); response.StatusCode == http.StatusOK {
		t.Fatal("registre's own client was restricted")
	}
	response, body := f.call(t, http.MethodGet, "/api/admin/users/"+f.operator.ID, nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reading the user answered %d", response.StatusCode)
	}
	apps, _ := body["apps"].([]any)
	if len(apps) != 1 || apps[0].(map[string]any)["granted"] != true {
		t.Fatalf("the user's apps = %v, want ardoise granted", apps)
	}
	var count int64
	if err := f.db.Model(&schemas.AdminEvent{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("admin events = %d (%v), want 2: the policy change and the grant", count, err)
	}
}
