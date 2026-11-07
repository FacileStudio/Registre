package directory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	spaces "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

type deletionPage struct {
	Deleted    []gdpr.Deleted `json:"deleted"`
	NextCursor string         `json:"next_cursor"`
}

func TestDeletionsPageThroughNoticesSharingOneTimestamp(t *testing.T) {
	dbURL, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(dbURL)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	ctx := context.Background()
	register := clients.New(db, credentials.New(2))
	hash, err := register.HashSecret(ctx, "s3cret")
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	for _, id := range []string{"agenda", "capsule"} {
		if _, err := register.Upsert(ctx, schemas.Client{
			ClientID: id, Subject: id + "-sub", Name: id, Kind: schemas.KindApplication, SecretHash: hash,
		}); err != nil {
			t.Fatalf("registering %s: %v", id, err)
		}
	}
	stamp := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	err = db.Exec(`INSERT INTO deletion_notices (client_id, subject, created_at)
		SELECT 'agenda', 'u' || lpad(n::text, 5, '0'), ? FROM generate_series(1, 1500) n`, stamp).Error
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	db.Exec(`INSERT INTO deletion_notices (client_id, subject, created_at) VALUES ('capsule', 'other', ?)`, stamp)

	logs := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := chi.NewRouter()
	Mount(router, New(register, spaces.New(db, nil, nil, logs), logs).WithDeletions(
		func(ctx context.Context, clientID string, since time.Time, after *gdpr.Cursor) ([]gdpr.Deleted, string, error) {
			return gdpr.DeletedFor(ctx, db, clientID, since, after)
		}))

	get := func(query string) (int, deletionPage) {
		req := httptest.NewRequest(http.MethodGet, "/api/directory/deletions"+query, nil)
		req.SetBasicAuth("agenda", "s3cret")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		var page deletionPage
		_ = json.Unmarshal(recorder.Body.Bytes(), &page)
		return recorder.Code, page
	}

	since := "?since=" + url.QueryEscape(stamp.Add(-time.Minute).Format(time.RFC3339))
	code, first := get(since)
	if code != http.StatusOK || len(first.Deleted) != 1000 || first.NextCursor == "" {
		t.Fatalf("first page: %d, %d rows, cursor %q; want 200, 1000 rows and a cursor", code, len(first.Deleted), first.NextCursor)
	}
	code, second := get(since + "&cursor=" + url.QueryEscape(first.NextCursor))
	if code != http.StatusOK || len(second.Deleted) != 500 || second.NextCursor != "" {
		t.Fatalf("second page: %d, %d rows, cursor %q; want 200, 500 rows and no cursor", code, len(second.Deleted), second.NextCursor)
	}
	seen := map[string]bool{}
	for _, d := range append(first.Deleted, second.Deleted...) {
		if seen[d.Subject] {
			t.Fatalf("%s listed twice", d.Subject)
		}
		seen[d.Subject] = true
	}
	if len(seen) != 1500 || seen["other"] {
		t.Fatalf("listed %d distinct subjects (other app's: %v), want exactly agenda's 1500", len(seen), seen["other"])
	}

	code, exact := get(fmt.Sprintf("%s&cursor=%s", since, url.QueryEscape(gdpr.Cursor{At: stamp, ID: 1 << 40}.Encode())))
	if code != http.StatusOK || len(exact.Deleted) != 0 || exact.NextCursor != "" {
		t.Fatalf("cursor past the end: %d, %d rows, cursor %q; want an empty last page", code, len(exact.Deleted), exact.NextCursor)
	}
	for _, bad := range []string{"!!!", "bm90LWEtY3Vyc29y", "LTE"} {
		if code, _ := get("?cursor=" + bad); code != http.StatusBadRequest {
			t.Errorf("cursor %q answered %d, want 400", bad, code)
		}
	}
	code, legacy := get(since)
	if code != http.StatusOK || len(legacy.Deleted) != 1000 {
		t.Fatalf("a caller with only since got %d, %d rows; want the first 1000 as before", code, len(legacy.Deleted))
	}
}
