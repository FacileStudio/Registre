// Package admin is the dashboard's API, phase 3's first slice. It is the
// module that owns HTTP over internal/keys and internal/clients — SPEC §5
// named this package for exactly that, so neither service ever had to move.
//
// Every /api/admin route answers only to an operator credential verified
// through the same identity path as `registre user-create`: an email and
// password presented as HTTP Basic on every request, checked by
// identity.Verify under the argon2 ceiling and refused unless the account
// holds the admin role. A live admin session cookie is accepted too, and then
// every mutation must carry X-Facile-CSRF, which no cross-origin page can set
// without a preflight this API never answers. Invite redemption is the one unauthenticated route, because it is
// how a human becomes one, and it runs rate-limited and ceiling-bound like
// the login endpoint it mirrors.
package admin

import (
	"context"
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/limits"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"

	"github.com/go-chi/chi/v5"
)

// DefaultRedeemLimit and DefaultRedeemBurst match the login endpoint's
// per-address budget. Redemption verifies a password-shaped secret against a
// token-shaped one, so it gets the same arithmetic a password guesser gets.
const (
	DefaultRedeemLimit = 5.0
	DefaultRedeemBurst = 10
)

// Services bundles the process-wide dependencies admin needs, so New stays
// inside the parameter ceiling as the dashboard grows.
type Services struct {
	Register *clients.Service
	Users    *identity.Service
	Keys     *keys.Service
	Invites  *invites.Service
	Logout   *logout.Service
	Sessions *accounts.Service
	Cookies  accounts.Cookies

	Access    *access.Service
	Audit     *audit.Service
	Avatars   *avatars.Service
	Directory *directory.Service

	// Self is registre's own client id, the one application the dashboard
	// refuses to restrict.
	Self string

	// Eraser deletes an account the way a person's own deletion does.
	Eraser Eraser
}

// Eraser deletes an account now, with every side effect of the
// self-service deletion: sessions ended, back-channel logout, one deletion
// notice per app (pushed to its deletion_uri and listed for polling), and
// the spaces it alone owned handed over. *gdpr.Service is the one in use.
type Eraser interface {
	Erase(ctx context.Context, userID string) error
}

// Service serves the dashboard's API over the register, the accounts, the
// keyring and the invitations.
type Service struct {
	register *clients.Service
	users    *identity.Service
	keyring  *keys.Service
	invites  *invites.Service
	logout   *logout.Service
	sessions *accounts.Service
	cookies  accounts.Cookies
	logger   *slog.Logger

	access    *access.Service
	audit     *audit.Service
	avatars   *avatars.Service
	directory *directory.Service
	self      string
	eraser    Eraser
}

// New builds the admin service over the process's services.
func New(deps Services, logger *slog.Logger) *Service {
	return &Service{
		register: deps.Register, users: deps.Users, keyring: deps.Keys,
		invites: deps.Invites, logout: deps.Logout, sessions: deps.Sessions,
		cookies: deps.Cookies, logger: logger,
		access: deps.Access, audit: deps.Audit, avatars: deps.Avatars, directory: deps.Directory, self: deps.Self,
		eraser: deps.Eraser,
	}
}

// Mount registers the admin routes on router. The /api/admin subtree is
// operator-only and rate-limited like login: Verify runs argon2 on every
// call, and the ceiling bounds memory, not attempts. /api/invites/redeem is
// public and gets the same budget.
func Mount(router chi.Router, service *Service) {
	router.Route("/api/admin", func(admin chi.Router) {
		admin.Use(limits.PerAddress(DefaultRedeemLimit, DefaultRedeemBurst))
		admin.Use(service.requireOperator)
		admin.Get("/clients", service.listClients)
		admin.Post("/clients", service.createClient)
		admin.Get("/clients/{clientID}", service.getClient)
		admin.Put("/clients/{clientID}", service.updateClient)
		admin.Post("/clients/{clientID}/update", service.updateClient)
		admin.Post("/clients/{clientID}/secret", service.rotateSecret)
		admin.Post("/clients/{clientID}/disable", service.disableClient)
		admin.Post("/clients/{clientID}/enable", service.enableClient)
		admin.Delete("/clients/{clientID}", service.deleteClient)
		admin.Get("/clients/{clientID}/access", service.clientAccess)
		admin.Put("/clients/{clientID}/access", service.setPolicy)
		admin.Put("/clients/{clientID}/grants/{userID}", service.putGrant)
		admin.Delete("/clients/{clientID}/grants/{userID}", service.deleteGrant)
		admin.Get("/events", service.listEvents)
		admin.Get("/users/{userID}", service.getUser)
		admin.Get("/users", service.listUsers)
		admin.Post("/users", service.createUser)
		admin.Put("/users/{userID}", service.updateUser)
		admin.Post("/users/{userID}/update", service.updateUser)
		admin.Post("/users/{userID}/password", service.setUserPassword)
		admin.Post("/users/{userID}/disable", service.disableUser)
		admin.Post("/users/{userID}/enable", service.enableUser)
		admin.Delete("/users/{userID}", service.deleteUser)
		admin.Get("/invites", service.listInvites)
		admin.Post("/invites", service.inviteUser)
		admin.Get("/keys", service.keyStatus)
	})
	router.With(limits.PerAddress(DefaultRedeemLimit, DefaultRedeemBurst)).Post("/api/invites/redeem", service.redeemInvite)
}
