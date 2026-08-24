package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

// root is the throwaway key the tests seal signing keys under.
var root = []byte("0123456789abcdef0123456789abcdef")

// discard is the logger the tests keep quiet.
func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

type fixture struct {
	router      chi.Router
	operator    schemas.User
	verifier    *credentials.Verifier
	register    *clients.Service
	users       *identity.Service
	invitations *invites.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	verifier := credentials.New(2)
	register := clients.New(db, verifier)
	users := identity.New(db, verifier)
	keyring := keys.New(db, root, keys.Config{
		RotateEvery: time.Hour, PublishDelay: 10 * time.Minute,
		Retention: 2 * time.Hour,
	}, discard())
	if err := keyring.Ensure(context.Background()); err != nil {
		t.Fatalf("ensuring the keyring: %v", err)
	}
	invitations := invites.New(db, verifier)
	service := New(Services{
		Register: register, Users: users, Keys: keyring, Invites: invitations,
	}, discard())
	router := chi.NewRouter()
	Mount(router, service)

	operator, err := users.Create(context.Background(), identity.NewUser{
		Email: "operator@example.com", Password: "operator password", Admin: true,
	})
	if err != nil {
		t.Fatalf("creating the operator: %v", err)
	}
	return fixture{router, operator, verifier, register, users, invitations}
}

func (f fixture) call(t *testing.T, method, path string, body any, auth bool) (*http.Response, map[string]any) {
	t.Helper()
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding the body: %v", err)
		}
		payload = bytes.NewReader(raw)
	}
	request := httptest.NewRequest(method, path, payload)
	if auth {
		request.SetBasicAuth("operator@example.com", "operator password")
	}
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	response := recorder.Result()
	var decoded map[string]any
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		decoded = nil
	}
	return response, decoded
}

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
