package seed

import (
	"os"
	"path/filepath"
	"slices"
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
//
// Glouton is excepted because the premise does not hold for it. The /api rule
// comes from the Go family mounting porte inside router.Route("/api", ...);
// Glouton is a TypeScript monorepo whose Hono backend serves the flow itself
// and defaults its redirect to DOMAIN + /auth/oidc/callback, in
// apps/backend/src/services/oidc.ts. Its seed entry matches the router that
// actually answers, so the seed is right and the blanket suffix check was
// wrong. Docs and Opus are excepted above for the same reason: the
// mount is the authority, not the family.
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
			if app.ClientID == "1IdqK3bA2LWI-zuWVLqFpnIlCTVu117F" {
				if !strings.HasSuffix(uri, "/auth/oidc/callback") {
					t.Errorf("%s: redirect %q does not end in /auth/oidc/callback", app.Name, uri)
				}
				continue
			}
			if !strings.HasSuffix(uri, "/api/auth/oidc/callback") {
				t.Errorf("%s: redirect %q does not end in /api/auth/oidc/callback", app.Name, uri)
			}
		}
	}
}

// TestTheProductionSeedLetsCourrierAndEchoReachBulle pins both grants to
// exactly bulle.read and bulle.write, so neither can widen or vanish unseen.
func TestTheProductionSeedLetsCourrierAndEchoReachBulle(t *testing.T) {
	file := loadProductionSeed(t)
	want := []ExchangeGrant{{Audience: "bulle", Scopes: []string{"bulle.read", "bulle.write"}}}
	for name, clientID := range map[string]string{
		"courrier": "tniN7PWx8mSOCwVAAvYpBexYnMFWAM5flt19rNl0",
		"echo":     "echo",
	} {
		i := slices.IndexFunc(file.Applications, func(a Application) bool { return a.ClientID == clientID })
		if i < 0 {
			t.Errorf("no %s client in the production seed", name)
			continue
		}
		if got := file.Applications[i].TokenExchange; !slices.EqualFunc(got, want, func(a, b ExchangeGrant) bool {
			return a.Audience == b.Audience && slices.Equal(a.Scopes, b.Scopes)
		}) {
			t.Errorf("%s exchanges %+v, want %+v", name, got, want)
		}
	}
}
