package directory

import (
	"context"
	"encoding/json"
	"testing"

	wire "github.com/FacileStudio/porte/directory"
	"gorm.io/gorm"
)

// peopleByID indexes a snapshot's people to their raw entitlements JSON.
func peopleByID(snapshot wire.Snapshot) map[string]string {
	out := map[string]string{}
	for _, person := range snapshot.People {
		raw, _ := json.Marshal(person.Entitlements)
		out[person.Subject] = string(raw)
	}
	return out
}

// setAdmin flips users.admin and users.disabled_at for one account.
func setAdmin(t *testing.T, db *gorm.DB, id string, admin, disabled bool) {
	t.Helper()
	statement := "UPDATE users SET admin = ?, disabled_at = CASE WHEN ? THEN now() END WHERE id = ?"
	if err := db.Exec(statement, admin, disabled, id).Error; err != nil {
		t.Fatalf("setting admin on %s: %v", id, err)
	}
}

// pushPlans applies one push whose free scope is bob and whose paid values
// sit on a space, the way Ardoise pushes every feature to every scope.
func pushPlans(t *testing.T, svc *Service, spaceID, bob string) {
	t.Helper()
	free := values("ai.access", "false", "ai.monthly_budget_usd", "0", "nuage.storage_gb", "5")
	paid := values("ai.access", "true", "ai.monthly_budget_usd", "20", "nuage.storage_gb", "100")
	_, err := svc.SetEntitlements(context.Background(), "ardoise", 3,
		[]SpaceEntitlements{{ID: spaceID, Entitlements: paid}},
		[]PersonEntitlements{{Subject: bob, Entitlements: free}})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
}

// TestAnOperatorHoldsEveryFeature is the rule: an enabled users.admin gets
// every bool true and every limit -1 in the snapshot, while a normal
// person's values and every space's stay byte for byte what Ardoise pushed.
func TestAnOperatorHoldsEveryFeature(t *testing.T) {
	svc, db, sub := fixture(t, "op@facile.studio", "bob@facile.studio")
	ctx := context.Background()
	op, bob := sub["op@facile.studio"], sub["bob@facile.studio"]
	space, _ := svc.Create(ctx, bob, "Team")
	pushPlans(t, svc, space.ID, bob)
	normal, _ := svc.Snapshot(ctx)
	if _, listed := peopleByID(normal)[op]; listed {
		t.Fatalf("a non-operator with no values is listed: %+v", normal.People)
	}

	setAdmin(t, db, op, true, false)
	granted, _ := svc.Snapshot(ctx)
	const full = `{"ai.access":true,"ai.monthly_budget_usd":-1,"nuage.storage_gb":-1}`
	if got := peopleByID(granted)[op]; got != full {
		t.Errorf("operator entitlements = %s, want %s", got, full)
	}
	if got, want := peopleByID(granted)[bob], peopleByID(normal)[bob]; got != want {
		t.Errorf("a normal person changed: %s, want %s", got, want)
	}
	if spaceValues(granted)[space.ID] != spaceValues(normal)[space.ID] {
		t.Error("a space's entitlements changed because an operator exists")
	}
}

// TestAnOperatorLosesThemWithAdminOrWhenDisabled pins revocation: the next
// snapshot after an operator loses admin or is disabled lists nothing extra.
func TestAnOperatorLosesThemWithAdminOrWhenDisabled(t *testing.T) {
	svc, db, sub := fixture(t, "op@facile.studio", "bob@facile.studio")
	ctx := context.Background()
	op, bob := sub["op@facile.studio"], sub["bob@facile.studio"]
	space, _ := svc.Create(ctx, bob, "Team")
	pushPlans(t, svc, space.ID, bob)
	normal, _ := svc.Snapshot(ctx)

	for _, state := range []struct {
		name            string
		admin, disabled bool
	}{{"disabled", true, true}, {"demoted", false, false}} {
		setAdmin(t, db, op, state.admin, state.disabled)
		snapshot, _ := svc.Snapshot(ctx)
		if got, want := peopleByID(snapshot), peopleByID(normal); len(got) != len(want) || got[bob] != want[bob] {
			t.Errorf("%s operator: people = %v, want %v", state.name, got, want)
		}
	}
}

// TestAnOperatorOverridesTheirOwnPlan covers an operator Ardoise already
// pushed free values for: the whole catalog replaces them.
func TestAnOperatorOverridesTheirOwnPlan(t *testing.T) {
	svc, db, sub := fixture(t, "op@facile.studio")
	op := sub["op@facile.studio"]
	setAdmin(t, db, op, true, false)
	_, err := svc.SetEntitlements(context.Background(), "ardoise", 1, nil,
		[]PersonEntitlements{{Subject: op, Entitlements: values("ai.access", "false", "nuage.storage_gb", "5")}})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	snapshot, _ := svc.Snapshot(context.Background())
	if got := peopleByID(snapshot)[op]; got != `{"ai.access":true,"nuage.storage_gb":-1}` {
		t.Errorf("operator entitlements = %s", got)
	}
	if len(snapshot.People) != 1 {
		t.Errorf("people = %+v, want only the operator", snapshot.People)
	}
}

// TestNoPushedEntitlementsGrantsNothing guards the fail-open history: with
// no catalog an operator is not listed, and neither is anyone, so a restored
// or empty Registre never turns paid features on.
func TestNoPushedEntitlementsGrantsNothing(t *testing.T) {
	svc, db, sub := fixture(t, "op@facile.studio")
	setAdmin(t, db, sub["op@facile.studio"], true, false)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.People) != 0 || snapshot.EntitlementsRevision != 0 {
		t.Errorf("people = %+v revision = %d, want none and 0", snapshot.People, snapshot.EntitlementsRevision)
	}
}

// TestAConflictingKindIsWithheld pins that a key stored as a bool in one row
// and a number in another is left out of the catalog.
func TestAConflictingKindIsWithheld(t *testing.T) {
	svc, db, sub := fixture(t, "op@facile.studio", "bob@facile.studio", "eve@facile.studio")
	setAdmin(t, db, sub["op@facile.studio"], true, false)
	_, err := svc.SetEntitlements(context.Background(), "ardoise", 1, nil, []PersonEntitlements{
		{Subject: sub["bob@facile.studio"], Entitlements: values("ai.access", "false", "x.y", "3")},
		{Subject: sub["eve@facile.studio"], Entitlements: values("x.y", "true")},
	})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	snapshot, _ := svc.Snapshot(context.Background())
	if got := peopleByID(snapshot)[sub["op@facile.studio"]]; got != `{"ai.access":true}` {
		t.Errorf("operator entitlements = %s", got)
	}
}
