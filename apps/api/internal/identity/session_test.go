package identity

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// touchFixture opens and empties the test database, or skips without one.
func touchFixture(t *testing.T) *Service {
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
	if err := db.Create(&schemas.User{ID: "alice", Email: "alice@x.test"}).Error; err != nil {
		t.Fatalf("seeding: %v", err)
	}
	return New(db, credentials.New(1))
}

func TestTouchSessionKeepsEveryAppThatSignsInAtOnce(t *testing.T) {
	users := touchFixture(t)
	ctx := context.Background()
	sessionID, err := users.StartSession(ctx, "alice", "nuage", time.Now().UTC())
	if err != nil {
		t.Fatalf("starting the session: %v", err)
	}
	apps := []string{"opus", "sablier", "agenda", "plume", "nuage"}
	var wg sync.WaitGroup
	for _, app := range apps {
		wg.Go(func() {
			if err := users.TouchSession(ctx, sessionID, app); err != nil {
				t.Errorf("touching %s: %v", app, err)
			}
		})
	}
	wg.Wait()
	var session schemas.Session
	if err := users.db.Where("id = ?", sessionID).Take(&session).Error; err != nil {
		t.Fatalf("reading the session: %v", err)
	}
	got := slices.Sorted(slices.Values(session.ClientIDs))
	want := []string{"agenda", "nuage", "opus", "plume", "sablier"}
	if !slices.Equal(got, want) {
		t.Fatalf("client_ids = %v, want %v", got, want)
	}
}
