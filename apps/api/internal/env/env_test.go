package env

import (
	"strings"
	"testing"
)

// base sets the three variables every load needs, so each test can change one
// thing and read the result.
func base(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/registre?sslmode=disable")
	t.Setenv("REGISTRE_ISSUER", "https://sso.facile.studio")
	t.Setenv("ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("APP_ENV", "production")
}

// TestARetentionShorterThanATokenIsRefusedAtBoot is the outage SPEC §7 names,
// caught by arithmetic instead of by eleven apps at once: a retired key that
// leaves the JWKS while tokens it signed are still valid 401s every one of
// them, and nothing about the configuration looks wrong when it happens.
func TestARetentionShorterThanATokenIsRefusedAtBoot(t *testing.T) {
	base(t)
	t.Setenv("REGISTRE_ACCESS_TOKEN_TTL", "2h")
	t.Setenv("REGISTRE_KEY_RETENTION", "1h")

	_, err := Load()
	if err == nil {
		t.Fatal("a retention shorter than the token lifetime was accepted")
	}
	if !strings.Contains(err.Error(), "REGISTRE_KEY_RETENTION") {
		t.Errorf("the error does not name the variable to fix: %v", err)
	}
}

// TestAPublishDelayLongerThanTheTermIsRefused catches the other end of the
// same lifecycle: a delay that outlasts the rotation interval means the next
// key is published before the current one has begun signing, and the state
// machine never settles.
func TestAPublishDelayLongerThanTheTermIsRefused(t *testing.T) {
	base(t)
	t.Setenv("REGISTRE_KEY_ROTATION_INTERVAL", "1h")
	t.Setenv("REGISTRE_KEY_PUBLISH_DELAY", "2h")

	if _, err := Load(); err == nil {
		t.Fatal("a publish delay longer than the rotation interval was accepted")
	}
}

func TestTheIssuerMustBeHTTPSOutsideDevelopment(t *testing.T) {
	base(t)
	t.Setenv("REGISTRE_ISSUER", "http://sso.facile.studio")

	if _, err := Load(); err == nil {
		t.Fatal("a plain-http issuer was accepted in production")
	}

	t.Setenv("APP_ENV", "development")
	if _, err := Load(); err != nil {
		t.Fatalf("a plain-http issuer was refused in development: %v", err)
	}
}

func TestTheIssuerIsRefusedWithAQueryOrFragment(t *testing.T) {
	for name, issuer := range map[string]string{
		"a query":    "https://sso.facile.studio?tenant=1",
		"a fragment": "https://sso.facile.studio#here",
		"no host":    "sso.facile.studio",
	} {
		t.Run(name, func(t *testing.T) {
			base(t)
			t.Setenv("REGISTRE_ISSUER", issuer)
			if _, err := Load(); err == nil {
				t.Fatalf("%s was accepted as an issuer", name)
			}
		})
	}
}

// TestTheDefaultsAddUp guards the relationship rather than the numbers: a
// deployment that sets nothing must not be one the guards above would refuse.
func TestTheDefaultsAddUp(t *testing.T) {
	base(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("the defaults do not load: %v", err)
	}
	if cfg.KeyRetention < cfg.AccessTokenTTL {
		t.Errorf("default retention %s is shorter than the default token %s", cfg.KeyRetention, cfg.AccessTokenTTL)
	}
	if cfg.PublishDelay >= cfg.RotationEvery {
		t.Errorf("default delay %s is not shorter than the default term %s", cfg.PublishDelay, cfg.RotationEvery)
	}
	if cfg.InsecureIssuer() {
		t.Error("a production configuration tolerates a plain-http issuer")
	}
}
