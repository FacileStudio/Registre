package keys

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// root is the throwaway key the tests seal signing keys under.
var root = []byte("0123456789abcdef0123456789abcdef")

// lifecycle is a rotation tight enough to drive by hand.
func lifecycle() Config {
	return Config{RotateEvery: time.Hour, PublishDelay: 10 * time.Minute, Retention: 2 * time.Hour}
}

// service opens a test database and returns a key service over it.
func service(t *testing.T, cfg Config) (*Service, *gorm.DB) {
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
	return New(db, root, cfg, slog.New(slog.DiscardHandler)), db
}

// TestEnsureRecoversFromZeroActiveKeys is the break-glass half of SPEC §7 that
// costs nothing to have.
//
// A register holding rows but no active key cannot sign. If Ensure only minted
// into a completely empty table — which is what it did — that state survived
// every restart, and the only recovery was hand-editing the database that
// holds every credential in the suite.
func TestEnsureRecoversFromZeroActiveKeys(t *testing.T) {
	keyring, db := service(t, lifecycle())
	ctx := context.Background()
	if err := keyring.Ensure(ctx); err != nil {
		t.Fatalf("first ensure: %v", err)
	}

	retired := time.Now().UTC()
	err := db.Model(&schemas.SigningKey{}).Where("state = ?", schemas.KeyActive).
		Updates(map[string]any{
			"state": schemas.KeyRetired, "retired_at": retired, "expires_at": retired.Add(2 * time.Hour),
		}).Error
	if err != nil {
		t.Fatalf("retiring the only key: %v", err)
	}
	if _, err := keyring.Signing(ctx); !errors.Is(err, ErrNoActiveKey) {
		t.Fatalf("the fixture did not reach the state under test: %v", err)
	}

	restarted := New(db, root, lifecycle(), slog.New(slog.DiscardHandler))
	if err := restarted.Ensure(ctx); err != nil {
		t.Fatalf("ensure after restart: %v", err)
	}
	if _, err := restarted.Signing(ctx); err != nil {
		t.Fatalf("still cannot sign after a restart: %v", err)
	}

	set, err := restarted.Set(ctx)
	if err != nil {
		t.Fatalf("reading the key set: %v", err)
	}
	if len(set) != 2 {
		t.Errorf("the key set holds %d keys, want 2: the retired one is still inside its retention", len(set))
	}
}

// TestEnsureIsANoOpWhenAKeyIsAlreadySigning: a restart must not mint a key on
// every boot. Each one is published and signing in the same instant, which the
// lifecycle otherwise refuses.
func TestEnsureIsANoOpWhenAKeyIsAlreadySigning(t *testing.T) {
	keyring, db := service(t, lifecycle())
	ctx := context.Background()
	if err := keyring.Ensure(ctx); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	first, err := keyring.Signing(ctx)
	if err != nil {
		t.Fatalf("reading the active key: %v", err)
	}

	for range 3 {
		if err := New(db, root, lifecycle(), slog.New(slog.DiscardHandler)).Ensure(ctx); err != nil {
			t.Fatalf("ensure on restart: %v", err)
		}
	}

	var count int64
	if err := db.Model(&schemas.SigningKey{}).Count(&count).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if count != 1 {
		t.Errorf("three restarts left %d keys, want 1", count)
	}
	second, err := keyring.Signing(ctx)
	if err != nil {
		t.Fatalf("reading the active key: %v", err)
	}
	if second.ID() != first.ID() {
		t.Errorf("a restart replaced the signing key: %s became %s", first.ID(), second.ID())
	}
}

// TestOnlyOneKeyEverSigns is the partial unique index doing its job. Two
// replicas promoting their own published key at the same second would each
// believe they had rotated, and tokens would be signed under a kid the other
// had already retired.
func TestOnlyOneKeyEverSigns(t *testing.T) {
	keyring, db := service(t, lifecycle())
	ctx := context.Background()
	if err := keyring.Ensure(ctx); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	second, err := keyring.mint(schemas.KeyActive)
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	if err := db.Create(second).Error; err == nil {
		t.Fatal("the database accepted a second active signing key")
	}
}

// TestAChangedRootKeyFailsTheBootRatherThanTheFirstToken is the failure this
// guard exists to move.
//
// Every row survives a root key change: the register still lists its clients,
// the JWKS still serves the public halves, /ready is green. Nothing is wrong
// until a token is asked for, and then it is a 500 saying "message
// authentication failed", which names neither the variable nor the restore that
// caused it.
func TestAChangedRootKeyFailsTheBootRatherThanTheFirstToken(t *testing.T) {
	keyring, db := service(t, lifecycle())
	ctx := context.Background()
	if err := keyring.Ensure(ctx); err != nil {
		t.Fatalf("first boot: %v", err)
	}

	elsewhere := New(db, []byte("fedcba9876543210fedcba9876543210"), lifecycle(), slog.New(slog.DiscardHandler))
	err := elsewhere.Ensure(ctx)
	if err == nil {
		t.Fatal("a boot under a different ENCRYPTION_KEY was accepted")
	}
	if !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
		t.Errorf("the error does not name the variable to fix: %v", err)
	}
}
