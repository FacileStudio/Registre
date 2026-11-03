package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestCompleteRefusesAnAppTheUserMayNotUse(t *testing.T) {
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
	now := time.Now().UTC()
	rows := []any{
		&schemas.Client{ClientID: "ardoise", Subject: "sub-ardoise", Kind: schemas.KindApplication,
			AccessPolicy: schemas.AccessAssigned},
		&schemas.User{ID: "alice", Email: "alice@x.test"},
		&schemas.User{ID: "bob", Email: "bob@x.test"},
		&schemas.AppGrant{ClientID: "ardoise", UserID: "alice", GrantedAt: now},
		&schemas.AuthRequest{ID: "req-bob", ClientID: "ardoise", ExpiresAt: now.Add(time.Minute)},
		&schemas.AuthRequest{ID: "req-alice", ClientID: "ardoise", ExpiresAt: now.Add(time.Minute)},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seeding %T: %v", row, err)
		}
	}
	users := New(db, credentials.New(1))
	ctx := context.Background()
	if err := users.Complete(ctx, "req-bob", "bob", "", now); !errors.Is(err, access.ErrNoAccess) {
		t.Fatalf("bob signed in to a restricted app he holds no grant to: %v", err)
	}
	if err := users.Complete(ctx, "req-alice", "alice", "", now); err != nil {
		t.Fatalf("alice holds a grant and was refused: %v", err)
	}
}
