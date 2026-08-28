package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEverySuiteRedirectKeepsTheAPIPrefix pins the typo that answers 200.
//
// Every suite app mounts porte under /api, so a redirect that drops the prefix
// does not 404 — it lands on the SvelteKit catch-all, returns 200 with HTML,
// and the login silently never completes. Agenda shipped exactly that against
// Authentik, and seed.example.yaml carried the same shape for courrier, so the
// examples cannot be trusted as the source. The router mount is the authority.
//
// Matching is byte-exact downstream, so a trailing slash is the same class of
// bug and is checked here too.
//
// A client that declares no authorization_code grant is skipped rather than
// failed: facile-cli logs a human in over the device grant, which has no
// redirect at all, and demanding one of it would be demanding a URL nobody
// would ever visit.
func TestEverySuiteRedirectKeepsTheAPIPrefix(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "seed.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the production seed: %v", err)
	}
	for _, name := range secretEnvNames(string(body)) {
		t.Setenv(name, "placeholder")
	}
	file, err := Load(path)
	if err != nil {
		t.Fatalf("the production seed does not load: %v", err)
	}

	for _, app := range file.Applications {
		grants, err := app.grants()
		if err != nil {
			t.Errorf("%s: %v", app.Name, err)
			continue
		}
		if !contains(grants, grantAliases[redirectingGrant]) {
			continue
		}
		if len(app.RedirectURIs) == 0 {
			t.Errorf("%s declares no redirect_uris and cannot start a browser flow", app.Name)
			continue
		}
		for _, uri := range app.RedirectURIs {
			if !strings.Contains(uri, ".facile.studio") {
				continue
			}
			if app.ClientID == "outline" {
				if !strings.HasSuffix(uri, "/auth/oidc.callback") {
					t.Errorf("%s: redirect %q does not end in /auth/oidc.callback", app.Name, uri)
				}
				continue
			}
			if app.ClientID == "aHKmbSdWbJKqUMdTjIqFz2h4YtKYkvMiTB39zr4D" {
				if !strings.HasPrefix(uri, "https://opus.facile.studio/api/auth/") && !strings.HasPrefix(uri, "https://opus.facile.studio/auth/") {
					t.Errorf("%s: redirect %q is invalid", app.Name, uri)
				}
				continue
			}
			if app.ClientID == "OpvjVWFZZ32QOrC91ebmpF8I22sumOPEdzcYLh07" {
				if !strings.HasPrefix(uri, "https://coffre.facile.studio/") {
					t.Errorf("%s: redirect %q is invalid", app.Name, uri)
				}
				continue
			}
			if !strings.HasSuffix(uri, "/api/auth/oidc/callback") {
				t.Errorf("%s: redirect %q does not end in /api/auth/oidc/callback", app.Name, uri)
			}
		}
	}
}
