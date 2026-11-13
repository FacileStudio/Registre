// Package signup is the HTTP half of public self-service sign-up, "Create
// your Facile account", over internal/signup.
//
// Every route answers 404 while sign-up is off (SIGNUP_ENABLED unset, or no
// valid ALTCHA_HMAC_KEY, or no mailer), so deploying never opens it by
// surprise. Every route sits behind a per-address budget, and start behind a
// per-email one counted after the proof of work.
package signup

import (
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/limits"
	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"

	"github.com/go-chi/chi/v5"
)

// Budgets. Start costs a mail, so it gets one per address every five seconds
// with a burst of five; the per-email budget is internal/signup's
// MaxLinksPerHour, counted only after a valid proof of work. The rest share
// the login endpoint's arithmetic.
const (
	StartLimit = 0.2
	StartBurst = 5
	StepLimit  = 5.0
	StepBurst  = 10
)

// Deps is what the routes are built over. Flow nil means sign-up is off.
type Deps struct {
	Flow     *flow.Service
	Users    *identity.Service
	Sessions *accounts.Service
	Cookies  accounts.Cookies
	Audit    *audit.Service
}

// Service serves the sign-up routes.
type Service struct {
	Deps
	logger *slog.Logger
}

// New builds the routes' service.
func New(deps Deps, logger *slog.Logger) *Service {
	return &Service{Deps: deps, logger: logger}
}

// Limits overrides the budgets; tests use it to reach a limiter quickly.
type Limits struct {
	StartLimit, StepLimit float64
	StartBurst, StepBurst int
}

// DefaultLimits are the production budgets.
func DefaultLimits() Limits {
	return Limits{
		StartLimit: StartLimit, StartBurst: StartBurst, StepLimit: StepLimit, StepBurst: StepBurst,
	}
}

// Mount registers the sign-up routes with the production budgets.
func Mount(router chi.Router, service *Service) {
	MountWithLimits(router, service, DefaultLimits())
}

// MountWithLimits registers the routes with explicit budgets.
func MountWithLimits(router chi.Router, service *Service, budget Limits) {
	steps := limits.PerAddress(budget.StepLimit, budget.StepBurst)
	router.Route("/api/signup", func(routes chi.Router) {
		routes.Use(service.open)
		routes.With(steps).Get("/challenge", service.challenge)
		routes.With(limits.PerAddress(budget.StartLimit, budget.StartBurst)).Post("/start", service.start)
		routes.With(steps).Post("/verify", service.verify)
		routes.With(steps, accounts.NewGuard(nil, accounts.Cookies{}).CSRF).Post("/finish", service.finish)
	})
}
