package accounts

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// fired is one back-channel logout the fake delivery received.
type fired struct {
	userID    string
	clientIDs []string
}

// delivery is a Notifier that records what it was asked to fire.
type delivery struct{ calls chan fired }

// Fire records one logout.
func (d delivery) Fire(_ context.Context, userID string, clientIDs []string) error {
	d.calls <- fired{userID, clientIDs}
	return nil
}

// next waits for the next logout, which announce sends from a goroutine.
func (d delivery) next(t *testing.T) fired {
	t.Helper()
	select {
	case call := <-d.calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("no back-channel logout was fired")
		return fired{}
	}
}

// none asserts nothing was fired. announce starts its goroutine before the
// call returns, so a short wait is enough to see one that was started.
func (d delivery) none(t *testing.T) {
	t.Helper()
	select {
	case call := <-d.calls:
		t.Fatalf("unexpected back-channel logout for %s to %v", call.userID, call.clientIDs)
	case <-time.After(200 * time.Millisecond):
	}
}

// signOut is the service under test, its database, the fake delivery and
// alice's cookie.
type signOut struct {
	svc   *Service
	db    *gorm.DB
	calls delivery
	token string
}

// openDB opens and empties the test database, or skips without one.
func openDB(t *testing.T) *gorm.DB {
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
	return db
}

// signOutFixture seeds two users, alice holding a session that reached nuage
// and opus, and refresh grants to nuage and to a CLI the session never reached.
func signOutFixture(t *testing.T) signOut {
	t.Helper()
	db := openDB(t)
	now := time.Now().UTC()
	rows := []any{
		&schemas.User{ID: "alice", Email: "alice@x.test"},
		&schemas.User{ID: "bob", Email: "bob@x.test"},
		&schemas.Session{ID: "s-alice", UserID: "alice", CreatedAt: now,
			ClientIDs: schemas.StringList{"nuage", "opus"}},
		&schemas.RefreshToken{ID: "rt-nuage", UserID: "alice", ClientID: "nuage", ExpiresAt: now.Add(time.Hour)},
		&schemas.RefreshToken{ID: "rt-cli", UserID: "alice", ClientID: "facile-cli", ExpiresAt: now.Add(time.Hour)},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seeding %T: %v", row, err)
		}
	}
	calls := delivery{calls: make(chan fired, 8)}
	svc := New(db).WithNotifier(calls, nil)
	token, err := svc.Issue(context.Background(), "s-alice")
	if err != nil {
		t.Fatalf("issuing the cookie: %v", err)
	}
	return signOut{svc: svc, db: db, calls: calls, token: token}
}

// revoked reports whether a refresh grant has been revoked.
func revoked(t *testing.T, db *gorm.DB, id string) bool {
	t.Helper()
	var row schemas.RefreshToken
	if err := db.Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	return row.RevokedAt != nil
}

func TestSignOutTellsTheSessionsAppsAndRevokesOnlyTheirGrants(t *testing.T) {
	f := signOutFixture(t)
	svc, db, calls, token := f.svc, f.db, f.calls, f.token
	ctx := context.Background()

	if err := svc.SignOut(ctx, token); err != nil {
		t.Fatalf("signing out: %v", err)
	}
	call := calls.next(t)
	if call.userID != "alice" || !slices.Equal(call.clientIDs, []string{"nuage", "opus"}) {
		t.Fatalf("fired %s to %v, want alice to [nuage opus]", call.userID, call.clientIDs)
	}
	if !revoked(t, db, "rt-nuage") {
		t.Fatal("the grant of an app the session reached survived the logout")
	}
	if revoked(t, db, "rt-cli") {
		t.Fatal("a browser logout revoked the grant of an app the session never reached")
	}
	if err := svc.SignOut(ctx, token); err != nil {
		t.Fatalf("signing out a dead cookie: %v", err)
	}
	calls.none(t)
}

func TestReplaceSignsOutOnlyAnotherPersonsSession(t *testing.T) {
	f := signOutFixture(t)
	svc, calls, token := f.svc, f.calls, f.token
	ctx := context.Background()

	if err := svc.Replace(ctx, token, "alice"); err != nil {
		t.Fatalf("replacing with the same person: %v", err)
	}
	calls.none(t)
	if _, _, err := svc.Resolve(ctx, token); err != nil {
		t.Fatalf("signing in again as the same person ended the session: %v", err)
	}

	if err := svc.Replace(ctx, token, "bob"); err != nil {
		t.Fatalf("replacing with another person: %v", err)
	}
	if call := calls.next(t); call.userID != "alice" {
		t.Fatalf("fired for %s, want alice", call.userID)
	}
	calls.none(t)
	if _, _, err := svc.Resolve(ctx, token); err == nil {
		t.Fatal("alice's session outlived bob signing in on the same browser")
	}
}
