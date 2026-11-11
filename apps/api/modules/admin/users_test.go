package admin

import (
	"context"
	"net/http"
	"testing"
)

func TestAUserIsCreatedVerifiedAndDisabled(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/users", map[string]any{
		"email": "newhuman@example.com", "password": "their password",
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("creating answered %d: %v", response.StatusCode, body)
	}
	view, _ := body["id"]
	if view == "" {
		t.Fatal("the created user has no subject")
	}
	if verified, _ := body["email_verified"].(bool); !verified {
		t.Fatal("an operator-minted account is not verified by construction")
	}
	if _, ok := body["password_hash"]; ok {
		t.Fatal("the view leaks the password hash")
	}

	user, err := f.users.ByEmail(context.Background(), "newhuman@example.com")
	if err != nil {
		t.Fatalf("reading the account: %v", err)
	}
	response, _ = f.call(t, http.MethodPost,
		"/api/admin/users/"+user.ID+"/disable", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("disabling answered %d", response.StatusCode)
	}
	if _, err := f.users.Verify(
		context.Background(), "newhuman@example.com", "their password",
	); err == nil {
		t.Fatal("the disabled account still verifies")
	}
}

func TestAUserPasswordIsUpdatedByAdmin(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/users", map[string]any{
		"email": "pwuser@example.com", "password": "initial password",
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("creating user answered %d: %v", response.StatusCode, body)
	}
	userID, _ := body["id"].(string)

	response, _ = f.call(t, http.MethodPost, "/api/admin/users/"+userID+"/password", map[string]any{
		"password": "short",
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("short password answered %d, want 400", response.StatusCode)
	}

	response, body = f.call(t, http.MethodPost, "/api/admin/users/"+userID+"/password", map[string]any{
		"password": "brand new secure password",
	}, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("updating password answered %d: %v", response.StatusCode, body)
	}

	if _, err := f.users.Verify(context.Background(), "pwuser@example.com", "initial password"); err == nil {
		t.Fatal("old password still verifies")
	}
	if _, err := f.users.Verify(context.Background(), "pwuser@example.com", "brand new secure password"); err != nil {
		t.Fatalf("new password does not verify: %v", err)
	}
}

func TestAUserCanBeUpdatedEnabledAndDeleted(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/users", map[string]any{
		"email": "member@example.com", "password": "secure password", "name": "Initial Name",
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("creating answered %d: %v", response.StatusCode, body)
	}
	userID, _ := body["id"].(string)

	adminTrue := true
	response, body = f.call(t, http.MethodPut, "/api/admin/users/"+userID, map[string]any{
		"name": "Updated Name", "admin": &adminTrue,
	}, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("updating user answered %d: %v", response.StatusCode, body)
	}
	if name, _ := body["name"].(string); name != "Updated Name" {
		t.Fatalf("name was not updated: %v", name)
	}
	if isAdmin, _ := body["admin"].(bool); !isAdmin {
		t.Fatal("admin flag was not updated")
	}

	response, _ = f.call(t, http.MethodPost, "/api/admin/users/"+userID+"/disable", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("disabling answered %d", response.StatusCode)
	}

	response, _ = f.call(t, http.MethodPost, "/api/admin/users/"+userID+"/enable", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("enabling answered %d", response.StatusCode)
	}
	if _, err := f.users.Verify(context.Background(), "member@example.com", "secure password"); err != nil {
		t.Fatalf("re-enabled user does not verify: %v", err)
	}

	response, _ = f.call(t, http.MethodDelete, "/api/admin/users/"+userID, nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("deleting answered %d", response.StatusCode)
	}
	if _, err := f.users.ByID(context.Background(), userID); err == nil {
		t.Fatal("deleted user still exists")
	}
}
