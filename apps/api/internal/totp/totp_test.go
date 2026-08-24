package totp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

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
	root := make([]byte, 32)
	if _, err := rand.Read(root); err != nil {
		t.Fatalf("minting a root key: %v", err)
	}
	return New(db, root), identity.New(db, credentials.New(2)), db
}

// code computes what an authenticator would show for a seed at a moment. The
// verifier under test comes from pquerna/otp, so generating with an
// independent implementation is what makes the assertion mean anything: two
// copies of the same library agreeing would prove only that it is consistent.
func code(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("decoding the seed: %v", err)
	}
	counter := make([]byte, 8)
	binary.BigEndian.PutUint64(counter, uint64(at.Unix()/Period))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000)
}

// enrolled runs a user all the way to a live second factor and returns the
// seed and the recovery codes.
func enrolled(t *testing.T, svc *Service, users *identity.Service) (schemas.User, string, []string) {
	t.Helper()
	ctx := context.Background()
	user, err := users.Create(ctx, identity.NewUser{Email: "human@example.test", Password: "a password"})
	if err != nil {
		t.Fatalf("creating the user: %v", err)
	}
	enrolment, err := svc.Begin(ctx, user, "registre")
	if err != nil {
		t.Fatalf("beginning enrolment: %v", err)
	}
	pending, err := users.ByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("re-reading the user: %v", err)
	}
	codes, err := svc.Confirm(ctx, pending, code(t, enrolment.Secret, time.Now().UTC()))
	if err != nil {
		t.Fatalf("confirming: %v", err)
	}
	live, err := users.ByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("re-reading the user: %v", err)
	}
	return live, enrolment.Secret, codes
}

// TestAFreshlyEnrolledFactorVerifiesInTheSameWindow is a regression found by
// driving the real flow end to end.
//
// Confirm used to burn the time step it verified, so the first login after
// enrolling fell inside the same thirty seconds and the code the
// authenticator was still displaying came back refused, wearing the same
// generic error a wrong code gets. Enrol and sign in is the one sequence
// every user performs exactly once, and it was the sequence that failed.
func TestAFreshlyEnrolledFactorVerifiesInTheSameWindow(t *testing.T) {
	svc, users, _ := service(t)
	user, secret, _ := enrolled(t, svc, users)

	if err := svc.Verify(context.Background(), user, code(t, secret, time.Now().UTC())); err != nil {
		t.Fatalf("a correct code was refused right after enrolment: %v", err)
	}
}

// TestACodeCannotBeSpentTwice is RFC 6238 §5.2. Without it a code captured on
// the wire stays good for the rest of its window, which is exactly the window
// somebody sitting on the wire has.
func TestACodeCannotBeSpentTwice(t *testing.T) {
	svc, users, _ := service(t)
	user, secret, _ := enrolled(t, svc, users)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := svc.Verify(ctx, user, code(t, secret, now)); err != nil {
		t.Fatalf("the first use was refused: %v", err)
	}
	if err := svc.Verify(ctx, user, code(t, secret, now)); !errors.Is(err, ErrInvalid) {
		t.Errorf("the same code was accepted twice, err = %v", err)
	}
}

// TestAWrongCodeIsRefused covers the ordinary case, and that the refusal is
// the one error every other refusal uses.
func TestAWrongCodeIsRefused(t *testing.T) {
	svc, users, _ := service(t)
	user, _, _ := enrolled(t, svc, users)

	if err := svc.Verify(context.Background(), user, "000000"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a wrong code gave %v, want ErrInvalid", err)
	}
}

// TestAnAccountWithNoFactorRefusesEveryCode stops an unenrolled account from
// being waved through by a second-factor check that has nothing to check.
func TestAnAccountWithNoFactorRefusesEveryCode(t *testing.T) {
	svc, users, _ := service(t)
	user, err := users.Create(context.Background(), identity.NewUser{
		Email: "bare@example.test", Password: "a password",
	})
	if err != nil {
		t.Fatalf("creating the user: %v", err)
	}

	if err := svc.Verify(context.Background(), user, "000000"); !errors.Is(err, ErrInvalid) {
		t.Errorf("an account with no factor gave %v, want ErrInvalid", err)
	}
}

// TestBeginRefusesToReplaceALiveFactor keeps enrolment from being the way
// around enrolment. Replacing a confirmed factor without proving the current
// one would make the second factor optional for whoever holds the session.
func TestBeginRefusesToReplaceALiveFactor(t *testing.T) {
	svc, users, _ := service(t)
	user, _, _ := enrolled(t, svc, users)

	if _, err := svc.Begin(context.Background(), user, "registre"); err == nil {
		t.Error("Begin replaced a live factor")
	}
}

// TestDisableTakesTheRecoveryCodesWithIt checks the codes do not outlive the
// factor. A leftover list is a way into an account whose owner believes the
// second factor is off.
func TestDisableTakesTheRecoveryCodesWithIt(t *testing.T) {
	svc, users, db := service(t)
	user, _, _ := enrolled(t, svc, users)

	if err := svc.Disable(context.Background(), user.ID); err != nil {
		t.Fatalf("disabling: %v", err)
	}
	var count int64
	if err := db.Model(&schemas.RecoveryCode{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 0 {
		t.Errorf("%d recovery codes survived disabling the factor", count)
	}
}

// TestRegeneratingReplacesEveryCode checks the old list stops working. Topping
// up would leave the codes that were regenerated because they may have been
// seen.
func TestRegeneratingReplacesEveryCode(t *testing.T) {
	svc, users, _ := service(t)
	user, _, old := enrolled(t, svc, users)
	ctx := context.Background()

	fresh, err := svc.RegenerateRecoveryCodes(ctx, user)
	if err != nil {
		t.Fatalf("regenerating: %v", err)
	}
	if len(fresh) != RecoveryCodeCount {
		t.Fatalf("got %d codes, want %d", len(fresh), RecoveryCodeCount)
	}
	if err := svc.RedeemRecoveryCode(ctx, user.ID, old[0]); !errors.Is(err, ErrInvalid) {
		t.Errorf("an old recovery code still works after regenerating, err = %v", err)
	}
	if err := svc.RedeemRecoveryCode(ctx, user.ID, fresh[0]); err != nil {
		t.Errorf("a fresh recovery code was refused: %v", err)
	}
}

// TestARecoveryCodeIsSingleUseAndScopedToItsOwner covers both ways a code
// could open more than it should.
func TestARecoveryCodeIsSingleUseAndScopedToItsOwner(t *testing.T) {
	svc, users, _ := service(t)
	user, _, codes := enrolled(t, svc, users)
	ctx := context.Background()

	other, err := users.Create(ctx, identity.NewUser{Email: "other@example.test", Password: "a password"})
	if err != nil {
		t.Fatalf("creating the other user: %v", err)
	}
	if err := svc.RedeemRecoveryCode(ctx, other.ID, codes[0]); !errors.Is(err, ErrInvalid) {
		t.Errorf("one account's recovery code opened another, err = %v", err)
	}
	if err := svc.RedeemRecoveryCode(ctx, user.ID, codes[0]); err != nil {
		t.Fatalf("the owner's code was refused: %v", err)
	}
	if err := svc.RedeemRecoveryCode(ctx, user.ID, codes[0]); !errors.Is(err, ErrInvalid) {
		t.Errorf("a recovery code was spent twice, err = %v", err)
	}
}

// TestTheSeedIsNotReadableFromTheRow checks the column holds ciphertext. The
// seed is the factor: a database dump that carries it in the clear makes the
// second factor a formality.
func TestTheSeedIsNotReadableFromTheRow(t *testing.T) {
	svc, users, db := service(t)
	user, secret, _ := enrolled(t, svc, users)

	var row schemas.User
	if err := db.Where("id = ?", user.ID).Take(&row).Error; err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	if len(row.TOTPSecret) == 0 || len(row.TOTPNonce) == 0 {
		t.Fatal("the row carries no sealed seed at all")
	}
	if string(row.TOTPSecret) == secret {
		t.Error("the seed is stored in the clear")
	}
}
