package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/FacileStudio/Registre/apps/api/internal/breakglass"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"

	"gorm.io/gorm"
)

// runBreakGlass answers `registre break-glass`, the recovery SPEC §7 gates
// phase 4 on: the way back in when the way in is the problem.
//
// It is a subcommand rather than a route on purpose. Whoever can run this can
// already reach the database, so it grants nothing they did not have, and a
// break-glass path reachable over HTTP is a back door with a nicer name.
//
// The password comes from the environment for the same reason every other
// secret in this binary does: a flag lands in shell history, in ps output and
// in whatever shipped the operator's terminal somewhere.
func runBreakGlass(ctx context.Context, db *gorm.DB, verifier *credentials.Verifier, log *slog.Logger) int {
	fs := flag.NewFlagSet("break-glass", flag.ContinueOnError)
	email := fs.String("email", "", "the operator address to restore (required)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return 2
	}
	password := os.Getenv("REGISTRE_BREAK_GLASS_PASSWORD")
	if *email == "" || password == "" {
		printBreakGlassUsage()
		return 2
	}
	outcome, err := breakglass.New(db, verifier).Restore(ctx, *email, password)
	if err != nil {
		log.Error("registre: break-glass refused", slog.Any("error", err))
		return 1
	}
	log.Warn("registre: break-glass restored an operator account",
		slog.String("user_id", outcome.UserID), slog.String("email", outcome.Email),
		slog.Bool("created", outcome.Created), slog.Bool("reenabled", outcome.Reenabled),
		slog.Bool("factor_cleared", outcome.FactorCleared),
		slog.Bool("promoted_to_admin", outcome.PromotedToAdmin),
		slog.Int64("sessions_ended", outcome.SessionsEnded))
	printBreakGlassOutcome(outcome)
	return 0
}

// printBreakGlassUsage explains the command on stderr, including why the
// password is not a flag.
func printBreakGlassUsage() {
	fmt.Fprintln(os.Stderr,
		"usage: REGISTRE_BREAK_GLASS_PASSWORD=<secret> registre break-glass --email <address>")
	fmt.Fprintln(os.Stderr,
		"the password comes from REGISTRE_BREAK_GLASS_PASSWORD, never the command line:")
	fmt.Fprintln(os.Stderr, "a flag lands in shell history and ps output")
	fmt.Fprintf(os.Stderr, "it must be at least %d characters\n", breakglass.MinPasswordLength)
}

// printBreakGlassOutcome tells the operator what changed, on stdout, because
// an operation run in an incident has to be verifiable at the moment it runs
// rather than reconstructed from logs afterwards.
func printBreakGlassOutcome(outcome breakglass.Outcome) {
	fmt.Printf("restored %s (%s)\n", outcome.Email, outcome.UserID)
	if outcome.Created {
		fmt.Println("  the address held no account, so one was created")
	}
	if outcome.Reenabled {
		fmt.Println("  the account was disabled and is now enabled")
	}
	if outcome.FactorCleared {
		fmt.Println("  the second factor and its recovery codes were removed")
	}
	if outcome.PromotedToAdmin {
		fmt.Println("  the account did not hold admin and now does")
	}
	fmt.Printf("  %d live session(s) ended\n", outcome.SessionsEnded)
	fmt.Println("enrol a second factor again from /account before closing the incident")
}
