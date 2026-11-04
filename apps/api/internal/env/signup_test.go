package env

import "testing"

// TestSignupDefaultsAreClosedAndBoundToTheIssuer.
func TestSignupDefaultsAreClosedAndBoundToTheIssuer(t *testing.T) {
	t.Setenv("SIGNUP_ENABLED", "")
	t.Setenv("ALTCHA_HMAC_KEY", "")
	got, err := LoadSignup("https://sso.facile.studio", false)
	if err != nil {
		t.Fatalf("LoadSignup: %v", err)
	}
	if got.Open() || got.RPID != "sso.facile.studio" || len(got.Origins) != 1 ||
		got.Origins[0] != "https://sso.facile.studio" || !got.BreachCheck {
		t.Fatalf("defaults = %+v", got)
	}
	t.Setenv("SIGNUP_ENABLED", "true")
	t.Setenv("ALTCHA_HMAC_KEY", "too-short")
	if got, _ := LoadSignup("https://sso.facile.studio", false); got.Open() {
		t.Fatal("a short ALTCHA key opened sign-up")
	}
	t.Setenv("ALTCHA_HMAC_KEY", "0123456789abcdef0123456789abcdef")
	if got, _ := LoadSignup("https://sso.facile.studio", false); !got.Open() {
		t.Fatal("a valid key and SIGNUP_ENABLED did not open sign-up")
	}
}

// TestOriginsAreStrict refuses http, paths and foreign hosts.
func TestOriginsAreStrict(t *testing.T) {
	for _, origins := range []string{"http://sso.facile.studio", "https://evil.example", "https://sso.facile.studio/path"} {
		t.Setenv("WEBAUTHN_ORIGINS", origins)
		if _, err := LoadSignup("https://sso.facile.studio", false); err == nil {
			t.Errorf("WEBAUTHN_ORIGINS=%s was accepted", origins)
		}
	}
}
