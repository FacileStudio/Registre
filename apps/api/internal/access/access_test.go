package access

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

func open(t *testing.T) *gorm.DB {
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

func seed(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []any{
		&schemas.Client{ClientID: "ardoise", Subject: "sub-ardoise", Name: "Ardoise", Kind: schemas.KindApplication,
			VisibleRoles: schemas.StringList{"admin"}},
		&schemas.Client{ClientID: "registre", Subject: "sub-registre", Name: "Registre", Kind: schemas.KindApplication},
		&schemas.Client{ClientID: "ci", Subject: "sub-ci", Name: "CI", Kind: schemas.KindServiceAccount},
		&schemas.User{ID: "alice", Email: "alice@x.test"},
		&schemas.User{ID: "bob", Email: "bob@x.test"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seeding %T: %v", row, err)
		}
	}
	for _, user := range []string{"alice", "bob"} {
		token := schemas.RefreshToken{ID: "rt-" + user, UserID: user, ClientID: "ardoise", AuthTime: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour)}
		if err := db.Create(&token).Error; err != nil {
			t.Fatalf("seeding a refresh token: %v", err)
		}
	}
}

func revoked(t *testing.T, db *gorm.DB, id string) bool {
	t.Helper()
	var count int64
	db.Table("refresh_tokens").Where("id = ? AND revoked_at IS NOT NULL", id).Count(&count)
	return count == 1
}

func TestAccessFollowsThePolicyAndTheGrants(t *testing.T) {
	db := open(t)
	seed(t, db)
	svc := New(db, "registre")
	ctx := context.Background()

	if err := svc.Require(ctx, "ardoise", "bob"); err != nil {
		t.Fatalf("an app open to everyone refused bob: %v", err)
	}
	if _, err := svc.Grant(ctx, "ardoise", "alice", []string{"admin", "admin"}, "op"); err != nil {
		t.Fatalf("granting alice: %v", err)
	}
	if _, err := svc.Grant(ctx, "ardoise", "alice", []string{"owner"}, "op"); !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("an undeclared role was accepted: %v", err)
	}
	if err := svc.SetPolicy(ctx, "ardoise", schemas.AccessAssigned); err != nil {
		t.Fatalf("restricting ardoise: %v", err)
	}
	if err := svc.Require(ctx, "ardoise", "bob"); !errors.Is(err, ErrNoAccess) {
		t.Fatalf("restricted ardoise admitted bob: %v", err)
	}
	if !revoked(t, db, "rt-bob") || revoked(t, db, "rt-alice") {
		t.Fatal("restricting should revoke only the ungranted user's refresh tokens")
	}
	if roles, err := svc.Roles(ctx, "ardoise", "alice"); err != nil || !slices.Equal(roles, []string{"admin"}) {
		t.Fatalf("alice's roles = %v (%v), want [admin]", roles, err)
	}
	if err := svc.Require(ctx, "ci", "bob"); err != nil {
		t.Fatalf("a service account has no human policy: %v", err)
	}
	if err := svc.SetPolicy(ctx, "registre", schemas.AccessAssigned); !errors.Is(err, ErrSelfLocked) {
		t.Fatalf("registre's own client was restricted: %v", err)
	}
	if err := svc.Revoke(ctx, "ardoise", "alice"); err != nil {
		t.Fatalf("revoking alice: %v", err)
	}
	if err := svc.Require(ctx, "ardoise", "alice"); !errors.Is(err, ErrNoAccess) || !revoked(t, db, "rt-alice") {
		t.Fatalf("a revoked grant still admits alice or kept her refresh token: %v", err)
	}
}
