package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	spacesdir "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/pwned"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	"github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/modules/account"
	"github.com/FacileStudio/Registre/apps/api/modules/legal"
	signuproutes "github.com/FacileStudio/Registre/apps/api/modules/signup"

	mailer "github.com/FacileStudio/facteur"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// housekeepingEvery is how often due deletions run, deletion notices are
// retried, and expired ceremonies, sign-up links, proofs and invitations are purged.
const housekeepingEvery = 10 * time.Minute

// selfServices is what public sign-up added: passkeys, consent, the sign-up
// flow (nil while sign-up is closed) and the mailer they share.
type selfServices struct {
	passkeys *passkeys.Service
	consents *consent.Service
	flow     *signup.Service
	audit    *audit.Service
	mailer   recovery.Mailer
	pwned    *pwned.Checker
}

// buildSelf builds the sign-up era's services. Sign-up stays closed unless
// SIGNUP_ENABLED is set, ALTCHA_HMAC_KEY is long enough and SMTP is
// configured; the boot log says which.
func buildSelf(db *gorm.DB, root []byte, cfg env.Config, svc services) (*selfServices, error) {
	settings, err := env.LoadSignup(cfg.Issuer, cfg.InsecureIssuer())
	if err != nil {
		return nil, err
	}
	keys, err := passkeys.New(db, passkeys.Config{RPID: settings.RPID, Origins: settings.Origins}, svc.log)
	if err != nil {
		return nil, err
	}
	out := &selfServices{passkeys: keys, consents: consent.New(db, root), audit: audit.New(db)}
	out.mailer, _ = outboundMail(mailer.FromEnv(os.Getenv), svc.log)
	if settings.BreachCheck {
		out.pwned = pwned.New(svc.log)
	}
	if !openSignup(settings, out.mailer != nil, svc.log) {
		return out, nil
	}
	out.flow = signup.New(signup.Deps{
		DB: db, Verifier: svc.verifier, Consents: out.consents, Passkeys: keys, Pwned: out.pwned, Mailer: out.mailer,
	}, settings.AltchaKey, cfg.Issuer)
	return out, nil
}

// openSignup decides and logs whether sign-up serves.
func openSignup(settings env.Signup, mail bool, log *slog.Logger) bool {
	switch {
	case !settings.Requested:
		log.Info("registre: public sign-up is off (SIGNUP_ENABLED is not set)")
	case !settings.Open():
		log.Error("registre: SIGNUP_ENABLED is set but ALTCHA_HMAC_KEY is missing or shorter than 32 bytes; sign-up stays off")
	case !mail:
		log.Error("registre: SIGNUP_ENABLED is set but SMTP is not configured; sign-up stays off")
	default:
		log.Info("registre: public sign-up is on")
		return true
	}
	return false
}

// invitationsFor is how space invitations send their notices.
func (s *selfServices) invitationsFor(cfg env.Config) spacesdir.Invitations {
	return spacesdir.Invitations{Mailer: s.mailer, Issuer: cfg.Issuer, SignupOpen: s.flow != nil}
}

// accountSelf builds the account pages' sign-up era services, including the
// GDPR service, and starts the hourly housekeeping.
func accountSelf(cfg env.Config, p people) account.SelfService {
	rights := gdpr.New(gdpr.Deps{
		DB: p.db, Sessions: p.users, Audit: p.self.audit, Mailer: p.self.mailer,
		Notifier: logout.New(p.keyring, p.register, cfg.Issuer, p.log), Directory: p.teams, Logger: p.log,
	}, cfg.Issuer)
	rights.Start(p.ctx, housekeepingEvery, func(ctx context.Context) {
		cutoff := time.Now().UTC().Add(-time.Hour)
		purge(ctx, p.log, p.self.passkeys.Purge, cutoff)
		purge(ctx, p.log, p.teams.PurgeInvitations, cutoff)
		purge(ctx, p.log, p.social.PurgeExpired, cutoff)
		if p.self.flow != nil {
			purge(ctx, p.log, p.self.flow.Purge, cutoff)
		}
	})
	return account.SelfService{
		Passkeys: p.self.passkeys, Consents: p.self.consents, Rights: rights, Audit: p.self.audit, Pwned: p.self.pwned,
		Social: p.social,
	}
}

// purge runs one purge and logs a failure.
func purge(ctx context.Context, log *slog.Logger, run func(context.Context, time.Time) error, cutoff time.Time) {
	if err := run(ctx, cutoff); err != nil {
		log.Warn("registre: housekeeping purge failed", slog.Any("error", err))
	}
}

// mountSignup registers the sign-up and legal routes.
func mountSignup(router chi.Router, p people, sessions *accounts.Service, cookies accounts.Cookies) {
	signuproutes.Mount(router, signuproutes.New(signuproutes.Deps{
		Flow: p.self.flow, Users: p.users, Sessions: sessions, Cookies: cookies, Audit: p.self.audit,
	}, p.log))
	legal.Mount(router)
}
