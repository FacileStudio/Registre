package passkeys_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys/passkeytest"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

const (
	rpID   = "localhost"
	origin = "http://localhost:8080"
)

func open(t *testing.T) (*passkeys.Service, *gorm.DB) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	svc, err := passkeys.New(db, passkeys.Config{RPID: rpID, Origins: []string{origin}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("passkeys.New: %v", err)
	}
	return svc, db
}

func user(t *testing.T, db *gorm.DB, id, hash string) {
	t.Helper()
	if err := db.Create(&schemas.User{ID: id, Email: id + "@example.com", PasswordHash: hash}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

// register runs a full registration ceremony for userID through the
// virtual authenticator and stores the credential.
func register(t *testing.T, svc *passkeys.Service, device *passkeytest.Authenticator, userID string) schemas.Passkey {
	t.Helper()
	ctx := context.Background()
	options, err := svc.BeginRegistration(ctx, passkeys.PurposeRegister, userID, userID+"@example.com")
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	answer, err := device.Create(options.PublicKey)
	if err != nil {
		t.Fatalf("authenticator create: %v", err)
	}
	registered, err := svc.FinishRegistration(ctx, options.Ceremony, passkeys.PurposeRegister, answer)
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	row, err := svc.Store(ctx, nil, registered, "Laptop")
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	return row
}

// TestAPasskeyRegistersAndSignsIn runs registration then a discoverable
// assertion through go-webauthn's real verification.
func TestAPasskeyRegistersAndSignsIn(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	device := passkeytest.New(rpID, origin)
	register(t, svc, device, "alice")

	ctx := context.Background()
	options, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	answer, err := device.Get(options.PublicKey)
	if err != nil {
		t.Fatalf("authenticator get: %v", err)
	}
	userID, err := svc.FinishLogin(ctx, options.Ceremony, answer)
	if err != nil || userID != "alice" {
		t.Fatalf("FinishLogin = %q, %v; want alice", userID, err)
	}
	if _, err := svc.FinishLogin(ctx, options.Ceremony, answer); !errors.Is(err, passkeys.ErrInvalid) {
		t.Fatalf("replayed assertion = %v, want ErrInvalid", err)
	}
}

// TestAWrongOriginIsRefused proves the origin check is exact.
func TestAWrongOriginIsRefused(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	ctx := context.Background()
	options, err := svc.BeginRegistration(ctx, passkeys.PurposeRegister, "alice", "alice@example.com")
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	answer, _ := passkeytest.New(rpID, "http://evil.localhost:8080").Create(options.PublicKey)
	if _, err := svc.FinishRegistration(ctx, options.Ceremony, passkeys.PurposeRegister, answer); !errors.Is(err, passkeys.ErrInvalid) {
		t.Fatalf("FinishRegistration from another origin = %v, want ErrInvalid", err)
	}
}

// TestACeremonyIsBoundToItsPurpose refuses a registration finished as
// another purpose.
func TestACeremonyIsBoundToItsPurpose(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	ctx := context.Background()
	options, _ := svc.BeginRegistration(ctx, passkeys.PurposeRegister, "alice", "alice@example.com")
	answer, _ := passkeytest.New(rpID, origin).Create(options.PublicKey)
	if _, err := svc.FinishRegistration(ctx, options.Ceremony, passkeys.PurposeSignup, answer); !errors.Is(err, passkeys.ErrInvalid) {
		t.Fatalf("cross-purpose finish = %v, want ErrInvalid", err)
	}
}

// TestTheLastFactorCannotBeRemoved keeps a passkey-only account reachable.
func TestTheLastFactorCannotBeRemoved(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	only := register(t, svc, passkeytest.New(rpID, origin), "alice")
	if err := svc.Remove(context.Background(), "alice", only.ID); !errors.Is(err, passkeys.ErrLastFactor) {
		t.Fatalf("removing the only passkey = %v, want ErrLastFactor", err)
	}
	second := register(t, svc, passkeytest.New(rpID, origin), "alice")
	if err := svc.Remove(context.Background(), "alice", second.ID); err != nil {
		t.Fatalf("removing one of two passkeys: %v", err)
	}
	if err := svc.Rename(context.Background(), "alice", only.ID, "Phone"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := svc.Rename(context.Background(), "bob", only.ID, "Mine"); !errors.Is(err, passkeys.ErrNotFound) {
		t.Fatalf("renaming someone else's passkey = %v, want ErrNotFound", err)
	}
}

// TestAPasswordAccountMayRemoveItsOnlyPasskey: the password is a factor.
func TestAPasswordAccountMayRemoveItsOnlyPasskey(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "$argon2id$stub")
	only := register(t, svc, passkeytest.New(rpID, origin), "alice")
	if err := svc.Remove(context.Background(), "alice", only.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

// TestReauthenticationIsBoundToItsAccount refuses a reauth ceremony begun
// for one account and finished for another.
func TestReauthenticationIsBoundToItsAccount(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	device := passkeytest.New(rpID, origin)
	register(t, svc, device, "alice")
	ctx := context.Background()
	options, err := svc.BeginReauth(ctx, "alice", "alice@example.com")
	if err != nil {
		t.Fatalf("BeginReauth: %v", err)
	}
	answer, _ := device.Get(options.PublicKey)
	if err := svc.FinishReauth(ctx, options.Ceremony, "bob", answer); !errors.Is(err, passkeys.ErrInvalid) {
		t.Fatalf("FinishReauth as bob = %v, want ErrInvalid", err)
	}
	options, _ = svc.BeginReauth(ctx, "alice", "alice@example.com")
	answer, _ = device.Get(options.PublicKey)
	if err := svc.FinishReauth(ctx, options.Ceremony, "alice", answer); err != nil {
		t.Fatalf("FinishReauth: %v", err)
	}
}

// TestSignInRequiresUserVerification refuses an assertion proving presence
// only: a passkey sign-in owes no second factor, so it must verify the person.
func TestSignInRequiresUserVerification(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	device := passkeytest.New(rpID, origin)
	register(t, svc, device, "alice")
	device.SkipVerification = true
	options, _ := svc.BeginLogin(context.Background())
	answer, _ := device.Get(options.PublicKey)
	if _, err := svc.FinishLogin(context.Background(), options.Ceremony, answer); !errors.Is(err, passkeys.ErrInvalid) {
		t.Fatalf("an unverified assertion = %v, want ErrInvalid", err)
	}
}

// TestRegistrationRequiresUserVerification refuses a passkey created without
// verifying the person.
func TestRegistrationRequiresUserVerification(t *testing.T) {
	svc, db := open(t)
	user(t, db, "alice", "")
	device := passkeytest.New(rpID, origin)
	device.SkipVerification = true
	ctx := context.Background()
	options, err := svc.BeginRegistration(ctx, passkeys.PurposeRegister, "alice", "alice@example.com")
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	answer, _ := device.Create(options.PublicKey)
	if _, err := svc.FinishRegistration(ctx, options.Ceremony, passkeys.PurposeRegister, answer); !errors.Is(err, passkeys.ErrInvalid) {
		t.Fatalf("an unverified registration = %v, want ErrInvalid", err)
	}
}
