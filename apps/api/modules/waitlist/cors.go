package waitlist

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// Prefix is where the waitlist routes live.
const Prefix = "/api/waitlist"

// CORS applies the waitlist's cross-origin policy under Prefix and fallback
// everywhere else. The policy lists the landing origins, allows only GET and
// POST with a JSON body, and never credentials, so a landing page can reach
// the waitlist and nothing else of registre's.
func CORS(origins []string, fallback func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	policy := troncmiddleware.CORS(troncmiddleware.CORSConfig{
		AllowedOrigins: origins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Accept", "Content-Type"},
		ExposedHeaders: []string{"Retry-After"},
		MaxAgeSeconds:  3600,
	})
	return func(next http.Handler) http.Handler {
		waitlistNext, otherNext := policy(next), fallback(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == Prefix || strings.HasPrefix(r.URL.Path, Prefix+"/") {
				waitlistNext.ServeHTTP(w, r)
				return
			}
			otherNext.ServeHTTP(w, r)
		})
	}
}

// allowed reports whether a request may reach the routes: no Origin (a mail
// client's one-click POST, curl), registre's own pages, or a listed landing.
// CORS alone only stops a browser reading the answer; this stops the write.
func (s *Service) allowed(origin, host string) bool {
	if origin == "" || slices.Contains(s.Origins, origin) {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host == host
}
