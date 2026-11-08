package oidc

import (
	"net/http"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// publicCORS is the cross-origin policy of the endpoints an OAuth client
// running in a browser must reach: discovery, authorization server metadata,
// the key set, the token, revocation and registration endpoints.
//
// Any origin, and never credentials. None of these endpoints reads a cookie,
// so a page on another origin learns nothing it could not learn with curl,
// and without credentials no browser will attach registre's session to the
// request in the first place. MCP-Protocol-Version is allowed because MCP
// clients send it on every request, metadata fetches included.
func publicCORS() func(http.Handler) http.Handler {
	return troncmiddleware.CORS(troncmiddleware.CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "MCP-Protocol-Version"},
		ExposedHeaders: []string{"WWW-Authenticate", "Retry-After"},
		MaxAgeSeconds:  3600,
	})
}

// CORS applies publicCORS to the public OAuth paths and fallback to every
// other path. It replaces the router-wide CORS middleware, which runs before
// routing and would answer a preflight to the token endpoint with 403 for
// every origin it does not list.
func CORS(public []string, fallback func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	paths := make(map[string]bool, len(public))
	for _, path := range public {
		paths[path] = true
	}
	open := publicCORS()
	return func(next http.Handler) http.Handler {
		openNext, guardedNext := open(next), fallback(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if paths[r.URL.Path] {
				openNext.ServeHTTP(w, r)
				return
			}
			guardedNext.ServeHTTP(w, r)
		})
	}
}
