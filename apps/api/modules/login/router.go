// Package login is registre's login backend: it checks credentials for the
// client app, which owns every page a human sees.
//
// It owns HTTP because it is HTTP — SPEC §5's rule is that anything under
// modules/ declares its routes in router.go. The pages themselves live in
// apps/client and reach this package through POST /api/login.
package login

import (
	"github.com/go-chi/chi/v5"
)

// Mount registers the login routes on router. The client build owns every
// page, including "/"; this package only answers its credential check.
func Mount(router chi.Router, service *Service) {
	router.Post("/api/login", service.jsonSubmit)
}
