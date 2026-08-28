package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestAClientIsMintedWithASecretShownOnceAndRotates(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "courrier-machine", "name": "Courrier agent",
		"kind": schemas.KindServiceAccount,
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("minting answered %d: %v", response.StatusCode, body)
	}
	secret, _ := body["secret"].(string)
	if secret == "" {
		t.Fatal("minting did not show a secret")
	}
	if _, err := f.register.Authenticate(
		context.Background(), "courrier-machine", secret,
	); err != nil {
		t.Fatalf("the shown secret does not authenticate: %v", err)
	}

	response, body = f.call(t, http.MethodPost,
		"/api/admin/clients/courrier-machine/secret", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("rotating answered %d: %v", response.StatusCode, body)
	}
	fresh, _ := body["secret"].(string)
	if fresh == "" || fresh == secret {
		t.Fatal("rotation did not produce a new secret")
	}
	if _, err := f.register.Authenticate(
		context.Background(), "courrier-machine", fresh,
	); err != nil {
		t.Fatalf("the rotated secret does not authenticate: %v", err)
	}
	if _, err := f.register.Authenticate(
		context.Background(), "courrier-machine", secret,
	); err == nil {
		t.Fatal("the old secret still authenticates")
	}
}

func TestAClientCanBeUpdatedDisabledEnabledAndDeleted(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "test-app", "name": "Test App",
		"kind":          schemas.KindApplication,
		"redirect_uris": []string{"https://app.example.com/callback"},
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("minting answered %d: %v", response.StatusCode, body)
	}

	response, body = f.call(t, http.MethodPut, "/api/admin/clients/test-app", map[string]any{
		"client_id": "test-app", "name": "Updated Test App",
		"kind":          schemas.KindApplication,
		"redirect_uris": []string{"https://app.example.com/oauth2/callback"},
		"visible_roles": []string{"admin", "member"},
	}, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("updating answered %d: %v", response.StatusCode, body)
	}
	if name, _ := body["name"].(string); name != "Updated Test App" {
		t.Fatalf("client name not updated: %v", name)
	}

	response, _ = f.call(t, http.MethodPost, "/api/admin/clients/test-app/disable", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("disabling answered %d", response.StatusCode)
	}
	if _, err := f.register.ByClientID(context.Background(), "test-app"); err == nil {
		t.Fatal("disabled client still returns as enabled")
	}

	response, _ = f.call(t, http.MethodPost, "/api/admin/clients/test-app/enable", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("enabling answered %d", response.StatusCode)
	}
	if _, err := f.register.ByClientID(context.Background(), "test-app"); err != nil {
		t.Fatalf("re-enabled client failed lookup: %v", err)
	}

	response, _ = f.call(t, http.MethodDelete, "/api/admin/clients/test-app", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("deleting answered %d", response.StatusCode)
	}
}
