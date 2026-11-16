package oidc

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestAuthorizeHandsUILocalesToTheLoginPage sends the login page the first
// supported language of ui_locales as a display hint, and nothing when the
// app named none or only languages Registre does not speak.
func TestAuthorizeHandsUILocalesToTheLoginPage(t *testing.T) {
	f := newFixture(t, defaultKeyConfig())
	f.application("sablier", "http://127.0.0.1:9999/callback")
	_, challenge := pkcePair(t)
	for asked, want := range map[string]string{
		"":                    "",
		"fr":                  "ui_locales=fr",
		"it de-CH fr":         "ui_locales=de",
		"it":                  "",
		"fr&next=//x.example": "",
	} {
		extra := url.Values{"scope": {"openid"}}
		if asked != "" {
			extra.Set("ui_locales", asked)
		}
		got := (&singleCookie{}).getNoFollow(t, f.issuer+"/authorize?"+flowParams(challenge, extra).Encode())
		location := got.Header.Get("Location")
		if got.StatusCode != http.StatusFound || !strings.HasPrefix(location, loginPath) {
			t.Fatalf("ui_locales %q: %d to %q, want the login redirect", asked, got.StatusCode, location)
		}
		if _, hint, _ := strings.Cut(location, "&"); hint != want {
			t.Errorf("ui_locales %q: the login redirect carries %q, want %q", asked, hint, want)
		}
	}
}
