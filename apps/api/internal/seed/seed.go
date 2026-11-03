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

	"github.com/FacileStudio/Registre/apps/api/internal/clients"

	"gopkg.in/yaml.v3"
)

// File is the whole seed document.
type File struct {
	ServiceAccounts []ServiceAccount `yaml:"service_accounts"`

	// Applications declares the clients that log humans in: the suite's
	// web apps, and the CLIs that use the device grant. A web app cannot
	// work without its redirect URIs, so they are required of anything
	// declaring authorization_code — and of nothing else, because a device
	// flow has no redirect to validate.
	Applications []Application `yaml:"applications"`
}

// Application declares one relying party.
type Application struct {
	ClientID  string `yaml:"client_id"`
	Name      string `yaml:"name"`
	SecretEnv string `yaml:"secret_env"`

	// GrantTypes is what this client may ask the token endpoint for, as
	// aliases: authorization_code, refresh_token, device_code. Empty means
	// the browser login and its renewal, which is what every suite app
	// registered before this field existed.
	GrantTypes []string `yaml:"grant_types"`

	// Public marks a client that holds no secret and must therefore name
	// no secret_env. A CLI installed on somebody's laptop is the case:
	// anything shipped inside the binary is readable by whoever has the
	// binary, so calling it a secret would be a story rather than a
	// control. What defends a public client is PKCE on the browser grant
	// and, for the device grant, the fact that a human has to approve each
	// flow by hand.
	Public bool `yaml:"public"`

	// RedirectURIs is the exact-match list. There is no wildcard and no
	// prefix match anywhere downstream: what lands here is byte-compared
	// against what arrives at /authorize.
	RedirectURIs []string `yaml:"redirect_uris"`

	// Scopes defaults to openid profile email. offline_access must be
	// listed explicitly for the app to receive refresh tokens, and roles
	// for it to receive any roles at all — VisibleRoles still filters the
	// claim per app.
	Scopes []string `yaml:"scopes"`

	// BackchannelLogoutURI is where logout tokens are POSTed when a session
	// ends. Unset means the app hears nothing about deactivations.
	BackchannelLogoutURI string `yaml:"backchannel_logout_uri"`

	// DirectorySyncURI is where registre nudges the app after a change to
	// shared spaces: https://<app>/api/registre/sync. Unset means the app
	// only picks changes up on its own pull interval.
	DirectorySyncURI string `yaml:"directory_sync_uri"`

	// VisibleRoles is the whitelist of roles this app may see. Empty means
	// none, even with the roles scope granted: visibility is opt-in.
	VisibleRoles []string `yaml:"visible_roles"`

	// TokenExchange lists the apps this client may trade a signed-in
	// human's token for (RFC 8693), each with the scopes it may ask for
	// there. Empty means the client cannot exchange. Declaring any entry is
	// what grants the exchange grant; it is not listed in grant_types.
	TokenExchange []ExchangeGrant `yaml:"token_exchange"`
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
	for _, application := range file.Applications {
		row, err := application.row(ctx, register)
		if err != nil {
			return err
		}
		if _, err := register.Upsert(ctx, row); err != nil {
			return err
		}
		declared[application.ClientID] = true
		logger.Info("registre: seeded an application", slog.String("client_id", application.ClientID))
	}
	return reportUndeclared(ctx, register, declared, logger)
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
