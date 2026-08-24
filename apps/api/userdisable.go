package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/crypto"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"

	"gorm.io/gorm"
)

// runUserDisable answers `registre user-disable`: it revokes an account's
// ability to sign in, ends its provider sessions and fires back-channel
// logout at every app those sessions had reached. It is the phase-2 exit
// walked by hand — deactivating a human drops them from every app within
// seconds — until the phase-3 dashboard owns it.
//
// It runs before assemble in run(), so it builds the pieces it needs itself:
// the keyring to sign the logout tokens with and the register to read each
// app's logout URI from.
func runUserDisable(ctx context.Context, db *gorm.DB, verifier *credentials.Verifier, log *slog.Logger) int {
	fs := flag.NewFlagSet("user-disable", flag.ContinueOnError)
	email := fs.String("email", "", "the account's email address (required)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return 2
	}
	if *email == "" {
		fmt.Fprintln(os.Stderr, "usage: registre user-disable --email <address>")
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
	keyring := keys.New(db, root, keys.Config{
		RotateEvery: cfg.RotationEvery, PublishDelay: cfg.PublishDelay, Retention: cfg.KeyRetention,
	}, log)
	register := clients.New(db, verifier)
	users := identity.New(db, verifier)

	user, err := users.ByEmail(ctx, *email)
	if err != nil {
		log.Error("registre: no such user", slog.String("email", *email), slog.Any("error", err))
		return 1
	}
	reached, err := users.Disable(ctx, user.ID, time.Now().UTC())
	if err != nil {
		log.Error("registre: failed to disable the user", slog.Any("error", err))
		return 1
	}
	fire := logout.New(keyring, register, cfg.Issuer, log)
	if err := fire.Fire(ctx, user.ID, reached); err != nil {
		log.Warn("registre: back-channel logout incomplete", slog.Any("error", err))
	}
	fmt.Printf("disabled %s (%s); back-channel logout sent to %d app(s)\n", user.ID, user.Email, len(reached))
	return 0
}
