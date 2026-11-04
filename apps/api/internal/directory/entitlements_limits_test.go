package directory

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestARevisionCannotLeapAhead keeps one bad push from making every honest
// push after it stale.
func TestARevisionCannotLeapAhead(t *testing.T) {
	svc, _, _ := fixture(t)
	ctx := context.Background()
	if _, err := svc.SetEntitlements(ctx, "ardoise", 1, nil, nil); err != nil {
		t.Fatalf("first push: %v", err)
	}
	if _, err := svc.SetEntitlements(ctx, "ardoise", 2+MaxRevisionJump, nil, nil); !errors.Is(err, ErrInvalidEntitlements) {
		t.Errorf("a leap past the bound answered %v, want ErrInvalidEntitlements", err)
	}
	if _, err := svc.SetEntitlements(ctx, "ardoise", 1+MaxRevisionJump, nil, nil); err != nil {
		t.Errorf("a leap exactly at the bound was refused: %v", err)
	}
}

// TestOnlyAppliedPushesAreAudited pins the audit row to the transaction:
// one row per applied push, none for a refused one.
func TestOnlyAppliedPushesAreAudited(t *testing.T) {
	svc, db, _ := fixture(t)
	ctx := context.Background()
	_, _ = svc.SetEntitlements(ctx, "ardoise", 5, nil, nil)
	_, _ = svc.SetEntitlements(ctx, "ardoise", 4, nil, nil)
	_, _ = svc.SetEntitlements(ctx, "ardoise", 6, []SpaceEntitlements{{ID: "x", Entitlements: values("A", "1")}}, nil)
	var rows []schemas.AdminEvent
	db.Where("action = ?", "entitlements.set").Find(&rows)
	if len(rows) != 1 || rows[0].ActorID != "ardoise" || rows[0].TargetID != "5" {
		t.Errorf("audit rows = %+v, want exactly the applied revision 5 by ardoise", rows)
	}
}

// TestLargePushesAreCappedAndBatched covers both ends of the size rules: a
// push over MaxScopes is refused, and one spanning several write batches
// stores every known scope once.
func TestLargePushesAreCappedAndBatched(t *testing.T) {
	svc, _, sub := fixture(t, "yann@facile.studio")
	ctx := context.Background()
	if _, err := svc.SetEntitlements(ctx, "ardoise", 1, make([]SpaceEntitlements, MaxScopes+1), nil); !errors.Is(err, ErrInvalidEntitlements) {
		t.Errorf("a push over MaxScopes answered %v", err)
	}
	var push []SpaceEntitlements
	for i := 0; i < 3*writeBatch; i++ {
		push = append(push, SpaceEntitlements{ID: fmt.Sprintf("spc_unknown_%d", i), Entitlements: values("a.b", "1")})
	}
	for _, name := range []string{"Alpha", "Beta"} {
		space, _ := svc.Create(ctx, sub["yann@facile.studio"], name)
		push = append(push, SpaceEntitlements{ID: space.ID, Entitlements: values("a.b", "1")},
			SpaceEntitlements{ID: space.ID, Entitlements: values("a.b", "2")})
	}
	applied, err := svc.SetEntitlements(ctx, "ardoise", 1, push, nil)
	if err != nil || applied.Spaces != 2 || applied.SkippedSpaces != 3*writeBatch {
		t.Fatalf("applied = %+v, err = %v; want 2 stored and %d skipped", applied, err, 3*writeBatch)
	}
	snapshot, _ := svc.Snapshot(ctx)
	for id, got := range spaceValues(snapshot) {
		if got != `{"a.b":2}` {
			t.Errorf("space %s = %s, want the last value pushed for it", id, got)
		}
	}
}

// TestASnapshotNeverStraddlesTwoPushes runs pushes and snapshots together.
// Every push sets the value to its own revision, so a snapshot whose value
// and revision disagree read two different commits.
func TestASnapshotNeverStraddlesTwoPushes(t *testing.T) {
	svc, _, sub := fixture(t, "yann@facile.studio")
	ctx := context.Background()
	space, _ := svc.Create(ctx, sub["yann@facile.studio"], "Alpha")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for revision := int64(1); revision <= 40; revision++ {
			value := strconv.FormatInt(revision, 10)
			_, _ = svc.SetEntitlements(ctx, "ardoise", revision,
				[]SpaceEntitlements{{ID: space.ID, Entitlements: values("a.b", value)}}, nil)
		}
	}()
	for i := 0; i < 40; i++ {
		snapshot, err := svc.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		got, want := spaceValues(snapshot)[space.ID], fmt.Sprintf(`{"a.b":%d}`, snapshot.EntitlementsRevision)
		if snapshot.EntitlementsRevision > 0 && got != want {
			t.Fatalf("snapshot at revision %d carries %s", snapshot.EntitlementsRevision, got)
		}
	}
	wg.Wait()
}
