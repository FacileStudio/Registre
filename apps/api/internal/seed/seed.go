// Package seed bootstraps the register from a file.
//
// SPEC §2 is about one thing: the identity layer was the only component in the
// suite configured by clicking, and its version-controlled copy disclaimed its
// own accuracy. The seed is the answer — a fresh instance comes up with its
// service accounts already declared, from a file that is read at every boot
// and is therefore never a mirror of something truer elsewhere.
//
// It carries no credentials. A secret is named, not written: each account
// points at an environment variable, and a missing one fails the boot rather
// than quietly leaving an account nobody can authenticate as.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"gopkg.in/yaml.v3"
)

// File is the whole seed document.
type File struct {
	ServiceAccounts []ServiceAccount `yaml:"service_accounts"`
}

// ServiceAccount declares one machine principal.
type ServiceAccount struct {
	ClientID string `yaml:"client_id"`
	Name     string `yaml:"name"`

	// SecretEnv names the environment variable holding this account's
	// secret. The variable is named here rather than the value being
	// written here because the seed file is in git, and a committed hash
	// is a committed credential.
	SecretEnv string `yaml:"secret_env"`

	Scopes    []string `yaml:"scopes"`
	Audiences []string `yaml:"audiences"`

	// AccessTokenTTL is a Go duration. Empty takes the process default.
	AccessTokenTTL string `yaml:"access_token_ttl"`
}

// Load reads and validates a seed file without applying it.
//
// Unknown keys are an error. A seed whose typo is ignored is a seed that says
// one thing and does another, which is the failure §2 exists to end.
func Load(path string) (File, error) {
	handle, err := os.Open(path)
	if err != nil {
		return File{}, fmt.Errorf("seed: open %s: %w", path, err)
	}
	defer func() { _ = handle.Close() }()

	decoder := yaml.NewDecoder(handle)
	decoder.KnownFields(true)

	var file File
	if err := decoder.Decode(&file); err != nil {
		return File{}, fmt.Errorf("seed: read %s: %w", path, err)
	}
	if err := file.validate(); err != nil {
		return File{}, fmt.Errorf("seed: %s: %w", path, err)
	}
	return file, nil
}

// validate rejects a document that could not be applied, before anything is
// written and while the error can still name the line the reader has open.
func (f File) validate() error {
	seen := map[string]bool{}
	for index, account := range f.ServiceAccounts {
		where := fmt.Sprintf("service_accounts[%d]", index)
		switch {
		case account.ClientID == "":
			return fmt.Errorf("%s: client_id is required", where)
		case seen[account.ClientID]:
			return fmt.Errorf("%s: client_id %q is declared twice", where, account.ClientID)
		case account.SecretEnv == "":
			return fmt.Errorf("%s (%s): secret_env is required", where, account.ClientID)
		}
		seen[account.ClientID] = true
		if _, err := account.lifetime(); err != nil {
			return fmt.Errorf("%s (%s): %w", where, account.ClientID, err)
		}
		if os.Getenv(account.SecretEnv) == "" {
			return fmt.Errorf(
				"%s (%s): secret_env %s is unset — a declared account with no credential cannot authenticate",
				where, account.ClientID, account.SecretEnv)
		}
	}
	return nil
}

// lifetime parses the declared access token lifetime.
func (a ServiceAccount) lifetime() (time.Duration, error) {
	if a.AccessTokenTTL == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(a.AccessTokenTTL)
	if err != nil {
		return 0, fmt.Errorf("access_token_ttl %q is not a duration: %w", a.AccessTokenTTL, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("access_token_ttl %q is not positive", a.AccessTokenTTL)
	}
	return parsed, nil
}

// Apply writes the seed into the register. It is idempotent: every entry is an
// upsert keyed on client_id, and an account already present keeps the subject
// it was issued, because that is what every app matched it on.
//
// It never deletes. An account dropped from the file is reported and left
// alone — a client vanishing because somebody edited YAML is a surprise nobody
// wants at boot, and disabling one is a deliberate act with a row of its own.
func Apply(ctx context.Context, register *clients.Service, file File, logger *slog.Logger) error {
	declared := map[string]bool{}
	for _, account := range file.ServiceAccounts {
		row, err := account.row(ctx, register)
		if err != nil {
			return err
		}
		if _, err := register.Upsert(ctx, row); err != nil {
			return err
		}
		declared[account.ClientID] = true
		logger.Info("registre: seeded a service account", slog.String("client_id", account.ClientID))
	}
	return reportUndeclared(ctx, register, declared, logger)
}

// row turns a declaration into the register row it becomes.
//
// The empty-secret check repeats validate's, which is not an oversight: that
// one exists to fail the boot with a message naming the variable to set, and
// this one exists because hashing an empty string produces a perfectly valid
// hash that an empty secret then matches. Only one of the two is load-bearing
// for security, and it is this one.
//
// Hashing goes
// through the register rather than through porte directly, so the seed sits
// under the same argon2 ceiling every other caller does.
func (a ServiceAccount) row(ctx context.Context, register *clients.Service) (schemas.Client, error) {
	secret := os.Getenv(a.SecretEnv)
	if secret == "" {
		return schemas.Client{}, fmt.Errorf("seed: %s has no secret in %s", a.ClientID, a.SecretEnv)
	}
	hash, err := register.HashSecret(ctx, secret)
	if err != nil {
		return schemas.Client{}, fmt.Errorf("seed: hash the secret for %s: %w", a.ClientID, err)
	}
	subject, err := ids.New()
	if err != nil {
		return schemas.Client{}, fmt.Errorf("seed: mint a subject for %s: %w", a.ClientID, err)
	}
	lifetime, err := a.lifetime()
	if err != nil {
		return schemas.Client{}, err
	}
	return schemas.Client{
		ClientID: a.ClientID, Subject: subject, Name: a.Name,
		Kind: schemas.KindServiceAccount, SecretHash: hash,
		GrantTypes:     schemas.StringList{string(zoidc.GrantTypeClientCredentials)},
		Scopes:         schemas.StringList(a.Scopes),
		Audiences:      schemas.StringList(a.Audiences),
		AccessTokenTTL: int64(lifetime / time.Second),
		Managed:        true,
	}, nil
}

// reportUndeclared logs the managed rows the file no longer names. They keep
// working; somebody has to decide whether that is what was meant.
func reportUndeclared(
	ctx context.Context, register *clients.Service, declared map[string]bool, logger *slog.Logger,
) error {
	rows, err := register.List(ctx)
	if err != nil {
		return err
	}
	var stale []string
	for _, row := range rows {
		if row.Managed && !declared[row.ClientID] {
			stale = append(stale, row.ClientID)
		}
	}
	if len(stale) > 0 {
		logger.Warn("registre: seeded clients no longer in the seed file, left enabled",
			slog.Any("client_ids", stale))
	}
	return nil
}
