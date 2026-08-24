package breakglass

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// restorePassword is long enough to clear MinPasswordLength, because the
// service refuses anything shorter and a fixture that trips its own guard
// tests the guard rather than the restore.
const restorePassword = "a break-glass password nobody guesses"

func service(t *testing.T) (*Service, *identity.Service, *gorm.DB) {
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
	verifier := credentials.New(2)
	return New(db, verifier), identity.New(db, verifier), db
}

// TestRestoreLetsALockedOutOperatorBackIn is the drill SPEC §7 gates phase 4
// on. It builds the worst state the operator account can be in: disabled,
// second factor enrolled on a device that is gone, password forgotten, a live
// session belonging to whoever caused the incident, and asserts that one
// offline call answers all four.
func TestRestoreLetsALockedOutOperatorBackIn(t *testing.T) {
	svc, users, db := service(t)
	ctx := context.Background()

	user, err := users.Create(ctx, identity.NewUser{
		Email: "operator@facile.studio", Password: "the forgotten one", Admin: true,
	})
	if err != nil {
		t.Fatalf("creating the operator: %v", err)
	}
	enrol(t, db, user.ID)
	disable(t, db, user.ID)
	if _, err := users.StartSession(ctx, user.ID, "sablier", time.Now().UTC()); err != nil {
		t.Fatalf("opening a session: %v", err)
	}

	outcome, err := svc.Restore(ctx, "Operator@Facile.Studio", restorePassword)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if outcome.Created {
		t.Error("restore minted a new account for an address that already had one")
	}
	if !outcome.Reenabled {
		t.Error("restore did not report re-enabling a disabled account")
	}
	if !outcome.FactorCleared {
		t.Error("restore did not report clearing the second factor")
	}
	if outcome.SessionsEnded != 1 {
		t.Errorf("ended %d sessions, want 1", outcome.SessionsEnded)
	}

	restored, err := users.Verify(ctx, "operator@facile.studio", restorePassword)
	if err != nil {
		t.Fatalf("the restored password does not verify: %v", err)
	}
	if !restored.Enabled() {
		t.Error("the restored account is still disabled")
	}
	if !restored.Admin {
		t.Error("the restored account does not hold admin")
	}
	if restored.TOTPEnrolled() {
		t.Error("the second factor survived the restore, so the lost device still gates the account")
	}
	assertNoRecoveryCodes(t, db, user.ID)
	assertNoLiveSessions(t, db, user.ID)
}

// TestRestoreMintsTheAccountWhenTheAddressHasNone covers the case the drill
// exists for at its worst: the last administrator row deleted outright.
func TestRestoreMintsTheAccountWhenTheAddressHasNone(t *testing.T) {
	svc, users, _ := service(t)
	ctx := context.Background()

	outcome, err := svc.Restore(ctx, "nobody@facile.studio", restorePassword)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if !outcome.Created {
		t.Fatal("restore did not report creating the account")
	}
	user, err := users.Verify(ctx, "nobody@facile.studio", restorePassword)
	if err != nil {
		t.Fatalf("the minted account does not verify: %v", err)
	}
	if !user.Admin {
		t.Error("a break-glass account that does not hold admin cannot fix anything")
	}
}

// TestRestoreRefusesAShortPassword keeps the floor honest. The account this
// mints reaches every app in the suite, and it is set by somebody improvising
// under pressure, which is exactly when a short password gets chosen.
func TestRestoreRefusesAShortPassword(t *testing.T) {
	svc, _, db := service(t)

	if _, err := svc.Restore(context.Background(), "operator@facile.studio", "short"); err == nil {
		t.Fatal("restore accepted a password under the floor")
	}
	var count int64
	if err := db.Model(&schemas.User{}).Count(&count).Error; err != nil {
		t.Fatalf("counting users: %v", err)
	}
	if count != 0 {
		t.Errorf("a refused restore still wrote %d user rows", count)
	}
}

// TestRestoreRefusesSomethingThatIsNotAnAddress guards the argument that
// names which account gets the keys to everything.
func TestRestoreRefusesSomethingThatIsNotAnAddress(t *testing.T) {
	svc, _, _ := service(t)

	_, err := svc.Restore(context.Background(), "operator", restorePassword)
	if err == nil {
		t.Fatal("restore accepted an argument that is not an email address")
	}
	if !strings.Contains(err.Error(), "email address") {
		t.Errorf("the refusal does not say what was wrong: %v", err)
	}
}

// enrol puts a confirmed second factor on a user, standing in for the device
// the drill assumes has been lost. It writes the columns directly rather than
// going through internal/totp: what the restore has to clear is the row, and
// a real enrolment would only add a dependency on the thing being torn down.
func enrol(t *testing.T, db *gorm.DB, userID string) {
	t.Helper()
	at := time.Now().UTC()
	err := db.Model(&schemas.User{}).Where("id = ?", userID).Updates(map[string]any{
		"totp_secret": []byte("sealed"), "totp_nonce": []byte("nonce"), "totp_confirmed_at": at,
	}).Error
	if err != nil {
		t.Fatalf("enrolling a factor: %v", err)
	}
	code := schemas.RecoveryCode{ID: "code-1", UserID: userID, CodeHash: "digest", CreatedAt: at}
	if err := db.Create(&code).Error; err != nil {
		t.Fatalf("seeding a recovery code: %v", err)
	}
}

// disable writes disabled_at directly rather than going through
// identity.Disable, which ends the account's sessions as it goes. The drill
// needs a live session at the moment of the restore to prove the restore
// closes it, and going through the service would have left nothing to close
// and quietly turned that assertion into a tautology.
func disable(t *testing.T, db *gorm.DB, userID string) {
	t.Helper()
	err := db.Model(&schemas.User{}).Where("id = ?", userID).
		Update("disabled_at", time.Now().UTC()).Error
	if err != nil {
		t.Fatalf("disabling: %v", err)
	}
}

func assertNoRecoveryCodes(t *testing.T, db *gorm.DB, userID string) {
	t.Helper()
	var count int64
	if err := db.Model(&schemas.RecoveryCode{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("counting recovery codes: %v", err)
	}
	if count != 0 {
		t.Errorf("%d recovery codes survived the restore, so the old list still opens the account", count)
	}
}

func assertNoLiveSessions(t *testing.T, db *gorm.DB, userID string) {
	t.Helper()
	var count int64
	err := db.Model(&schemas.Session{}).
		Where("user_id = ? AND ended_at IS NULL", userID).Count(&count).Error
	if err != nil {
		t.Fatalf("counting sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("%d sessions survived the restore, so whoever caused the incident is still signed in", count)
	}
}
