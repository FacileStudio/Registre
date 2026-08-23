package seed

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// write puts a seed document in a scratch file and returns its path.
func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seed.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing the seed: %v", err)
	}
	return path
}

// register opens a test database and returns the register over it.
func register(t *testing.T) (*clients.Service, *gorm.DB) {
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
	return clients.New(db, credentials.New(2)), db
}

// TestATypoIsAnErrorRatherThanASilentlyIgnoredKey is the whole reason the seed
// exists. SPEC §2 is about an identity layer whose version-controlled copy
// disclaimed its own accuracy; a file that says one thing and applies another
// would reproduce that here, in git, with a straight face.
func TestATypoIsAnErrorRatherThanASilentlyIgnoredKey(t *testing.T) {
	t.Setenv("AGENT_SECRET", "set")
	path := write(t, `service_accounts:
  - client_id: agent
    secret_env: AGENT_SECRET
    scopez: [courrier.read]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("a misspelled key was accepted")
	}
	if !strings.Contains(err.Error(), "scopez") {
		t.Errorf("the error does not name the offending key: %v", err)
	}
}

func TestLoadRejectsADocumentItCouldNotApply(t *testing.T) {
	for name, body := range map[string]string{
		"no client_id":  "service_accounts:\n  - secret_env: SET_SECRET\n",
		"no secret_env": "service_accounts:\n  - client_id: agent\n",
		"a duplicate id": "service_accounts:\n  - client_id: agent\n    secret_env: SET_SECRET\n" +
			"  - client_id: agent\n    secret_env: SET_SECRET\n",
		"a bad ttl":      "service_accounts:\n  - client_id: agent\n    secret_env: SET_SECRET\n    access_token_ttl: soon\n",
		"a negative ttl": "service_accounts:\n  - client_id: agent\n    secret_env: SET_SECRET\n    access_token_ttl: -1h\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SET_SECRET", "set")
			if _, err := Load(write(t, body)); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

// TestAnUnsetSecretFailsTheBoot: a declared account with no credential is a
// half-configured client that nothing can authenticate as, and discovering
// that at the first token request is discovering it too late.
func TestAnUnsetSecretFailsTheBoot(t *testing.T) {
	_, err := Load(write(t, "service_accounts:\n  - client_id: agent\n    secret_env: REGISTRE_TEST_ABSENT\n"))
	if err == nil {
		t.Fatal("an account whose secret_env is unset was accepted")
	}
	if !strings.Contains(err.Error(), "REGISTRE_TEST_ABSENT") {
		t.Errorf("the error does not name the variable to set: %v", err)
	}
}

// TestAnEmptySecretNeverBecomesACredential is the second guard, and the one
// that matters: hashing an empty string produces a perfectly valid hash, which
// an empty secret would then match.
func TestAnEmptySecretNeverBecomesACredential(t *testing.T) {
	account := ServiceAccount{ClientID: "agent", SecretEnv: "REGISTRE_TEST_EMPTY"}
	if _, err := account.row(context.Background(), nil); err == nil {
		t.Fatal("a declaration with no secret produced a register row")
	}
}

// TestApplyIsIdempotentAndKeepsTheSubject is the property SPEC §9 turns on:
// apps match an account on (issuer, subject), so a seed that re-minted the
// subject on every boot would orphan the account in every app that had ever
// seen it — silently, and once per restart.
func TestApplyIsIdempotentAndKeepsTheSubject(t *testing.T) {
	store, _ := register(t)
	t.Setenv("REGISTRE_TEST_SECRET", "a-secret-worth-keeping")
	ctx := context.Background()
	file := load(t, `service_accounts:
  - client_id: agent
    name: An agent
    secret_env: REGISTRE_TEST_SECRET
    scopes: [courrier.read]
    audiences: [courrier]
    access_token_ttl: 30m
`)
	logs := slog.New(slog.DiscardHandler)

	if err := Apply(ctx, store, file, logs); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	first, err := store.ByClientID(ctx, "agent")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if err := Apply(ctx, store, file, logs); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	second, err := store.ByClientID(ctx, "agent")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if second.Subject != first.Subject {
		t.Errorf("the subject moved from %s to %s across two boots", first.Subject, second.Subject)
	}
	if second.ID != first.ID {
		t.Errorf("the row was recreated: id %d became %d", first.ID, second.ID)
	}
	if _, err := store.Authenticate(ctx, "agent", "a-secret-worth-keeping"); err != nil {
		t.Errorf("the seeded secret no longer authenticates after a re-apply: %v", err)
	}
	if second.AccessTokenTTL != 1800 {
		t.Errorf("access_token_ttl = %d seconds, want 1800", second.AccessTokenTTL)
	}
	if !second.Scopes.Contains("courrier.read") || !second.Audiences.Contains("courrier") {
		t.Errorf("the seeded lists did not survive: scopes %v audiences %v", second.Scopes, second.Audiences)
	}
}

// TestReapplyingTheSeedLeavesADisabledAccountDisabled is a revocation test
// wearing a seed test's clothes.
//
// Disabling a compromised service account is the whole response to a leaked
// credential. Before the register wrote only the declared columns, a re-apply
// cleared disabled_at, so the account came back live with the same secret on
// the next deploy, restart or crash — a revocation that undid itself, having
// already been reported as done.
func TestReapplyingTheSeedLeavesADisabledAccountDisabled(t *testing.T) {
	store, db := register(t)
	t.Setenv("REGISTRE_TEST_SECRET", "leaked")
	ctx := context.Background()
	file := load(t, "service_accounts:\n  - client_id: agent\n    secret_env: REGISTRE_TEST_SECRET\n")
	logs := slog.New(slog.DiscardHandler)

	if err := Apply(ctx, store, file, logs); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	err := db.Model(&schemas.Client{}).Where("client_id = ?", "agent").
		Update("disabled_at", time.Now().UTC()).Error
	if err != nil {
		t.Fatalf("disabling: %v", err)
	}
	if _, err := store.Authenticate(ctx, "agent", "leaked"); err == nil {
		t.Fatal("a disabled account still authenticated")
	}

	if err := Apply(ctx, store, file, logs); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if _, err := store.Authenticate(ctx, "agent", "leaked"); err == nil {
		t.Fatal("re-applying the seed brought a disabled account back to life")
	}

	var row schemas.Client
	if err := db.Where("client_id = ?", "agent").Take(&row).Error; err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if row.DisabledAt == nil {
		t.Fatal("disabled_at was cleared by the re-apply")
	}
}

// load reads a seed document from a scratch file, failing the test rather than
// returning an error nobody checks.
func load(t *testing.T, body string) File {
	t.Helper()
	file, err := Load(write(t, body))
	if err != nil {
		t.Fatalf("loading the seed: %v", err)
	}
	return file
}
