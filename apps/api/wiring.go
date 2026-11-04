package main

import (
	"context"
	"log/slog"
	"os"
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
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	"github.com/FacileStudio/Registre/apps/api/internal/seed"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"
	"github.com/FacileStudio/Registre/apps/api/internal/web"
	"github.com/FacileStudio/Registre/apps/api/modules/account"
	"github.com/FacileStudio/Registre/apps/api/modules/admin"
	"github.com/FacileStudio/Registre/apps/api/modules/avatar"
	"github.com/FacileStudio/Registre/apps/api/modules/device"
	"github.com/FacileStudio/Registre/apps/api/modules/directory"
	"github.com/FacileStudio/Registre/apps/api/modules/docs"
	"github.com/FacileStudio/Registre/apps/api/modules/login"
	"github.com/FacileStudio/Registre/apps/api/modules/oidc"
	"github.com/FacileStudio/Registre/apps/api/modules/reset"
	"github.com/FacileStudio/Registre/apps/api/modules/spaces"

	mailer "github.com/FacileStudio/facteur"
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
	users := identity.New(db, svc.verifier)
	invitationService := invites.New(db, svc.verifier)
	deviceFlows := devices.New(db)
	pictures := avatars.New(db, cfg.Issuer)
	teams := spacesdir.New(db, pictures.URL, spacesdir.NewNudger(register.List, svc.log), svc.log)

	storage := oidc.NewStorage(db, oidc.Services{
		Keys: keyring, Register: register, Users: users,
		Logout:  logout.New(keyring, register, cfg.Issuer, svc.log),
		Devices: deviceFlows, Avatars: pictures,
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
	router := httpx.NewRouter(httpx.Config{
		TrustedProxies: cfg.TrustedProxies,
		CDNProxies:     cfg.CDNProxies,
		CDNHeader:      cfg.CDNHeader,
		Logger:         svc.log,
		CORS: troncmiddleware.CORSConfig{
			AllowedOrigins:   cfg.CORSAllowedOrigins,
			AllowCredentials: true,
		},
	})
	health.Mount(router, health.DB(sqlDB))
	sessions, cookies := accounts.New(db), accounts.NewCookies(!cfg.InsecureIssuer())
	oidc.Mount(router, provider, oidc.Binding{Storage: storage, Sessions: sessions, Cookies: cookies})
	mountPeople(router, cfg, people{
		ctx: ctx, db: db, users: users, invites: invitationService, register: register,
		keyring: keyring, services: svc, root: root,
		deviceFlows: deviceFlows, verificationURL: verificationURL, pictures: pictures, teams: teams,
		authorizeURL: provider.AuthorizationEndpoint().Absolute(cfg.Issuer), sessions: sessions, cookies: cookies,
	})
	avatar.Mount(router, pictures)
	directory.Mount(router, directory.New(register, teams, svc.log).WithWriter(svc.billing.EntitlementsWriter))
	docs.Mount(router)
	client, err := web.Handler()
	if err != nil {
		return nil, err
	}
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

	login.Mount(router, login.New(p.users, p.log).
		WithBrowserConfig(cfg.JournalBrowserURL, cfg.JournalBrowserKey).
		WithSecondFactor(login.SecondFactor{
			Challenges: pending,
			Codes:      factors,
			Accounts:   p.sessions,
			Cookies:    p.cookies,
		}))
	account.Mount(router, account.New(account.Services{
		Sessions: p.sessions, TOTP: factors, Users: p.users, Passwords: p.verifier,
		DB: p.db, Cookies: p.cookies, Issuer: cfg.Issuer,
		Logout:  logout.New(p.keyring, p.register, cfg.Issuer, p.log),
		Avatars: p.pictures,
	}, p.log))
	spaces.Mount(router, spaces.New(p.teams, p.sessions, p.cookies, p.log).WithBilling(billing.FromConfig(p.billing, p.log), cfg.Issuer))
	reset.Mount(router, reset.New(recoveryService(cfg, p), p.log))
	admin.Mount(router, admin.New(admin.Services{
		Register: p.register, Users: p.users, Keys: p.keyring, Invites: p.invites,
		Logout:   logout.New(p.keyring, p.register, cfg.Issuer, p.log),
		Sessions: p.sessions, Cookies: p.cookies,
		Access: access.New(p.db, clients.SelfClientID), Audit: audit.New(p.db),
		Avatars: p.pictures, Directory: p.teams, Self: clients.SelfClientID,
	}, p.log))
	device.Mount(router, device.New(device.Services{
		Devices: p.deviceFlows, Sessions: p.sessions, Register: p.register, Cookies: p.cookies,
		AuthorizeURL: p.authorizeURL, RedirectURI: p.verificationURL,
	}, p.log))
}

// challengeRetention is how long a spent or expired login challenge row
// outlives the challenge itself. Short: the row records that a password was
// proven, which is not a fact worth keeping once the flow it belonged to is
// over, and the table would otherwise grow with every second-factor login.
const challengeRetention = time.Hour

// recoveryService builds password recovery, or returns nil when this instance
// has no mailer.
//
// nil is a supported state rather than a boot failure. Recovery is the only
// feature in registre that depends on something outside it, and an instance
// whose operators mint every account by hand has no use for it. The routes
// still mount and still answer identically, so the absence is not something a
// caller can detect.
func recoveryService(cfg env.Config, p people) *recovery.Service {
	smtp := mailer.FromEnv(os.Getenv)
	outbound, configured := outboundMail(smtp, p.log)
	if !configured {
		p.log.Warn("registre: SMTP is not configured, password recovery will accept " +
			"requests and send nothing; set SMTP_HOST and SMTP_FROM to turn it on")
		return nil
	}
	p.log.Info("registre: password recovery is on", slog.String("smtp_host", smtp.Host))
	return recovery.New(p.db, p.verifier, outbound, cfg.Issuer)
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
