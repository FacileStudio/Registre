package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"

	"gorm.io/gorm"
)

// runUserCreate answers `registre user-create`, the phase-1 way to mint the
// first human accounts before any admin UI exists. It shares the process's
// database handle and credentials ceiling so a created password is hashed
// exactly like one that arrives through the login page.
func runUserCreate(ctx context.Context, db *gorm.DB, verifier *credentials.Verifier, log *slog.Logger) int {
	fs := flag.NewFlagSet("user-create", flag.ContinueOnError)
	email := fs.String("email", "", "the user's email address (required)")
	name := fs.String("name", "", "the user's display name")
	admin := fs.Bool("admin", false, "grant the admin role")
	roles := fs.String("roles", "", "comma-separated roles the account holds")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return 2
	}
	password := os.Getenv("REGISTRE_USER_PASSWORD")
	if *email == "" || password == "" {
		fmt.Fprintln(os.Stderr, "usage: REGISTRE_USER_PASSWORD=<secret> registre user-create --email <address>")
		fmt.Fprintln(os.Stderr, "                     [--admin] [--roles a,b]")
		fmt.Fprintln(os.Stderr, "the password comes from REGISTRE_USER_PASSWORD, never the command line:")
		fmt.Fprintln(os.Stderr, "a flag lands in shell history and ps output")
		return 2
	}
	users := identity.New(db, verifier)
	user, err := users.Create(ctx, *email, password, *name, *admin, splitRoles(*roles)...)
	if errors.Is(err, identity.ErrInvalid) {
		log.Error("registre: refused to create the user", slog.Any("error", err))
		return 1
	}
	if err != nil {
		log.Error("registre: failed to create the user", slog.Any("error", err))
		return 1
	}
	fmt.Printf("created user %s (%s)\n", user.ID, user.Email)
	return 0
}

// splitRoles splits a comma-separated role list, dropping empties.
func splitRoles(raw string) []string {
	var roles []string
	for _, role := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(role); trimmed != "" {
			roles = append(roles, trimmed)
		}
	}
	return roles
}
