package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/internal/billing"
	"github.com/FacileStudio/Registre/apps/api/internal/challenges"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	spacesdir "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/grants"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
	"github.com/FacileStudio/Registre/apps/api/internal/metadoc"
	"github.com/FacileStudio/Registre/apps/api/internal/netguard"
	"github.com/FacileStudio/Registre/apps/api/internal/seed"
	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/internal/throttle"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"
	"github.com/FacileStudio/Registre/apps/api/internal/web"
	"github.com/FacileStudio/Registre/apps/api/modules/account"
	"github.com/FacileStudio/Registre/apps/api/modules/admin"
	"github.com/FacileStudio/Registre/apps/api/modules/avatar"
	"github.com/FacileStudio/Registre/apps/api/modules/connect"
	"github.com/FacileStudio/Registre/apps/api/modules/device"
	"github.com/FacileStudio/Registre/apps/api/modules/directory"
	"github.com/FacileStudio/Registre/apps/api/modules/docs"
	"github.com/FacileStudio/Registre/apps/api/modules/login"
	"github.com/FacileStudio/Registre/apps/api/modules/oidc"
	"github.com/FacileStudio/Registre/apps/api/modules/reset"
	"github.com/FacileStudio/Registre/apps/api/modules/spaces"

	"github.com/FacileStudio/tronc/health"
	"github.com/FacileStudio/tronc/httpx"
	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// services bundles the process-wide dependencies assemble wires into routes,
// so the function stays under the parameter ceiling as registre grows.
type services struct {
	verifier *credentials.Verifier
	log      *slog.Logger
	billing  env.Billing
	self     *selfServices
}

// assemble wires the process's services into routes. The dependencies travel
// together as one value so the function stays inside the parameter ceiling.
//
// Route precedence: oidc.Mount and login.Mount first, then the embedded
// client as the catch-all, so every page a human sees — "/" included — is the
// SvelteKit app while API routes keep priority over it.
func assemble(
	ctx context.Context,
	db *gorm.DB,
	root []byte,
	cfg env.Config,
	svc services,
) (chi.Router, error) {
	keyring := keys.New(db, root, keys.Config{
		RotateEvery:  cfg.RotationEvery,
		PublishDelay: cfg.PublishDelay,
		Retention:    cfg.KeyRetention,
	}, svc.log)
	if err := keyring.Ensure(ctx); err != nil {
		return nil, err
	}
	keyring.Start(ctx)

	register := clients.New(db, svc.verifier)
	if err := applySeed(ctx, register, cfg.SeedFile, svc.log); err != nil {
		return nil, err
	}
	verificationURL := strings.TrimSuffix(cfg.Issuer, "/") + devices.VerificationPath
	if err := register.EnsureSelf(ctx, verificationURL); err != nil {
		return nil, err
	}
	self, err := buildSelf(db, root, cfg, svc)
	if err != nil {
		return nil, err
	}
	svc.self = self
	connections, err := buildSocial(db, cfg, self.flow != nil, svc.log)
	if err != nil {
		return nil, err
	}
	users := identity.New(db, svc.verifier).WithConsent(self.consents)
	invitationService := invites.New(db, svc.verifier).WithBreachCheck(self.pwned)
	deviceFlows := devices.New(db)
	pictures := avatars.New(db, cfg.Issuer)
	teams := spacesdir.New(db, pictures.URL, spacesdir.NewNudger(register.List, svc.log), svc.log).
		WithInvitations(self.invitationsFor(cfg))

	backchannel := logout.New(keyring, register, cfg.Issuer, svc.log)
	storage := oidc.NewStorage(db, oidc.Services{
		Keys: keyring, Register: register, Users: users,
		Logout:  backchannel,
		Devices: deviceFlows, Avatars: pictures, Connections: connections,
		Documents: documentFetcher(), Grants: grants.New(db),
	}, oidc.Config{
		Issuer:          cfg.Issuer,
		AccessTokenTTL:  cfg.AccessTokenTTL,
		RefreshTokenTTL: cfg.RefreshTokenTTL,
	}, svc.log)
	storage.StartPurge(ctx, tokenRecordRetention)

	provider, err := oidc.NewProvider(storage, oidc.ProviderConfig{
		Issuer:   cfg.Issuer,
		RootKey:  root,
		Insecure: cfg.InsecureIssuer(),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	security, err := securityConfig()
	if err != nil {
		return nil, err
	}
	router := httpx.NewRouter(httpx.Config{
		TrustedProxies: cfg.TrustedProxies,
		CDNProxies:     cfg.CDNProxies,
		CDNHeader:      cfg.CDNHeader,
		Logger:         svc.log,
		Security:       security,
	})
	router.Use(oidc.CORS(oidc.PublicPaths(provider), guardedCORS(cfg.CORSAllowedOrigins)))
	health.Mount(router, health.DB(sqlDB))
	mountVersion(router)
	sessions := accounts.New(db).WithNotifier(backchannel, svc.log)
	cookies := accounts.NewCookies(!cfg.InsecureIssuer())
	client, err := web.Handler()
	if err != nil {
		return nil, err
	}
	mountLogout(router, sessions, cookies, client)
	oidc.MountWithLimits(router, provider, oidc.Binding{Storage: storage, Sessions: sessions, Cookies: cookies},
		oidc.Limits{
			Token: cfg.TokenRate, TokenBurst: cfg.TokenBurst,
			PublicToken: cfg.PublicTokenRate, PublicTokenBurst: cfg.PublicTokenBurst,
		})
	mountPeople(router, cfg, people{
		ctx: ctx, db: db, users: users, invites: invitationService, register: register,
		keyring: keyring, services: svc, root: root,
		deviceFlows: deviceFlows, verificationURL: verificationURL, pictures: pictures, teams: teams,
		authorizeURL: provider.AuthorizationEndpoint().Absolute(cfg.Issuer), sessions: sessions, cookies: cookies,
		social: connections,
	})
	avatar.Mount(router, pictures)
	directory.Mount(router, directory.New(register, teams, svc.log).WithWriter(svc.billing.EntitlementsWriter).
		WithDeletions(func(ctx context.Context, clientID string, since time.Time, after *gdpr.Cursor) ([]gdpr.Deleted, string, error) {
			return gdpr.DeletedFor(ctx, db, clientID, since, after)
		}))
	connect.Mount(router, connect.New(connect.Services{
		DB: db, Register: register, Grants: grants.New(db), Sessions: sessions, Cookies: cookies, Issuer: cfg.Issuer,
	}, svc.log))
	docs.Mount(router)
	router.Handle("/*", client)
	return router, nil
}

// people bundles what the human-facing modules are built from, so mountPeople
// stays inside the parameter ceiling as the pages grow.
type people struct {
	ctx         context.Context
	db          *gorm.DB
	users       *identity.Service
	invites     *invites.Service
	register    *clients.Service
	keyring     *keys.Service
	deviceFlows *devices.Service
	pictures    *avatars.Service
	teams       *spacesdir.Service
	social      *social.Service
	sessions    *accounts.Service
	cookies     accounts.Cookies
	root        []byte
	services

	// authorizeURL and verificationURL are the sign-in round trip the
	// device page sends a session-less human on. They are built here, from
	// the provider and the configured issuer, so that no handler is ever in
	// a position to assemble a redirect target out of a request.
	authorizeURL    string
	verificationURL string
}

// mountPeople registers every route a human reaches: the login flow and its
// second factor, the account pages behind it, password recovery, the operator
// dashboard, and the page a device flow is approved from.
//
// The account session and cookie policy are built once in assemble and shared
// by the callback binding, login and account: two instances would be two cookie
// names the day one is built with the wrong scheme, a "random logout" bug.
func mountPeople(router chi.Router, cfg env.Config, p people) {
	factors := totp.New(p.db, p.root)
	pending := challenges.New(p.db)
	pending.StartPurge(p.ctx, challengeRetention, p.log)
	rights := buildRights(cfg, p)

	login.MountWithLimits(router, login.New(p.users, p.log).
		WithThrottle(throttle.New(p.db).StartPurge(p.ctx, p.log)).
		WithBrowserConfig(cfg.JournalBrowserURL, cfg.JournalBrowserKey).
		WithPasskeys(p.self.passkeys).WithSignup(p.self.flow != nil).WithSocial(p.social).
		WithSecondFactor(login.SecondFactor{
			Challenges: pending,
			Codes:      factors,
			Accounts:   p.sessions,
			Cookies:    p.cookies,
		}), cfg.LoginRate, cfg.LoginBurst)
	account.Mount(router, account.New(account.Services{
		Sessions: p.sessions, TOTP: factors, Users: p.users, Passwords: p.verifier,
		DB: p.db, Cookies: p.cookies, Issuer: cfg.Issuer,
		Logout:  logout.New(p.keyring, p.register, cfg.Issuer, p.log),
		Avatars: p.pictures, Self: accountSelf(p, rights),
	}, p.log))
	mountSignup(router, p, p.sessions, p.cookies)
	spaces.Mount(router, spaces.New(p.teams, p.sessions, p.cookies, p.log).WithBilling(billing.FromConfig(p.billing, p.log), cfg.Issuer))
	reset.Mount(router, reset.New(recoveryService(cfg, p), p.log))
	admin.Mount(router, admin.New(admin.Services{
		Register: p.register, Users: p.users, Keys: p.keyring, Invites: p.invites,
		Logout:   logout.New(p.keyring, p.register, cfg.Issuer, p.log),
		Sessions: p.sessions, Cookies: p.cookies,
		Access: access.New(p.db, clients.SelfClientID), Audit: audit.New(p.db),
		Avatars: p.pictures, Directory: p.teams, Self: clients.SelfClientID, Eraser: rights,
	}, p.log))
	device.Mount(router, device.New(device.Services{
		Devices: p.deviceFlows, Sessions: p.sessions, Register: p.register, Cookies: p.cookies,
		AuthorizeURL: p.authorizeURL, RedirectURI: p.verificationURL, Consent: p.users.ConsentOwed,
	}, p.log))
}

// openerPolicy keeps the sign-in pages reachable from a popup. tronc's
// same-origin default puts a page opened from another origin in a browsing
// context group of its own, so the app or MCP client that opened the popup
// loses it: window.closed reads true at once and nothing it waits on arrives.
// An identity provider is that popup, so it opts out, as it did before tronc
// sent the header.
const openerPolicy = "unsafe-none"

// visionScript is the origin of Vision's tracker, which app.html loads on the
// production host.
const visionScript = "https://vision.facile.studio"

// securityConfig is tronc's default security headers with the opener policy
// above, and a script-src that allows Vision's tracker and, by hash, the
// inline scripts of the embedded client.
func securityConfig() (troncmiddleware.SecurityConfig, error) {
	hashes, err := web.InlineScriptHashes()
	if err != nil {
		return troncmiddleware.SecurityConfig{}, err
	}
	return troncmiddleware.SecurityConfig{
		CrossOriginOpenerPolicy: openerPolicy,
		CSPSources:              map[string][]string{"script-src": append([]string{visionScript}, hashes...)},
	}, nil
}

// challengeRetention is how long a spent or expired login challenge row
// outlives the challenge itself. Short: the row records that a password was
// proven, which is not a fact worth keeping once the flow it belonged to is
// over, and the table would otherwise grow with every second-factor login.
const challengeRetention = time.Hour

// mountLogout answers /logout and /api/logout. Signing out tells every app
// the session reached, so these log the person out of the suite.
//
// POST needs X-Facile-CSRF and answers JSON. GET signs out and goes home only
// when accounts.MayLogOut accepts it, which is what a suite app's sign-out
// redirect looks like. Any other GET ends nothing: /logout serves page, the
// client's "Sign out of Facile?" confirmation, and /api/logout answers 405.
func mountLogout(router chi.Router, sessions *accounts.Service, cookies accounts.Cookies, page http.Handler) {
	end := func(w http.ResponseWriter, r *http.Request) {
		if err := sessions.SignOut(r.Context(), cookies.Read(r)); err != nil {
			slog.Warn("registre: ending the account session failed", slog.Any("error", err))
		}
		cookies.Clear(w)
	}
	refused := map[string]http.Handler{
		"/logout":     page,
		"/api/logout": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { accounts.RefuseLogout(w) }),
	}
	csrf := accounts.NewGuard(nil, accounts.Cookies{}).CSRF
	for path, refuse := range refused {
		router.Get(path, logoutByLink(end, refuse))
		router.With(csrf).Post(path, logoutByAPI(end))
	}
}

// logoutByLink ends the session for a GET that accounts.MayLogOut accepts
// and redirects to /, and hands every other GET to refuse.
func logoutByLink(end http.HandlerFunc, refuse http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !accounts.MayLogOut(r) {
			refuse.ServeHTTP(w, r)
			return
		}
		end(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// logoutByAPI ends the session for a POST the CSRF guard let through and
// answers JSON.
func logoutByAPI(end http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		end(w, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}
}

// documentFetcher reads client ID metadata documents through the SSRF guard:
// public addresses only, checked again at connect time, no proxy, no redirect.
func documentFetcher() *metadoc.Fetcher {
	return &metadoc.Fetcher{Client: (&netguard.Guard{}).Client(5*time.Second, nil)}
}

// guardedCORS is the credentialed cross-origin policy of registre's own API,
// or no policy at all when no origin is configured, which is what the router
// applied before the public OAuth endpoints got their own.
func guardedCORS(origins []string) func(http.Handler) http.Handler {
	if len(origins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return troncmiddleware.CORS(troncmiddleware.CORSConfig{AllowedOrigins: origins, AllowCredentials: true})
}

// applySeed brings the register up to the file, when one is configured.
func applySeed(ctx context.Context, register *clients.Service, path string, log *slog.Logger) error {
	if path == "" {
		log.Info("registre: no seed file configured, the database is the only source of clients")
		return nil
	}
	file, err := seed.Load(path)
	if err != nil {
		return err
	}
	return seed.Apply(ctx, register, file, log)
}
