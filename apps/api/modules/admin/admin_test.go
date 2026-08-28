package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestRoutesRefuseWrongAndNonOperatorCredentials(t *testing.T) {
	f := newFixture(t)

	response, _ := f.call(t, http.MethodGet, "/api/admin/users", nil, false)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no credentials answered %d, want 401", response.StatusCode)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.SetBasicAuth("operator@example.com", "the wrong password")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong password answered %d, want 401", recorder.Code)
	}

	nonAdmin, err := f.users.Create(context.Background(), identity.NewUser{
		Email: "human@example.com", Password: "human password",
	})
	if err != nil {
		t.Fatalf("creating a non-admin: %v", err)
	}
	if nonAdmin.Admin {
		t.Fatal("the non-admin holds the admin role")
	}
	request = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.SetBasicAuth("human@example.com", "human password")
	recorder = httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("a non-admin answered %d, want 401", recorder.Code)
	}
}

func TestAnInvitationFlowsFromIssueToRedemption(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/invites", map[string]any{
		"email": "invited@example.com",
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("inviting answered %d: %v", response.StatusCode, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("inviting did not show a token")
	}

	public := httptest.NewRequest(http.MethodPost, "/api/invites/redeem",
		bytes.NewBufferString(`{"token":"`+token+`","password":"chosen password"}`))
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, public)
	if recorder.Code != http.StatusOK {
		t.Fatalf("redemption answered %d", recorder.Code)
	}
	if _, err := f.users.Verify(
		context.Background(), "invited@example.com", "chosen password",
	); err != nil {
		t.Fatalf("the redeemed password does not verify: %v", err)
	}

	response, replayBody := f.call(t, http.MethodPost, "/api/invites/redeem", map[string]any{
		"token": token, "password": "another password",
	}, false)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a replayed invitation answered %d: %v, want 401", response.StatusCode, replayBody)
	}
}

func TestKeyStatusReportsTheActiveKey(t *testing.T) {
	f := newFixture(t)

	request := httptest.NewRequest(http.MethodGet, "/api/admin/keys", nil)
	request.SetBasicAuth("operator@example.com", "operator password")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("key status answered %d", recorder.Code)
	}
	var states []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &states); err != nil {
		t.Fatalf("decoding the status: %v", err)
	}
	active := false
	for _, state := range states {
		if (state.State == schemas.KeyActive || state.State == schemas.KeyPublished) && state.ID != "" {
			active = true
		}
	}
	if !active {
		t.Fatal("key status shows no signing or published key")
	}
}
