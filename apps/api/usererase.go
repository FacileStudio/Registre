package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"os"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/crypto"
	spacesdir "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"

	"gorm.io/gorm"
)

// runUserErase answers `registre user-erase`: it erases one account now,
// through gdpr.Erase, the path the grace-period sweeper and the dashboard
// delete run. Sessions end, the account and its rows go, the apps get their
// back-channel logout and deletion notices. It exists for throwaway smoke
// accounts, which should not wait thirty days.
//
// It takes exactly one exact address and prints only a status, so a log of
// the run holds no address. No farewell mail is sent: the mailer delivers on
// a goroutine this short process would not wait for.
func runUserErase(ctx context.Context, db *gorm.DB, verifier *credentials.Verifier, log *slog.Logger) int {
	email, ok := eraseTarget(os.Args[2:])
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: registre user-erase --email <address>")
		return 2
	}
	cfg, err := env.Load()
	if err != nil {
		log.Error("registre: failed to load the configuration", slog.Any("error", err))
		return 1
	}
	root, err := crypto.KeyFromBase64(cfg.EncryptionKey)
	if err != nil {
		log.Error("registre: invalid ENCRYPTION_KEY", slog.Any("error", err))
		return 1
	}
	users, rights := eraseServices(db, root, cfg, services{verifier: verifier, log: log})
	status, err := eraseUser(ctx, users, rights, email)
	fmt.Println(status)
	if err != nil {
		log.Error("registre: user-erase "+status, slog.Any("error", err))
		return 1
	}
	return 0
}

// eraseServices builds the rights service the way the dashboard's delete
// gets it, through buildRights, minus the mailer.
func eraseServices(db *gorm.DB, root []byte, cfg env.Config, svc services) (*identity.Service, *gdpr.Service) {
	register := clients.New(db, svc.verifier)
	users := identity.New(db, svc.verifier)
	svc.self = &selfServices{audit: audit.New(db)}
	return users, buildRights(cfg, people{
		db: db, users: users, register: register, services: svc,
		keyring: keys.New(db, root, keys.Config{
			RotateEvery: cfg.RotationEvery, PublishDelay: cfg.PublishDelay, Retention: cfg.KeyRetention,
		}, svc.log),
		teams: spacesdir.New(db, avatars.New(db, cfg.Issuer).URL, spacesdir.NewNudger(register.List, svc.log), svc.log),
	})
}

// eraseTarget answers the one address args name, and false unless args are
// exactly --email with a single bare address.
func eraseTarget(args []string) (string, bool) {
	fs := flag.NewFlagSet("user-erase", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	email := fs.String("email", "", "the account's exact email address (required)")
	if fs.Parse(args) != nil || fs.NArg() != 0 {
		return "", false
	}
	parsed, err := mail.ParseAddress(*email)
	if err != nil || parsed.Name != "" || parsed.Address != *email {
		return "", false
	}
	return *email, true
}

// eraseUser erases the account at email and answers a status that names
// no address: erased, not found or failed.
func eraseUser(ctx context.Context, users *identity.Service, rights *gdpr.Service, email string) (string, error) {
	user, err := users.ByEmail(ctx, email)
	if err != nil {
		return "not found", err
	}
	if err := rights.Erase(ctx, user.ID); err != nil {
		return "failed", fmt.Errorf("erase %s: %w", user.ID, err)
	}
	return "erased", nil
}
