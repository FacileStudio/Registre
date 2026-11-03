package directory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"gorm.io/gorm"
)

// fixture opens a clean database and mints the named users, answering their
// subjects by email.
func fixture(t *testing.T, emails ...string) (*Service, *gorm.DB, map[string]string) {
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
	users := identity.New(db, credentials.New(2))
	subjects := map[string]string{}
	for _, email := range emails {
		user, err := users.Create(context.Background(), identity.NewUser{Email: email, Password: "correct horse battery"})
		if err != nil {
			t.Fatalf("creating %s: %v", email, err)
		}
		subjects[email] = user.ID
	}
	return New(db, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))), db, subjects
}

// rolesIn flattens a snapshot's one space into email -> role.
func rolesIn(t *testing.T, snapshot wire.Snapshot) map[string]string {
	t.Helper()
	if len(snapshot.Spaces) != 1 {
		t.Fatalf("snapshot has %d spaces, want the 1 merged space: %+v", len(snapshot.Spaces), snapshot.Spaces)
	}
	out := map[string]string{}
	for _, member := range snapshot.Spaces[0].Members {
		out[member.Email] = member.Role
	}
	return out
}

// TestImportsMergeByNameKeepTheHighestRoleAndHappenOnce is the first-deploy
// contract: the same team recreated in two apps becomes one space, everybody
// keeps the highest role they held, unknown people are skipped, and an app
// importing again cannot undo what an owner changed since.
func TestImportsMergeByNameKeepTheHighestRoleAndHappenOnce(t *testing.T) {
	svc, _, sub := fixture(t, "yann@facile.studio", "bob@facile.studio", "carol@facile.studio")
	ctx := context.Background()

	_, err := svc.Import(ctx, "agenda", []wire.ImportSpace{
		{Name: "Facile", Members: []wire.ImportMember{
			{Subject: sub["yann@facile.studio"], Role: "owner"},
			{Email: "BOB@facile.studio", Role: "admin"},
			{Email: "ghost@nowhere.test", Role: "owner"},
		}},
		{Name: "Nobody", Members: []wire.ImportMember{{Email: "ghost@nowhere.test", Role: "owner"}}},
	})
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	snapshot, err := svc.Import(ctx, "opus", []wire.ImportSpace{{Name: "  facile ", Members: []wire.ImportMember{
		{Email: "bob@facile.studio", Role: "owner"},
		{Email: "carol@facile.studio", Role: "viewer"},
		{Email: "yann@facile.studio", Role: "member"},
	}}})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	want := map[string]string{"yann@facile.studio": "owner", "bob@facile.studio": "owner", "carol@facile.studio": "member"}
	if got := rolesIn(t, snapshot); !equal(got, want) {
		t.Fatalf("merged roles = %v, want %v", got, want)
	}

	if err := svc.RemoveMember(ctx, sub["yann@facile.studio"], snapshot.Spaces[0].ID, sub["carol@facile.studio"]); err != nil {
		t.Fatalf("removing carol: %v", err)
	}
	again, err := svc.Import(ctx, "opus", []wire.ImportSpace{{Name: "Facile", Members: []wire.ImportMember{
		{Email: "carol@facile.studio", Role: "owner"},
	}}})
	if err != nil {
		t.Fatalf("repeated import: %v", err)
	}
	if _, back := rolesIn(t, again)["carol@facile.studio"]; back {
		t.Error("a repeated import brought back a member an owner removed")
	}
	if again.Version <= snapshot.Version {
		t.Errorf("version %d did not rise past %d after a removal", again.Version, snapshot.Version)
	}
}

// TestDisablingSomebodyTakesThemOutOfEverySnapshot is how deprovisioning
// reaches the apps: a disabled human simply stops appearing.
func TestDisablingSomebodyTakesThemOutOfEverySnapshot(t *testing.T) {
	svc, db, sub := fixture(t, "yann@facile.studio", "bob@facile.studio")
	ctx := context.Background()
	space, err := svc.Create(ctx, sub["yann@facile.studio"], "Facile")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.AddMember(ctx, sub["yann@facile.studio"], space.ID, "bob@facile.studio", "member"); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	now := time.Now()
	if err := db.Model(&schemas.User{}).Where("id = ?", sub["bob@facile.studio"]).Update("disabled_at", &now).Error; err != nil {
		t.Fatalf("disabling bob: %v", err)
	}
	snapshot, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, present := rolesIn(t, snapshot)["bob@facile.studio"]; present {
		t.Error("a disabled user is still in the snapshot every app applies")
	}
}

// TestRanksGuardEveryChange pins porte's ladder rules on the one screen that
// now governs every app: an admin cannot touch an owner or mint one, and the
// last owner can neither leave nor be demoted.
func TestRanksGuardEveryChange(t *testing.T) {
	svc, _, sub := fixture(t, "owner@facile.studio", "admin@facile.studio", "member@facile.studio")
	ctx := context.Background()
	owner, admin, member := sub["owner@facile.studio"], sub["admin@facile.studio"], sub["member@facile.studio"]
	space, err := svc.Create(ctx, owner, "Facile")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for email, role := range map[string]string{"admin@facile.studio": "admin", "member@facile.studio": "member"} {
		if _, err := svc.AddMember(ctx, owner, space.ID, email, role); err != nil {
			t.Fatalf("adding %s: %v", email, err)
		}
	}
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"an admin demotes the owner", svc.SetRole(ctx, admin, space.ID, owner, "member"), ErrForbidden},
		{"an admin promotes to owner", svc.SetRole(ctx, admin, space.ID, member, "owner"), ErrForbidden},
		{"a member removes someone", svc.RemoveMember(ctx, member, space.ID, admin), ErrForbidden},
		{"the last owner leaves", svc.Leave(ctx, owner, space.ID), ErrLastOwner},
		{"the last owner demotes themself", svc.SetRole(ctx, owner, space.ID, owner, "admin"), ErrLastOwner},
		{"a stranger reads the space", errOf(svc.Get(ctx, "nobody", space.ID)), ErrNotFound},
	}
	for _, tc := range cases {
		if !errors.Is(tc.err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, tc.err, tc.want)
		}
	}
}

// errOf drops a value and keeps the error.
func errOf[T any](_ T, err error) error { return err }

// equal compares two string maps.
func equal(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
