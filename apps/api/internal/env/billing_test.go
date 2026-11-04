package env

import (
	"strings"
	"testing"
)

// TestTheBillingURLIsFixedAndTheTokenLong keeps the bearer off plain http and
// out of a URL, and makes a short token switch the plan section off.
func TestTheBillingURLIsFixedAndTheTokenLong(t *testing.T) {
	for raw, insecure := range map[string]bool{
		"http://api-ardoise.facile.studio":            false,
		"https://user:pass@api-ardoise.facile.studio": false,
		"https://api-ardoise.facile.studio?x=1":       false,
		"api-ardoise.facile.studio":                   false,
	} {
		t.Setenv("ARDOISE_BILLING_URL", raw)
		_, err := LoadBilling(insecure)
		if err == nil {
			t.Errorf("%q was accepted", raw)
		} else if strings.Contains(err.Error(), "pass") {
			t.Errorf("the error for %q leaks the URL's credentials: %v", raw, err)
		}
	}
	t.Setenv("ARDOISE_BILLING_URL", "https://api-ardoise.facile.studio/")
	t.Setenv("BILLING_API_TOKEN", strings.Repeat("x", MinBillingTokenLength-1))
	cfg, err := LoadBilling(false)
	if err != nil || cfg.ArdoiseURL != "https://api-ardoise.facile.studio" || cfg.PlansEnabled() {
		t.Fatalf("short token: cfg = %+v, err = %v; want a trimmed URL and the section off", cfg.ArdoiseURL, err)
	}
	t.Setenv("BILLING_API_TOKEN", strings.Repeat("x", MinBillingTokenLength))
	if cfg, _ := LoadBilling(false); !cfg.PlansEnabled() {
		t.Error("a full-length token did not turn the plan section on")
	}
}
