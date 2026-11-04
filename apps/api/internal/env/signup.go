package env

import (
	"fmt"
	"net/url"
	"strings"

	troncenv "github.com/FacileStudio/tronc/env"
)

// MinAltchaKeyLength is the shortest ALTCHA_HMAC_KEY that turns sign-up on.
// A shorter key is treated as absent: a guessable signing key must switch
// sign-up off, never weaken the proof of work.
const MinAltchaKeyLength = 32

// Signup is public self-service sign-up and passkeys.
type Signup struct {
	// Requested is SIGNUP_ENABLED. It defaults to false so that deploying
	// never opens sign-up by surprise; the owner flips it.
	Requested bool

	// AltchaKey is ALTCHA_HMAC_KEY, the key proof-of-work challenges are
	// signed under. It never leaves the server.
	AltchaKey string

	// RPID and Origins are WEBAUTHN_RP_ID and WEBAUTHN_ORIGINS, defaulting
	// to the issuer's host and origin. Passkeys work whether or not
	// sign-up is open.
	RPID    string
	Origins []string

	// BreachCheck is HIBP_ENABLED, on by default: screening new passwords
	// against Have I Been Pwned.
	BreachCheck bool
}

// Open reports whether sign-up may serve: requested, with a long enough
// key. Whether a mailer exists is decided at wiring.
func (s Signup) Open() bool {
	return s.Requested && len(s.AltchaKey) >= MinAltchaKeyLength
}

// LoadSignup reads the sign-up and passkey environment for issuer. An
// origin that is not https outside development, or not on the relying
// party's domain, fails the boot: a passkey bound to the wrong domain is
// unusable everywhere at once.
func LoadSignup(issuer string, insecure bool) (Signup, error) {
	parsed, err := url.Parse(issuer)
	if err != nil {
		return Signup{}, fmt.Errorf("env: REGISTRE_ISSUER is not a URL: %w", err)
	}
	requested, err := troncenv.Bool("SIGNUP_ENABLED", false)
	if err != nil {
		return Signup{}, err
	}
	breach, err := troncenv.Bool("HIBP_ENABLED", true)
	if err != nil {
		return Signup{}, err
	}
	out := Signup{
		Requested: requested, BreachCheck: breach,
		AltchaKey: troncenv.String("ALTCHA_HMAC_KEY", ""),
		RPID:      strings.TrimSpace(troncenv.String("WEBAUTHN_RP_ID", parsed.Hostname())),
	}
	for _, origin := range strings.Split(troncenv.String("WEBAUTHN_ORIGINS", parsed.Scheme+"://"+parsed.Host), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			out.Origins = append(out.Origins, strings.TrimSuffix(origin, "/"))
		}
	}
	return out, validateOrigins(out.RPID, out.Origins, insecure)
}

// validateOrigins checks each origin is a bare scheme://host on the relying
// party's domain.
func validateOrigins(rpID string, origins []string, insecure bool) error {
	if rpID == "" || len(origins) == 0 {
		return fmt.Errorf("env: WEBAUTHN_RP_ID and WEBAUTHN_ORIGINS must not be empty")
	}
	for _, origin := range origins {
		if err := checkOrigin(rpID, origin, insecure); err != nil {
			return err
		}
	}
	return nil
}

// checkOrigin checks one origin.
func checkOrigin(rpID, origin string, insecure bool) error {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.User != nil {
		return fmt.Errorf("env: WEBAUTHN_ORIGINS entry %q must be scheme://host", origin)
	}
	if parsed.Scheme != "https" && !(insecure && parsed.Scheme == "http") {
		return fmt.Errorf("env: WEBAUTHN_ORIGINS entry %q must be https", origin)
	}
	host := parsed.Hostname()
	if host != rpID && !strings.HasSuffix(host, "."+rpID) {
		return fmt.Errorf("env: WEBAUTHN_ORIGINS entry %q is not on WEBAUTHN_RP_ID %q", origin, rpID)
	}
	return nil
}
