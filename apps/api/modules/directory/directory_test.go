package directory

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	spaces "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

// TestOnlyAnAppWithItsOwnSecretReadsTheSnapshot keeps the directory a
// machine API: every space and member in the suite sits behind it, so a
// missing or wrong secret, or a service account's, must answer 401.
func TestOnlyAnAppWithItsOwnSecretReadsTheSnapshot(t *testing.T) {
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
	for id, kind := range map[string]string{"agenda": schemas.KindApplication, "ci": schemas.KindServiceAccount} {
		hash, err := register.HashSecret(context.Background(), "s3cret-"+id)
		if err != nil {
			t.Fatalf("hashing: %v", err)
		}
		if _, err := register.Upsert(context.Background(), schemas.Client{
			ClientID: id, Subject: id + "-sub", Name: id, Kind: kind, SecretHash: hash,
		}); err != nil {
			t.Fatalf("registering %s: %v", id, err)
		}
	}
	logs := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := chi.NewRouter()
	Mount(router, New(register, spaces.New(db, nil, nil, logs), logs))

	cases := map[string]struct {
		id, secret string
		want       int
	}{
		"no credentials":     {"", "", http.StatusUnauthorized},
		"wrong secret":       {"agenda", "nope", http.StatusUnauthorized},
		"a service account":  {"ci", "s3cret-ci", http.StatusUnauthorized},
		"the app's own pair": {"agenda", "s3cret-agenda", http.StatusOK},
	}
	for name, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/directory/snapshot", nil)
		if tc.id != "" {
			req.SetBasicAuth(tc.id, tc.secret)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != tc.want {
			t.Errorf("%s: answered %d, want %d", name, recorder.Code, tc.want)
		}
	}
}
