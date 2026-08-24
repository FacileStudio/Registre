package invites

import (
	"context"
	"errors"
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
	verifier := credentials.New(2)
	return New(db, verifier), identity.New(db, verifier), db
}

func TestRedeemSetsThePasswordOnANewAccount(t *testing.T) {
	svc, users, _ := service(t)
	ctx := context.Background()

	token, err := svc.Issue(ctx, "Human@Example.com", "operator")
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	if err := svc.Redeem(ctx, token, "a real password"); err != nil {
		t.Fatalf("redeeming: %v", err)
	}
	user, err := users.Verify(ctx, "human@example.com", "a real password")
	if err != nil {
		t.Fatalf("the redeemed password does not verify: %v", err)
	}
	if !user.EmailVerified {
		t.Fatal("redemption did not mark the address verified")
	}
}

func TestRedeemSetsThePasswordOnAnExistingAccount(t *testing.T) {
	svc, users, _ := service(t)
	ctx := context.Background()

	if _, err := users.Create(ctx, identity.NewUser{
		Email: "existing@example.com", Password: "the old password",
	}); err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	token, _ := svc.Issue(ctx, "existing@example.com", "operator")
	if err := svc.Redeem(ctx, token, "the new password"); err != nil {
		t.Fatalf("redeeming: %v", err)
	}
	if _, err := users.Verify(ctx, "existing@example.com", "the new password"); err != nil {
		t.Fatalf("the new password does not verify: %v", err)
	}
	if _, err := users.Verify(ctx, "existing@example.com", "the old password"); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("the old password still verifies")
	}
}

func TestATokenIsSingleUse(t *testing.T) {
	svc, _, _ := service(t)
	ctx := context.Background()

	token, _ := svc.Issue(ctx, "once@example.com", "operator")
	if err := svc.Redeem(ctx, token, "first password"); err != nil {
		t.Fatalf("first redemption: %v", err)
	}
	if err := svc.Redeem(ctx, token, "second password"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("the second redemption answered %v, want ErrInvalid", err)
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	svc, _, db := service(t)
	ctx := context.Background()

	token, _ := svc.Issue(ctx, "late@example.com", "operator")
	if err := db.Model(&schemas.InviteToken{}).Where("email = ?", "late@example.com").
		Update("expires_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expiring the invitation: %v", err)
	}
	if err := svc.Redeem(ctx, token, "a password"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an expired invitation answered %v, want ErrInvalid", err)
	}
}

func TestAnUnknownTokenFails(t *testing.T) {
	svc, _, _ := service(t)
	err := svc.Redeem(context.Background(), "not-a-token", "a password")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("an unknown token answered %v, want ErrInvalid", err)
	}
}

func TestIssueRefusesNonAddresses(t *testing.T) {
	svc, _, _ := service(t)
	if _, err := svc.Issue(context.Background(), "not-an-address", "operator"); err == nil {
		t.Fatal("issued an invitation for something that is not an address")
	}
}

func TestRedeemRefusesADisabledAccount(t *testing.T) {
	svc, users, db := service(t)
	ctx := context.Background()

	created, err := users.Create(ctx, identity.NewUser{
		Email: "gone@example.com", Password: "the old password",
	})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	reached, err := users.Disable(ctx, created.ID, time.Now().UTC())
	if err != nil || len(reached) != 0 {
		t.Fatalf("disabling the account: reached %v, error %v", reached, err)
	}
	token, err := svc.Issue(ctx, "gone@example.com", "operator")
	if err == nil {
		t.Fatal("issuing an invitation for a disabled account answered nil error")
	}
	if token != "" {
		t.Fatal("issuing an invitation for a disabled account returned a token")
	}
	var count int64
	if err := db.Model(&schemas.InviteToken{}).Count(&count).Error; err != nil {
		t.Fatalf("counting invitations: %v", err)
	}
	if count != 0 {
		t.Fatal("the refused invitation was recorded")
	}
}
