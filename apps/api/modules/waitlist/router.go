// Package waitlist is the HTTP half of the product waitlists, over
// internal/waitlist: the Scribe and Pastille landing pages post an address
// here from their own origins.
//
// Every route answers 404 while the waitlist is off (no valid
// ALTCHA_HMAC_KEY), so deploying never opens it by surprise. Joining costs a
// proof of work and sits behind the sign-up start budget per address; the
// other routes share the login endpoint's arithmetic. Cross-origin access is
// limited to the landing origins, without credentials: nothing here reads a
// cookie.
package waitlist

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/limits"
	"github.com/FacileStudio/Registre/apps/api/internal/waitlist"

	altcha "github.com/altcha-org/altcha-lib-go"
	"github.com/go-chi/chi/v5"
)

// Budgets. Join gets one request per address every five seconds with a
// burst of five, like a sign-up start; the challenge and unsubscribe share
// the login endpoint's arithmetic.
const (
	JoinLimit = 0.2
	JoinBurst = 5
	StepLimit = 5.0
	StepBurst = 10
)

// DefaultOrigins are the landing pages allowed to call the waitlist from a
// browser.
func DefaultOrigins() []string {
	return []string{"https://scribe.facile.studio", "https://pastille.facile.studio"}
}

// Prover mints and spends ALTCHA proofs of work. *signup.Service is the one
// in use, so sign-up and the waitlist share one key and one replay table.
type Prover interface {
	Challenge() (altcha.Challenge, error)
	Spend(ctx context.Context, proof string) error
}

// Service serves the waitlist routes. A nil List means the waitlist is off.
type Service struct {
	List    *waitlist.Service
	Proofs  Prover
	Origins []string
	logger  *slog.Logger
}

// New builds the routes' service over the allowed browser origins.
func New(list *waitlist.Service, proofs Prover, origins []string, logger *slog.Logger) *Service {
	return &Service{List: list, Proofs: proofs, Origins: origins, logger: logger}
}

// Limits overrides the budgets; tests use it to reach a limiter quickly.
type Limits struct {
	JoinLimit, StepLimit float64
	JoinBurst, StepBurst int
}

// DefaultLimits are the production budgets.
func DefaultLimits() Limits {
	return Limits{JoinLimit: JoinLimit, JoinBurst: JoinBurst, StepLimit: StepLimit, StepBurst: StepBurst}
}

// Mount registers the waitlist routes with the production budgets.
func Mount(router chi.Router, service *Service) {
	MountWithLimits(router, service, DefaultLimits())
}

// MountWithLimits registers the routes with explicit budgets.
func MountWithLimits(router chi.Router, service *Service, budget Limits) {
	steps := limits.PerAddress(budget.StepLimit, budget.StepBurst)
	router.Route(Prefix, func(routes chi.Router) {
		routes.Use(service.open)
		routes.With(steps).Get("/challenge", service.challenge)
		routes.With(limits.PerAddress(budget.JoinLimit, budget.JoinBurst)).Post("/", service.join)
		routes.With(steps).Post("/unsubscribe", service.unsubscribe)
	})
}

// open answers 404 for every route while the waitlist is off, refuses a
// browser on any other origin, and caps the body.
func (s *Service) open(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.List == nil || s.Proofs == nil {
			encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
			return
		}
		if !s.allowed(r.Header.Get("Origin"), r.Host) {
			encode(w, map[string]string{"error": "origin_not_allowed"}, http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next.ServeHTTP(w, r)
	})
}
