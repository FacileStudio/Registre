// Package account is the self-service pages' API: the routes a human uses to
// manage their own security, over internal/accounts, internal/totp and
// internal/identity.
//
// Every route here is reached with the account cookie, so unlike modules/admin
// this module has a CSRF surface and answers for it. requireCSRF runs ahead of
// requireSession on the whole subtree and refuses any non-GET request that
// arrives without X-Facile-CSRF, before a password is read or a hash is
// computed. porte enforces the same rule for the same reason, and a client
// that sends the cookie without the header sees every read succeed and every
// write 403, which reads as "the app works, saving is broken".
//
// Turning a factor off costs what turning it on cost: /totp/disable wants the
// current password and a live code, and /totp/recovery-codes wants the
// password. A session on a borrowed laptop that can strip the second factor by
// itself makes the factor decoration.
package account

import (
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/limits"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// MinPasswordLength is the shortest password this module will store. It is a
// floor, not a policy: length is the only property that reliably buys entropy,
// and a composition rule buys a predictable substitution instead.
const MinPasswordLength = credentials.MinPasswordLength

// Services bundles the process-wide dependencies the account pages need, so
// New stays inside the parameter ceiling as the pages grow.
//
// Passwords and DB are here because changing a password is the one thing these
// pages do that internal/identity has no writer for. Hashing goes through the
// same argon2 ceiling every other credential does; nothing in this package may
// reach porte/local itself.
type Services struct {
	Sessions  *accounts.Service
	TOTP      *totp.Service
	Users     *identity.Service
	Passwords *credentials.Verifier
	DB        *gorm.DB
	Cookies   accounts.Cookies
	Issuer    string
	Avatars   *avatars.Service

	// Logout tells the apps a session reached that it has ended. A password
	// change ends every session the account holds, and an app holding its
	// own cookie learns that only if registre says so.
	Logout *logout.Service

	// Self is the sign-up era's half: passkeys, consent, export and
	// deletion. A zero value turns those routes into 404s.
	Self SelfService
}

// SelfService bundles what the passkey, consent and GDPR routes need.
type SelfService struct {
	Passkeys *passkeys.Service
	Consents *consent.Service
	Rights   *gdpr.Service
	Audit    *audit.Service
	Pwned    credentials.Screen

	// Social lists, links and unlinks external sign-in accounts. Nil turns
	// the connections routes into an empty list and 404s.
	Social *social.Service
}

// Service serves the account pages over the session, the second factor and the
// human's own credentials.
type Service struct {
	sessions  *accounts.Service
	totp      *totp.Service
	users     *identity.Service
	passwords *credentials.Verifier
	db        *gorm.DB
	cookies   accounts.Cookies
	issuer    string
	notifier  *logout.Service
	avatars   *avatars.Service
	self      SelfService
	logger    *slog.Logger
}

// New builds the account service over the process's services.
func New(deps Services, logger *slog.Logger) *Service {
	return &Service{
		sessions: deps.Sessions, totp: deps.TOTP, users: deps.Users,
		passwords: deps.Passwords, db: deps.DB, cookies: deps.Cookies,
		issuer: deps.Issuer, notifier: deps.Logout, avatars: deps.Avatars, self: deps.Self, logger: logger,
	}
}

// Mount registers the account routes on router.
//
// Logout sits outside requireSession deliberately. porte shipped the opposite
// and it bit: behind the middleware a stale cookie got a 401 and was therefore
// never cleared, so the state that makes someone log out was the state where
// logging out failed. It still passes through requireCSRF, because a
// cross-site page ending someone's session signs them out of every app. GET
// skips that guard, so the handler applies accounts.MayLogOut itself.
func Mount(router chi.Router, service *Service) {
	guard := accounts.NewGuard(service.sessions, service.cookies)
	router.Route("/api/account", func(account chi.Router) {
		account.Use(guard.CSRF)
		account.Group(func(open chi.Router) {
			open.Post("/logout", service.logout)
			open.Get("/logout", service.logout)
		})
		account.Group(func(private chi.Router) {
			private.Use(guard.Session)
			private.Get("/", service.me)
			private.Post("/password", service.changePassword)
			private.Put("/avatar", service.putAvatar)
			private.Delete("/avatar", service.deleteAvatar)
			private.Post("/totp/begin", service.beginTOTP)
			private.Post("/totp/confirm", service.confirmTOTP)
			private.Post("/totp/disable", service.disableTOTP)
			private.Post("/totp/recovery-codes", service.regenerateRecoveryCodes)
			mountSelf(private, service)
		})
	})
}

// SelfLimit and SelfBurst are the per-address and per-session budget of the
// passkey, reauthentication, consent and GDPR routes.
const (
	SelfLimit = 2.0
	SelfBurst = 20
)

// mountSelf registers the passkey, reauthentication, profile, consent,
// export and deletion routes behind a per-address and a per-session budget.
// Adding or removing a passkey and requesting deletion also need a factor
// proven within accounts.ReauthWindow.
func mountSelf(private chi.Router, service *Service) {
	budget := chi.Chain(limits.PerAddress(SelfLimit, SelfBurst),
		limits.PerCookie(service.cookies.Name(), SelfLimit, SelfBurst))
	routes := private.With(budget...)
	routes.Post("/profile", service.updateProfile)
	routes.Post("/consent", service.acceptConsent)
	routes.Post("/reauth", service.reauthPassword)
	routes.Post("/reauth/passkey", service.beginReauthPasskey)
	routes.Post("/reauth/passkey/finish", service.finishReauthPasskey)
	routes.Get("/passkeys", service.listPasskeys)
	routes.Post("/passkeys/begin", service.beginPasskey)
	routes.Post("/passkeys/finish", service.finishPasskey)
	routes.Patch("/passkeys/{id}", service.renamePasskey)
	routes.Delete("/passkeys/{id}", service.removePasskey)
	routes.Get("/connections", service.listConnections)
	routes.Post("/connections/{provider}/link", service.linkConnection)
	routes.Get("/connections/{provider}/return", service.returnConnection)
	routes.Delete("/connections/{provider}", service.unlinkConnection)
	routes.Get("/export", service.export)
	routes.Post("/deletion", service.requestDeletion)
	routes.Delete("/deletion", service.cancelDeletion)
}
