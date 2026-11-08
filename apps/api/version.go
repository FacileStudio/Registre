package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// version is the release this binary was built from. The Dockerfile links it
// in from the top released heading of CHANGELOG.md; a plain `go build` or
// `go test` reports "dev".
var version = "dev"

// versionHandler answers the build's version as {"version":"vX.Y.Z"}. It is
// public on purpose: it reads no state and names nothing but the release.
func versionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
}

func mountVersion(router chi.Router) {
	router.Get("/version", versionHandler)
	router.Get("/api/version", versionHandler)
}
