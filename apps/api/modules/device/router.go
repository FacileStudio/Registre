// Package device is the verification page's API: the three routes a human
// uses to see what a CLI is asking for and to say yes or no.
//
// RFC 8628 leaves this half entirely to the implementation. zitadel/oidc polls
// a state and defines no way to set one, so the flow only completes because
// something outside the library writes the answer — these routes, over
// internal/devices, are that something.
//
// Every route that touches a flow is behind the account cookie, which is
// deliberate and is the whole security model: a device flow is approved by a
// named human, and the token that follows carries that human's subject, roles
// and authentication time. An anonymous approval would mint a credential nobody
// is accountable for. It also means a guesser who lands on a live user code has
// still proved nothing, because they cannot answer it.
//
// The exception is the route that hands a human a session in the first place.
// /api/device/authenticate takes a browser through registre's own /authorize,
// because the page is opened on a phone more often than anywhere else and a
// phone is the device least likely to already be signed in. signin.go carries
// why nothing in that redirect can come from the request.
//
// Every route sits under a per-address budget. RFC 8628 §5.2 requires it of
// the ones that accept a user code: a code short enough to type off a screen is
// short enough to guess, and the arithmetic in internal/devices' package doc
// assumes exactly this limit.
package device

import (
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/go-chi/chi/v5"
)

// DefaultVerifyLimit and DefaultVerifyBurst are the budget one source address
// gets on the routes that accept a user code.
//
// They are the login endpoint's numbers, on purpose. Both surfaces are a human
// typing a short secret that an attacker would rather guess, both are reached
// from a page rather than a script, and two different budgets would be two
// numbers to justify instead of one. internal/devices' package doc works out
// what this buys against a 34.6-bit code.
const (
	DefaultVerifyLimit = 5
	DefaultVerifyBurst = 10
)

// Services bundles what the verification page reads.
type Services struct {
	Devices  *devices.Service
	Sessions *accounts.Service
	Register *clients.Service
	Cookies  accounts.Cookies

	// AuthorizeURL and RedirectURI are the two halves of the sign-in
	// redirect, and both are process configuration. They arrive already
	// absolute — the authorization endpoint read off the provider itself so
	// it cannot drift from what discovery advertises, and the redirect built
	// from the configured issuer — because a URL this package assembled out
	// of parts would be a URL somebody could later be tempted to assemble
	// out of request parts.
	AuthorizeURL string
	RedirectURI  string
}

// Service serves the verification page over the device flows and the session
// the human already holds with registre.
type Service struct {
	devices      *devices.Service
	register     *clients.Service
	guard        accounts.Guard
	authorizeURL string
	redirectURI  string
	logger       *slog.Logger
}

// New builds the verification service.
func New(deps Services, logger *slog.Logger) *Service {
	return &Service{
		devices:      deps.Devices,
		register:     deps.Register,
		guard:        accounts.NewGuard(deps.Sessions, deps.Cookies),
		authorizeURL: deps.AuthorizeURL,
		redirectURI:  deps.RedirectURI,
		logger:       logger,
	}
}

// Mount registers the verification routes on router.
func Mount(router chi.Router, service *Service) {
	MountWithLimits(router, service, DefaultVerifyLimit, DefaultVerifyBurst)
}

// MountWithLimits registers them with an explicit per-address budget. Tests
// use it to make the limiter reachable without sending the real default's
// worth of traffic; production goes through Mount.
//
// /authenticate sits outside the session guard and inside everything else,
// which is the whole point of it: it is the route for somebody who has no
// session, so guarding it on one would 401 exactly the people it exists for.
// It is still inside the budget, and it still reads nothing from the request.
func MountWithLimits(router chi.Router, service *Service, limit float64, burst int) {
	budget := troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{
		Limit: limit, Burst: burst,
	})
	router.Route("/api/device", func(verify chi.Router) {
		verify.Use(budget, service.guard.CSRF)
		verify.Get("/authenticate", service.authenticate)
		verify.Group(func(private chi.Router) {
			private.Use(service.guard.Session)
			private.Get("/", service.pending)
			private.Post("/approve", service.approve)
			private.Post("/deny", service.deny)
		})
	})
}
