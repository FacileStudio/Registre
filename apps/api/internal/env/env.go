// Package env reads registre's process configuration.
package env

import (
	"fmt"
	"net/url"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"

	troncenv "github.com/FacileStudio/tronc/env"
)

// Defaults for the token and key lifecycle. They are conservative on purpose:
// a short token limits what a leaked one buys, and a long publish delay costs
// nothing but the wait before a new key starts signing.
const (
	DefaultAccessTokenTTL  = time.Hour
	DefaultRefreshTokenTTL = 30 * 24 * time.Hour
	DefaultRotation        = 30 * 24 * time.Hour
	DefaultPublishDelay    = 15 * time.Minute
)

// Config is the process configuration: the tronc core plus what an identity
// provider needs that no other app does.
type Config struct {
	troncenv.Core

	// Issuer is the URL registre answers as. Every app in the suite has it
	// as OIDC_ISSUER, and it is baked into every token, so changing it
	// invalidates everything already issued.
	Issuer string

	// EncryptionKey is the base64 root key the signing keys are sealed
	// under at rest.
	EncryptionKey string

	AccessTokenTTL time.Duration

	// RefreshTokenTTL is how long a refresh grant may be redeemed.
	RefreshTokenTTL time.Duration

	RotationEvery time.Duration

	// PublishDelay is how long a new key sits in the JWKS before it signs
	// anything. It has to outlast the key set cache of the slowest relying
	// party, or that party 401s every token until it refetches.
	PublishDelay time.Duration

	// KeyRetention is how long a retired public key stays in the JWKS after
	// it stops signing. Below one access token lifetime it would drop a key
	// while tokens it signed are still valid, so Load refuses that.
	KeyRetention time.Duration

	// SeedFile is the YAML the register bootstraps from. Unset means the
	// database is the only source of clients.
	SeedFile string

	// VerifyConcurrency is how many argon2 verifications run at once. It is
	// a memory ceiling wearing a count: multiply it by 64 MiB to get the
	// transient memory the token endpoint may hold under load.
	VerifyConcurrency int
}

// InsecureIssuer reports whether a plain-http issuer is tolerated. Only a
// development deployment gets one: an https issuer is what makes the token
// endpoint's client secret confidential in transit, and a boolean of its own
// would be a knob someone eventually sets in production.
func (c Config) InsecureIssuer() bool { return c.AppEnv == troncenv.Development }

// Load reads and validates the environment. Every check that can be made
// without a database is made here, at boot, rather than at the first token
// request: an identity provider that starts and then cannot sign is worse than
// one that refuses to start.
func Load() (Config, error) {
	core, err := troncenv.LoadCore()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Core: core}

	if cfg.Issuer, err = troncenv.Required("REGISTRE_ISSUER"); err != nil {
		return Config{}, err
	}
	if cfg.EncryptionKey, err = troncenv.Required("ENCRYPTION_KEY"); err != nil {
		return Config{}, err
	}
	if err := cfg.loadDurations(); err != nil {
		return Config{}, err
	}
	cfg.SeedFile = troncenv.String("REGISTRE_SEED_FILE", "")

	cfg.VerifyConcurrency, err = troncenv.Int("REGISTRE_VERIFY_CONCURRENCY", credentials.DefaultConcurrency())
	if err != nil {
		return Config{}, err
	}
	if cfg.VerifyConcurrency < 1 {
		return Config{}, fmt.Errorf(
			"REGISTRE_VERIFY_CONCURRENCY must be at least 1, got %d: "+
				"a ceiling of zero blocks every login rather than limiting it",
			cfg.VerifyConcurrency)
	}

	if err := validateIssuer(cfg.Issuer, cfg.InsecureIssuer()); err != nil {
		return Config{}, err
	}
	if cfg.PublishDelay >= cfg.RotationEvery {
		return Config{}, fmt.Errorf(
			"REGISTRE_KEY_PUBLISH_DELAY (%s) is not shorter than REGISTRE_KEY_ROTATION_INTERVAL (%s): "+
				"the next key would be published before the current one had started signing",
			cfg.PublishDelay, cfg.RotationEvery)
	}
	if cfg.KeyRetention < cfg.AccessTokenTTL {
		return Config{}, fmt.Errorf(
			"REGISTRE_KEY_RETENTION (%s) is shorter than REGISTRE_ACCESS_TOKEN_TTL (%s): "+
				"a retired key would leave the JWKS while tokens it signed are still valid",
			cfg.KeyRetention, cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL <= cfg.AccessTokenTTL {
		return Config{}, fmt.Errorf(
			"REGISTRE_REFRESH_TOKEN_TTL (%s) is not longer than REGISTRE_ACCESS_TOKEN_TTL (%s): "+
				"a grant that dies before the thing it renews is a login that breaks for no reason",
			cfg.RefreshTokenTTL, cfg.AccessTokenTTL)
	}
	return cfg, nil
}

func (c *Config) loadDurations() error {
	var err error
	if c.AccessTokenTTL, err = troncenv.Duration("REGISTRE_ACCESS_TOKEN_TTL", DefaultAccessTokenTTL); err != nil {
		return err
	}
	if c.RefreshTokenTTL, err = troncenv.Duration("REGISTRE_REFRESH_TOKEN_TTL", DefaultRefreshTokenTTL); err != nil {
		return err
	}
	if c.RotationEvery, err = troncenv.Duration("REGISTRE_KEY_ROTATION_INTERVAL", DefaultRotation); err != nil {
		return err
	}
	if c.PublishDelay, err = troncenv.Duration("REGISTRE_KEY_PUBLISH_DELAY", DefaultPublishDelay); err != nil {
		return err
	}
	c.KeyRetention, err = troncenv.Duration("REGISTRE_KEY_RETENTION", 2*c.AccessTokenTTL)
	return err
}

func validateIssuer(issuer string, insecure bool) error {
	parsed, err := url.Parse(issuer)
	if err != nil {
		return fmt.Errorf("REGISTRE_ISSUER is not a URL: %w", err)
	}
	if parsed.Host == "" {
		return fmt.Errorf("REGISTRE_ISSUER needs a host, got %q", issuer)
	}
	if parsed.Fragment != "" || parsed.RawQuery != "" {
		return fmt.Errorf("REGISTRE_ISSUER must carry no query or fragment, got %q", issuer)
	}
	if parsed.Scheme != "https" && !insecure {
		return fmt.Errorf("REGISTRE_ISSUER must be https outside development, got %q", issuer)
	}
	return nil
}
