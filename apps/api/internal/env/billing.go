package env

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	troncenv "github.com/FacileStudio/tronc/env"
)

// MinBillingTokenLength is the shortest BILLING_API_TOKEN that turns the plan
// section on. A shorter one is treated as absent: a guessable shared secret
// must disable the feature, never weaken it.
const MinBillingTokenLength = 32

// Billing is how registre talks to Ardoise, the suite's billing app. Each
// half is off by default and independent of the other.
type Billing struct {
	// EntitlementsWriter is the one client id allowed to PUT
	// /api/directory/entitlements, Ardoise's. Empty disables the route.
	EntitlementsWriter string

	// ArdoiseURL is the fixed base URL of Ardoise's billing API, such as
	// https://api-ardoise.facile.studio. It is the only host the plan
	// section ever calls.
	ArdoiseURL string

	// Token is the bearer registre presents to Ardoise. It never leaves the
	// server and is never logged.
	Token string
}

// PlansEnabled reports whether the spaces pages show the plan section: both
// the URL and a long enough token are set.
func (b Billing) PlansEnabled() bool {
	return b.ArdoiseURL != "" && len(b.Token) >= MinBillingTokenLength
}

// LoadAll is Load followed by LoadBilling, so the process reads its whole
// environment in one call. The Config is returned even when billing fails,
// so the caller can still build its logger to report why.
func LoadAll() (Config, Billing, error) {
	cfg, err := Load()
	if err != nil {
		return cfg, Billing{}, err
	}
	plans, err := LoadBilling(cfg.InsecureIssuer())
	return cfg, plans, err
}

// LoadBilling reads ENTITLEMENTS_WRITER, ARDOISE_BILLING_URL and
// BILLING_API_TOKEN. A malformed URL fails the boot, and so does plain http
// outside development, because the bearer would cross the network in clear.
func LoadBilling(insecure bool) (Billing, error) {
	out := Billing{
		EntitlementsWriter: strings.TrimSpace(troncenv.String("ENTITLEMENTS_WRITER", "")),
		ArdoiseURL:         strings.TrimSuffix(strings.TrimSpace(troncenv.String("ARDOISE_BILLING_URL", "")), "/"),
		Token:              troncenv.String("BILLING_API_TOKEN", ""),
	}
	if out.ArdoiseURL == "" {
		return out, nil
	}
	return out, validateBillingURL(out.ArdoiseURL, insecure)
}

// validateBillingURL refuses anything but a bare scheme://host[:port][/path].
func validateBillingURL(raw string, insecure bool) error {
	parsed, err := url.Parse(raw)
	switch {
	case err != nil:
		return errors.New("ARDOISE_BILLING_URL is not a URL")
	case parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "":
		return errors.New("ARDOISE_BILLING_URL must be scheme://host with no credentials, query or fragment")
	case parsed.Scheme == "https", parsed.Scheme == "http" && insecure:
		return nil
	default:
		return fmt.Errorf("ARDOISE_BILLING_URL must be https outside development, got scheme %q", parsed.Scheme)
	}
}
