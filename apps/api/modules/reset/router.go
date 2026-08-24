// Package reset is the HTTP half of password recovery: asking for a reset
// link, and spending one.
//
// It is a module of its own rather than two more routes on modules/login
// because the two are reached in opposite states. Login is for somebody who
// knows their password; this is for somebody who does not, and neither
// endpoint here may ever learn anything from the other's answers.
//
// Both routes are unauthenticated and both are rate-limited on the same
// per-address budget as the login endpoint, because both spend an argon2
// hash per call and an endpoint that hashes without a budget is the endpoint
// a flood picks.
package reset

import (
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/recovery"

	"github.com/go-chi/chi/v5"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// DefaultLimit and DefaultBurst are the per-address budget both routes get.
// They match modules/login's, which is the arithmetic a guesser faces
// everywhere else in this binary.
const (
	DefaultLimit = 5.0
	DefaultBurst = 10
)

// Service serves the recovery endpoints over internal/recovery.
type Service struct {
	resets *recovery.Service
	logger *slog.Logger
}

// New builds the service over the reset service. A nil resets service means
// recovery is not configured, and every route answers as though it were
// while doing nothing, so an instance with no SMTP does not become an
// address oracle by returning a different error.
func New(resets *recovery.Service, logger *slog.Logger) *Service {
	return &Service{resets: resets, logger: logger}
}

// Mount registers the recovery routes on router.
func Mount(router chi.Router, service *Service) {
	MountWithLimits(router, service, DefaultLimit, DefaultBurst)
}

// MountWithLimits registers the routes with an explicit per-address budget.
// Tests use it to reach the limiter without hammering the real default.
func MountWithLimits(router chi.Router, service *Service, limit float64, burst int) {
	router.Group(func(routes chi.Router) {
		routes.Use(troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{
			Limit: limit, Burst: burst,
		}))
		routes.Post("/api/recover", service.request)
		routes.Post("/api/recover/reset", service.redeem)
	})
}
