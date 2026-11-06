package gdpr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestTheOnlyOwnerOfASharedSpaceMustHandItOver refuses the request, and at
// erasure promotes the most senior other member.
func TestTheOnlyOwnerOfASharedSpaceMustHandItOver(t *testing.T) {
	svc, db, _ := open(t)
	ctx := context.Background()
	user := seed(t, db)
	db.Create(&schemas.User{ID: "mate", Email: "mate@example.com"})
	db.Create(&schemas.User{ID: "boss", Email: "boss@example.com"})
	db.Create(&schemas.Space{ID: "sp", Name: "Team"})
	db.Create(&schemas.SpaceMember{SpaceID: "sp", UserID: user.ID, Role: "owner", CreatedAt: time.Now().Add(-3 * time.Hour)})
	db.Create(&schemas.SpaceMember{SpaceID: "sp", UserID: "mate", Role: "member", CreatedAt: time.Now().Add(-2 * time.Hour)})
	db.Create(&schemas.SpaceMember{SpaceID: "sp", UserID: "boss", Role: "admin", CreatedAt: time.Now().Add(-time.Hour)})

	var sole *SoleOwnerError
	if _, err := svc.Request(ctx, user); !errors.As(err, &sole) || sole.Spaces[0] != "Team" {
		t.Fatalf("Request from the only owner = %v, want SoleOwnerError naming Team", err)
	}
	if err := svc.Erase(ctx, user.ID); err != nil {
		t.Fatalf("Erase: %v", err)
	}
	var boss schemas.SpaceMember
	db.Where("space_id = ? AND user_id = ?", "sp", "boss").Take(&boss)
	if boss.Role != "owner" {
		t.Fatalf("after erasure the admin is %q, want owner", boss.Role)
	}
}

// TestNoticesArePersistedRetriedAndListable covers M4: a push that fails is
// kept and retried with backoff, and every app can list the erasure.
func TestNoticesArePersistedRetriedAndListable(t *testing.T) {
	svc, db, effects := open(t)
	ctx := context.Background()
	user := seed(t, db)
	db.Exec(`INSERT INTO clients (client_id, subject, kind, directory_sync_uri, deletion_uri) VALUES
		('nuage', 'nuage-sub', 'application', 'https://nuage.example/api/registre/sync', 'https://nuage.example/api/registre/deleted'),
		('plume', 'plume-sub', 'application', 'https://plume.example/api/registre/sync', ''),
		('casier', 'casier-sub', 'application', '', '')`)
	db.Create(&schemas.User{ID: "other", Email: "other@example.com"})
	db.Create(&schemas.Space{ID: "sp2", Name: "Other"})
	if err := db.Create(&schemas.SpaceInvitation{ID: "inv", SpaceID: "sp2", Email: user.Email, Role: "member",
		InvitedBy: "other", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	effects.failing = true
	if err := svc.Erase(ctx, user.ID); err != nil {
		t.Fatalf("Erase: %v", err)
	}
	var pending []struct {
		Attempts      int
		NextAttemptAt time.Time
		DeliveredAt   *time.Time
	}
	db.Raw("SELECT attempts, next_attempt_at, delivered_at FROM deletion_notices WHERE client_id = 'nuage'").Scan(&pending)
	if len(pending) != 1 || pending[0].Attempts != 1 || pending[0].DeliveredAt != nil || !pending[0].NextAttemptAt.After(time.Now()) {
		t.Fatalf("after a failed push: %+v, want one row retried later", pending)
	}
	effects.failing = false
	db.Exec("UPDATE deletion_notices SET next_attempt_at = now() - interval '1 second'")
	svc.DeliverNotices(ctx)
	if len(effects.deleted) != 1 || effects.deleted[0] != user.ID+"@nuage" {
		t.Fatalf("delivered %v, want one push to nuage's deletion URI and none to a sync URI", effects.deleted)
	}
	var plume []struct{ DeliveredAt *time.Time }
	db.Raw("SELECT delivered_at FROM deletion_notices WHERE client_id = 'plume'").Scan(&plume)
	if len(plume) != 1 || plume[0].DeliveredAt != nil {
		t.Fatalf("plume, which has only a sync URI, shows %+v; a sync nudge must never count as delivery", plume)
	}
	for _, app := range []string{"nuage", "plume", "casier"} {
		listed, err := DeletedFor(ctx, db, app, time.Now().Add(-time.Hour))
		if err != nil || len(listed) != 1 || listed[0].Subject != user.ID {
			t.Fatalf("%s lists %v, %v; want the erased subject", app, listed, err)
		}
	}
	var invitations int64
	db.Model(&schemas.SpaceInvitation{}).Where("lower(email) = ?", user.Email).Count(&invitations)
	if invitations != 0 {
		t.Fatal("an invitation addressed to the erased person survived")
	}
}

// TestErasureScrubsTheAddressFromTheAuditTrail keeps the audit rows and
// removes the person's address from them.
func TestErasureScrubsTheAddressFromTheAuditTrail(t *testing.T) {
	svc, db, _ := open(t)
	user := seed(t, db)
	db.Exec(`INSERT INTO admin_events (actor_id, action, target_type, target_id, detail) VALUES
		('op', 'user.create', 'user', ?, jsonb_build_object('email', ?::text, 'admin', false)),
		('op', 'invite.create', 'invite', ?, '{}'::jsonb)`, user.ID, user.Email, user.Email)
	if err := svc.Erase(context.Background(), user.ID); err != nil {
		t.Fatalf("Erase: %v", err)
	}
	var leaks int64
	db.Raw(`SELECT count(*) FROM admin_events WHERE detail::text ILIKE ? OR target_id ILIKE ?`,
		"%"+user.Email+"%", user.Email).Scan(&leaks)
	if leaks != 0 {
		t.Fatalf("%d audit rows still carry the erased address", leaks)
	}
	var kept int64
	db.Raw(`SELECT count(*) FROM admin_events WHERE action IN ('user.create', 'invite.create')`).Scan(&kept)
	if kept != 2 {
		t.Fatalf("%d audit rows kept, want the 2 rows without the address", kept)
	}
}
