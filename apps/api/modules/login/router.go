// Package login is registre's login UI: the pages a human sees between the
// app that sent them here and the code that sends them back.
//
// It owns HTTP because it is HTTP — SPEC §5's rule is that anything under
// modules/ declares its routes in router.go. The simple version is
// server-rendered HTML with no JavaScript; apps/client grows around it later,
// and until then this is the whole face of the suite's identity provider.
package login

import (
	"github.com/go-chi/chi/v5"
)

// Mount registers the login routes on router.
func Mount(router chi.Router, service *Service) {
	router.Get("/", service.home)
	router.Get("/login", service.form)
	router.Post("/login", service.submit)
	router.Post("/api/login", service.jsonSubmit)
}
