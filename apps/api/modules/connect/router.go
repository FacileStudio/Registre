// Package connect is the person's half of a third-party connection: the
// consent screen's API, where a person allows or refuses a client that
// registered itself (CIMD or DCR), and the connected apps list, where they
// see and revoke what they allowed.
//
// Every route is behind the account session and every POST and DELETE behind
// the CSRF header. The consent routes also require that the session reading
// or answering a flow is the session that signed in to it: an auth request id
// alone grants nothing, the same rule the callback enforces.
package connect

import (
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/grants"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// Services is what the module reads and writes.
type Services struct {
	DB       *gorm.DB
	Register *clients.Service
	Grants   *grants.Service
	Sessions *accounts.Service
	Cookies  accounts.Cookies
	Issuer   string
}

// Service answers the connect routes.
type Service struct {
	db       *gorm.DB
	register *clients.Service
	grants   *grants.Service
	guard    accounts.Guard
	issuer   string
	logger   *slog.Logger
	now      func() time.Time
}

// New builds the service.
func New(deps Services, logger *slog.Logger) *Service {
	return &Service{
		db: deps.DB, register: deps.Register, grants: deps.Grants,
		guard: accounts.NewGuard(deps.Sessions, deps.Cookies), issuer: deps.Issuer, logger: logger,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Mount registers the routes.
func Mount(router chi.Router, service *Service) {
	router.Group(func(private chi.Router) {
		private.Use(service.guard.CSRF, service.guard.Session)
		private.Get("/api/connect", service.pending)
		private.Post("/api/connect", service.answer)
		private.Get("/api/account/apps", service.apps)
		private.Delete("/api/account/apps/{grantID}", service.revoke)
	})
}
