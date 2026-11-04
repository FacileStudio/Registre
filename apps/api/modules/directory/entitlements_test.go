package directory

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	spaces "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// writerRouters registers ardoise and agenda as apps and answers one router
// with ardoise as the writer and one with the push disabled.
func writerRouters(t *testing.T) (chi.Router, chi.Router, *gorm.DB) {
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
	register := clients.New(db, credentials.New(2))
	for _, id := range []string{"ardoise", "agenda"} {
		hash, _ := register.HashSecret(context.Background(), "s3cret-"+id)
		if _, err := register.Upsert(context.Background(), schemas.Client{
			ClientID: id, Subject: id + "-sub", Name: id, Kind: schemas.KindApplication, SecretHash: hash,
		}); err != nil {
			t.Fatalf("registering %s: %v", id, err)
		}
	}
	logs := slog.New(slog.DiscardHandler)
	team := spaces.New(db, nil, nil, logs)
	on, off := chi.NewRouter(), chi.NewRouter()
	Mount(on, New(register, team, logs).WithWriter("ardoise"))
	Mount(off, New(register, team, logs).WithWriter(""))
	return on, off, db
}

// put sends one push with the given Basic pair, empty meaning none.
func put(router chi.Router, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/directory/entitlements", strings.NewReader(body))
	if id != "" {
		req.SetBasicAuth(id, "s3cret-"+id)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestOnlyTheWriterPushesEntitlements is the trust boundary on plans: only
// Ardoise's own client may write, a disabled route answers nobody, and a
// body that is oversized, carries unknown fields or malformed values, or is
// older than what is stored, changes nothing.
func TestOnlyTheWriterPushesEntitlements(t *testing.T) {
	on, off, db := writerRouters(t)
	good := `{"revision":3,"spaces":[],"people":[]}`
	cases := []struct {
		name, id, body string
		router         chi.Router
		want           int
	}{
		{"no credentials", "", good, on, http.StatusUnauthorized},
		{"another app", "agenda", good, on, http.StatusForbidden},
		{"route disabled", "ardoise", good, off, http.StatusNotFound},
		{"the writer", "ardoise", good, on, http.StatusNoContent},
		{"stale revision", "ardoise", `{"revision":2}`, on, http.StatusConflict},
		{"unknown field", "ardoise", `{"revision":4,"plan":"pro"}`, on, http.StatusBadRequest},
		{"malformed value", "ardoise", `{"revision":4,"spaces":[{"id":"x","entitlements":{"a.b":"yes"}}]}`, on, http.StatusBadRequest},
		{"bad key", "ardoise", `{"revision":4,"spaces":[{"id":"x","entitlements":{"A":1}}]}`, on, http.StatusBadRequest},
		{"trailing data", "ardoise", good + `{}`, on, http.StatusBadRequest},
		{"oversized", "ardoise", `{"revision":4,"spaces":[` + strings.Repeat(`{"id":"x"},`, 1<<17) + `{"id":"x"}]}`, on, http.StatusBadRequest},
	}
	for _, tc := range cases {
		got := put(tc.router, tc.id, tc.body)
		if got.Code != tc.want {
			t.Errorf("%s: answered %d, want %d (%s)", tc.name, got.Code, tc.want, got.Body.String())
		}
		if tc.want == http.StatusConflict && !strings.Contains(got.Body.String(), `"current":3`) {
			t.Errorf("%s: body %s does not carry the current revision", tc.name, got.Body.String())
		}
	}
	var events int64
	db.Model(&schemas.AdminEvent{}).Where("action = ? AND actor_id = ?", "entitlements.set", "ardoise").Count(&events)
	if events != 1 {
		t.Errorf("audit rows for applied pushes = %d, want 1", events)
	}
}

// TestTheWriterNeedsItsOwnSecret keeps the client id alone from being a
// credential: the writer's name with a wrong secret is still 401.
func TestTheWriterNeedsItsOwnSecret(t *testing.T) {
	on, _, _ := writerRouters(t)
	req := httptest.NewRequest(http.MethodPut, "/api/directory/entitlements", strings.NewReader(`{"revision":1}`))
	req.SetBasicAuth("ardoise", "wrong")
	recorder := httptest.NewRecorder()
	on.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("a wrong secret answered %d, want 401", recorder.Code)
	}
}
