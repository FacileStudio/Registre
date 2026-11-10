package directory

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	spaces "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

// TestAppsReadAPersonsPendingInvitations lets a suite app, and only a suite
// app with its own secret, read the invitations waiting for one subject.
func TestAppsReadAPersonsPendingInvitations(t *testing.T) {
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
	ctx := context.Background()
	register := clients.New(db, credentials.New(2))
	declare := func(row schemas.Client, secret string) {
		hash, err := register.HashSecret(ctx, secret)
		if err != nil {
			t.Fatal(err)
		}
		row.SecretHash, row.Subject, row.Name = hash, row.ClientID+"-sub", row.ClientID
		if _, err := register.Upsert(ctx, row); err != nil {
			t.Fatalf("registering %s: %v", row.ClientID, err)
		}
	}
	declare(schemas.Client{ClientID: "nuage", Kind: schemas.KindApplication}, "s3cret-nuage")
	declare(schemas.Client{ClientID: "ci", Kind: schemas.KindServiceAccount}, "s3cret-ci")
	declare(schemas.Client{ClientID: "dcr-x", Kind: schemas.KindApplication, Registration: schemas.RegistrationDynamic}, "s3cret-dcr")
	declare(schemas.Client{ClientID: "cli", Kind: schemas.KindApplication, Public: true}, "s3cret-cli")

	users := identity.New(db, credentials.New(2))
	owner, _ := users.Create(ctx, identity.NewUser{Email: "noah@facile.studio", Password: "correct horse battery"})
	yann, _ := users.Create(ctx, identity.NewUser{Email: "yann@facile.studio", Password: "correct horse battery"})
	logs := slog.New(slog.NewTextHandler(io.Discard, nil))
	teams := spaces.New(db, nil, nil, logs).WithInvitations(spaces.Invitations{Issuer: "https://sso.example.com/"})
	space, _ := teams.Create(ctx, owner.ID, "EpiKodi")
	if err := teams.Invite(ctx, owner.ID, space.ID, "yann@facile.studio", "member"); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	Mount(router, New(register, teams, logs))

	ask := func(id, secret, sub string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodGet, "/api/directory/invitations?sub="+sub, nil)
		if id != "" {
			req.SetBasicAuth(id, secret)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		var out map[string]any
		_ = json.Unmarshal(recorder.Body.Bytes(), &out)
		return recorder.Code, out
	}
	refused := map[string][2]string{
		"no credentials":     {"", ""},
		"a wrong secret":     {"nuage", "nope"},
		"a service account":  {"ci", "s3cret-ci"},
		"a third-party app":  {"dcr-x", "s3cret-dcr"},
		"a public client":    {"cli", "s3cret-cli"},
		"another app's pair": {"nuage", "s3cret-ci"},
	}
	for name, pair := range refused {
		if status, _ := ask(pair[0], pair[1], yann.ID); status != http.StatusUnauthorized {
			t.Errorf("%s: answered %d, want 401", name, status)
		}
	}

	status, body := ask("nuage", "s3cret-nuage", yann.ID)
	list, _ := body["invitations"].([]any)
	if status != http.StatusOK || body["count"] != float64(1) || len(list) != 1 {
		t.Fatalf("yann's invitations = %d %v", status, body)
	}
	got := list[0].(map[string]any)
	if got["space_name"] != "EpiKodi" || got["role"] != "member" || got["invited_by"] != "noah@facile.studio" ||
		got["url"] != "https://sso.example.com/invitations/"+got["id"].(string) || got["expires_at"] == nil {
		t.Fatalf("the invitation = %v", got)
	}
	if status, body := ask("nuage", "s3cret-nuage", "no-such-subject"); status != http.StatusOK || body["count"] != float64(0) {
		t.Fatalf("an unknown subject = %d %v, want 200 and count 0", status, body)
	}
	if status, _ := ask("nuage", "s3cret-nuage", ""); status != http.StatusBadRequest {
		t.Fatalf("no subject = %d, want 400", status)
	}
	db.Exec("UPDATE users SET disabled_at = now() WHERE id = ?", yann.ID)
	if _, body := ask("nuage", "s3cret-nuage", yann.ID); body["count"] != float64(0) {
		t.Fatalf("a disabled person still has invitations: %v", body)
	}
}
