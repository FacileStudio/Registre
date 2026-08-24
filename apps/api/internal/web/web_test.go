package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mustHandler builds the handler or fails the test. Handler now returns an
// error so an unreadable build refuses the boot rather than answering 500.
func mustHandler(t *testing.T) http.Handler {
	t.Helper()
	handler, err := Handler()
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}
	return handler
}

// clientRoutes is every path the SvelteKit build owns. None of them is a file
// in the build, so each one only works if the catch-all answers it with the
// index document.
var clientRoutes = []string{
	"/login",
	"/account",
	"/account/security",
	"/admin",
	"/admin/clients",
	"/admin/invitations",
	"/admin/keys",
	"/invites/redeem",
	"/recover",
	"/recover/reset",
}

// TestEveryClientRouteAnswersWithTheApp is the regression that matters here.
//
// The first version of this handler rewrote an unknown path to /index.html and
// handed it to http.FileServerFS, which canonicalises that name by redirecting
// to "./". Every deep link therefore answered 301 to the origin root and threw
// the route away, so the login page was unreachable by URL and the redirect
// back from an OIDC flow landed on the marketing page. A status assertion is
// the whole test: 200 and HTML, never a redirect.
func TestEveryClientRouteAnswersWithTheApp(t *testing.T) {
	handler := mustHandler(t)
	for _, route := range clientRoutes {
		t.Run(route, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("%s answered %d, want 200: a redirect here discards the route",
					route, recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), "<!doctype html>") {
				t.Errorf("%s did not answer with the index document", route)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("%s cached the index as %q: a stale copy names chunks that are gone", route, got)
			}
		})
	}
}

// TestARealAssetIsServedAsItself checks the fallback has not swallowed the
// build. An index document returned in place of a stylesheet is a page that
// renders unstyled rather than one that fails.
func TestARealAssetIsServedAsItself(t *testing.T) {
	recorder := httptest.NewRecorder()
	mustHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/favicon.svg", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("favicon.svg answered %d, want 200", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "<!doctype html>") {
		t.Error("favicon.svg was answered with the index document")
	}
}

// TestAnUnroutedAPIPathIs404JSON stops the catch-all from answering a missing
// endpoint with a page. A caller parsing JSON reads 200-and-HTML as a
// malformed success, which is how Journal lost reports in silence.
func TestAnUnroutedAPIPathIs404JSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	mustHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("answered %d, want 404", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("content type %q, want JSON", got)
	}
}

// TestABareAPIPrefixIsAlsoJSON covers the path a caller hits by mistake, which
// used to fall through to the index and answer 200 with a page.
func TestABareAPIPrefixIsAlsoJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	mustHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("answered %d, want 404", recorder.Code)
	}
}

// TestUnhashedAssetsAreNotCached keeps _app/version.json fresh. It is what
// SvelteKit polls to notice a deploy, so a heuristically cached copy leaves a
// tab convinced it is current forever.
func TestUnhashedAssetsAreNotCached(t *testing.T) {
	recorder := httptest.NewRecorder()
	mustHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/favicon.svg", nil))

	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("favicon.svg cache header %q, want no-cache", got)
	}
}

// TestImmutableAssetsAreCachedForever checks the one directory whose names
// carry a content hash gets the caching that makes a repeat visit free.
func TestImmutableAssetsAreCachedForever(t *testing.T) {
	name, ok := anyImmutableAsset(t)
	if !ok {
		t.Skip("the build has no hashed assets to check")
	}
	recorder := httptest.NewRecorder()
	mustHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/"+name, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("%s answered %d, want 200", name, recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != immutableCache {
		t.Errorf("%s cache header %q, want %q", name, got, immutableCache)
	}
}

// anyImmutableAsset finds one build-hashed file to assert caching against,
// rather than naming a hash that changes with every build.
func anyImmutableAsset(t *testing.T) (string, bool) {
	t.Helper()
	entries, err := dist.ReadDir("dist/" + immutablePrefix + "chunks")
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return immutablePrefix + "chunks/" + entry.Name(), true
		}
	}
	return "", false
}
