package directory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
)

// values builds one scope from key=JSON pairs.
func values(pairs ...string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i]] = json.RawMessage(pairs[i+1])
	}
	return out
}

// spaceValues indexes a snapshot's space entitlements by space id.
func spaceValues(snapshot wire.Snapshot) map[string]string {
	out := map[string]string{}
	for _, space := range snapshot.Spaces {
		if len(space.Entitlements) > 0 {
			raw, _ := json.Marshal(space.Entitlements)
			out[space.ID] = string(raw)
		}
	}
	return out
}

// TestAPushReplacesEverythingAndSkipsStrangers is the full-replace contract:
// every known scope lands in the snapshot, unknown ones are counted and
// skipped, and the next push clears whatever it leaves out.
func TestAPushReplacesEverythingAndSkipsStrangers(t *testing.T) {
	svc, _, sub := fixture(t, "yann@facile.studio", "bob@facile.studio")
	ctx := context.Background()
	alpha, _ := svc.Create(ctx, sub["yann@facile.studio"], "Alpha")
	beta, _ := svc.Create(ctx, sub["yann@facile.studio"], "Beta")
	before, _ := svc.Snapshot(ctx)

	applied, err := svc.SetEntitlements(ctx, "ardoise", 5, []SpaceEntitlements{
		{ID: alpha.ID, Entitlements: values("nuage.storage_gb", "100")},
		{ID: beta.ID, Entitlements: values("glouton.access", "true")},
		{ID: "spc_nope", Entitlements: values("glouton.access", "true")},
	}, []PersonEntitlements{
		{Subject: sub["yann@facile.studio"], Entitlements: values("courrier.mailboxes", "-1")},
		{Email: "BOB@facile.studio", Entitlements: values("nuage.storage_gb", "5")},
		{Subject: "ghost", Entitlements: values("nuage.storage_gb", "5")},
	})
	if err != nil {
		t.Fatalf("SetEntitlements: %v", err)
	}
	if want := (Applied{Spaces: 2, People: 2, SkippedSpaces: 1, SkippedPeople: 1}); applied != want {
		t.Errorf("applied = %+v, want %+v", applied, want)
	}
	snapshot, _ := svc.Snapshot(ctx)
	if before.EntitlementsRevision != 0 || snapshot.EntitlementsRevision != 5 {
		t.Errorf("entitlements revision went %d -> %d, want 0 -> 5", before.EntitlementsRevision, snapshot.EntitlementsRevision)
	}
	if snapshot.Version <= before.Version {
		t.Errorf("version %d did not rise past %d", snapshot.Version, before.Version)
	}
	if got := spaceValues(snapshot); got[alpha.ID] != `{"nuage.storage_gb":100}` || got[beta.ID] != `{"glouton.access":true}` {
		t.Errorf("space entitlements = %v", got)
	}
	if len(snapshot.People) != 2 {
		t.Fatalf("people = %+v, want yann and bob", snapshot.People)
	}
}

// TestANarrowerPushClearsTheRest is the full-replace half: whatever the
// next push leaves out loses its values.
func TestANarrowerPushClearsTheRest(t *testing.T) {
	svc, _, sub := fixture(t, "yann@facile.studio")
	ctx := context.Background()
	alpha, _ := svc.Create(ctx, sub["yann@facile.studio"], "Alpha")
	beta, _ := svc.Create(ctx, sub["yann@facile.studio"], "Beta")
	if _, err := svc.SetEntitlements(ctx, "ardoise", 1, []SpaceEntitlements{
		{ID: alpha.ID, Entitlements: values("nuage.storage_gb", "100")},
		{ID: beta.ID, Entitlements: values("glouton.access", "true")},
	}, []PersonEntitlements{{Subject: sub["yann@facile.studio"], Entitlements: values("glouton.access", "true")}}); err != nil {
		t.Fatalf("first push: %v", err)
	}
	if _, err := svc.SetEntitlements(ctx, "ardoise", 2, []SpaceEntitlements{{ID: alpha.ID, Entitlements: values("nuage.storage_gb", "500")}}, nil); err != nil {
		t.Fatalf("second push: %v", err)
	}
	snapshot, _ := svc.Snapshot(ctx)
	if got := spaceValues(snapshot); len(got) != 1 || got[alpha.ID] != `{"nuage.storage_gb":500}` {
		t.Errorf("after a narrower push, space entitlements = %v, want only alpha", got)
	}
	if len(snapshot.People) != 0 {
		t.Errorf("after a push with no people, people = %+v", snapshot.People)
	}
}

// TestAStalePushChangesNothing keeps a delayed or replayed push from rolling
// the suite back: older answers the current revision, equal is a retry.
func TestAStalePushChangesNothing(t *testing.T) {
	svc, _, sub := fixture(t, "yann@facile.studio")
	ctx := context.Background()
	space, _ := svc.Create(ctx, sub["yann@facile.studio"], "Alpha")
	push := []SpaceEntitlements{{ID: space.ID, Entitlements: values("glouton.access", "true")}}
	if _, err := svc.SetEntitlements(ctx, "ardoise", 18, push, nil); err != nil {
		t.Fatalf("first push: %v", err)
	}
	_, err := svc.SetEntitlements(ctx, "ardoise", 17, nil, nil)
	var stale *StaleRevisionError
	if !errors.As(err, &stale) || stale.Current != 18 {
		t.Fatalf("an older push answered %v, want a stale revision carrying 18", err)
	}
	snapshot, _ := svc.Snapshot(ctx)
	if len(spaceValues(snapshot)) != 1 {
		t.Error("a refused push still cleared the stored entitlements")
	}
	if _, err := svc.SetEntitlements(ctx, "ardoise", 18, push, nil); err != nil {
		t.Errorf("a retry of the current revision was refused: %v", err)
	}
}

// TestAnUnverifiedEmailNamesNobody keeps a push from attaching a plan to an
// address its holder never proved.
func TestAnUnverifiedEmailNamesNobody(t *testing.T) {
	svc, db, sub := fixture(t, "bob@facile.studio")
	if err := db.Model(&schemas.User{}).Where("id = ?", sub["bob@facile.studio"]).
		Update("email_verified", false).Error; err != nil {
		t.Fatalf("unverifying bob: %v", err)
	}
	applied, err := svc.SetEntitlements(context.Background(), "ardoise", 1, nil,
		[]PersonEntitlements{{Email: "bob@facile.studio", Entitlements: values("glouton.access", "true")}})
	if err != nil || applied.SkippedPeople != 1 {
		t.Errorf("applied = %+v, err = %v; want bob skipped", applied, err)
	}
}

// TestMalformedPushesAreRefused pins the validation every push passes
// before anything is written.
func TestMalformedPushesAreRefused(t *testing.T) {
	svc, _, _ := fixture(t)
	tooMany := map[string]json.RawMessage{}
	for i := 0; i <= MaxEntitlementKeys; i++ {
		tooMany[fmt.Sprintf("app.feature_%d", i)] = json.RawMessage("1")
	}
	cases := map[string]struct {
		revision int64
		scope    map[string]json.RawMessage
	}{
		"revision zero":     {0, values("nuage.storage_gb", "1")},
		"uppercase key":     {1, values("Nuage.storage_gb", "1")},
		"key without a dot": {1, values("nuage", "1")},
		"fraction":          {1, values("nuage.storage_gb", "1.5")},
		"below -1":          {1, values("nuage.storage_gb", "-2")},
		"string":            {1, values("glouton.access", `"yes"`)},
		"null":              {1, values("glouton.access", "null")},
		"object":            {1, values("glouton.access", "{}")},
		"overflow":          {1, values("nuage.storage_gb", "99999999999999999999")},
		"too many keys":     {1, tooMany},
	}
	for name, tc := range cases {
		_, err := svc.SetEntitlements(context.Background(), "ardoise", tc.revision,
			[]SpaceEntitlements{{ID: "spc_x", Entitlements: tc.scope}}, nil)
		if !errors.Is(err, ErrInvalidEntitlements) {
			t.Errorf("%s: err = %v, want ErrInvalidEntitlements", name, err)
		}
	}
	if _, err := svc.SetEntitlements(context.Background(), "ardoise", 1, nil, []PersonEntitlements{{}}); !errors.Is(err, ErrInvalidEntitlements) {
		t.Errorf("a person with neither sub nor email: err = %v", err)
	}
}
