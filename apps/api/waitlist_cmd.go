package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/waitlist"

	"gorm.io/gorm"
)

// UnsubscribePath is the page an unsubscribe link opens on the issuer.
const UnsubscribePath = "/waitlist/unsubscribe"

// waitlistArgs is what the waitlist subcommands take.
type waitlistArgs struct {
	product string
	email   string
}

// runWaitlistExport answers `registre waitlist-export [--product p]`: the
// list as CSV on stdout, one row per entry with its unsubscribe link, for
// the owner to load into the launch mail. Nothing is logged about the rows.
func runWaitlistExport(ctx context.Context, db *gorm.DB, _ *credentials.Verifier, log *slog.Logger) int {
	args, ok := parseWaitlistArgs("waitlist-export", os.Args[2:], false)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: registre waitlist-export [--product scribe|pastille]")
		return 2
	}
	list, issuer, err := waitlistService(db)
	if err != nil {
		log.Error("registre: waitlist-export failed", slog.Any("error", err))
		return 1
	}
	if err := exportWaitlist(ctx, os.Stdout, list, issuer, args.product); err != nil {
		log.Error("registre: waitlist-export failed", slog.Any("error", err))
		return 1
	}
	return 0
}

// runWaitlistDelete answers `registre waitlist-delete --email <address>
// [--product p]`, the delete-on-request path. It prints only a count.
func runWaitlistDelete(ctx context.Context, db *gorm.DB, _ *credentials.Verifier, log *slog.Logger) int {
	args, ok := parseWaitlistArgs("waitlist-delete", os.Args[2:], true)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: registre waitlist-delete --email <address> [--product scribe|pastille]")
		return 2
	}
	deleted, err := waitlist.New(db, nil).Delete(ctx, args.email, args.product)
	if err != nil {
		log.Error("registre: waitlist-delete failed", slog.Any("error", err))
		return 1
	}
	fmt.Printf("deleted %d\n", deleted)
	return 0
}

// parseWaitlistArgs reads --product and, when needEmail, a required --email.
func parseWaitlistArgs(name string, raw []string, needEmail bool) (waitlistArgs, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	product := fs.String("product", "", "scribe or pastille; empty means every list")
	email := fs.String("email", "", "the address to delete")
	if fs.Parse(raw) != nil || fs.NArg() != 0 {
		return waitlistArgs{}, false
	}
	if *product != "" && !slices.Contains(waitlist.Products(), *product) {
		return waitlistArgs{}, false
	}
	if needEmail != (strings.TrimSpace(*email) != "") {
		return waitlistArgs{}, false
	}
	return waitlistArgs{product: *product, email: *email}, true
}

// waitlistService builds the list over the running instance's ALTCHA key,
// the key its unsubscribe tokens are derived from.
func waitlistService(db *gorm.DB) (*waitlist.Service, string, error) {
	cfg, err := env.Load()
	if err != nil {
		return nil, "", fmt.Errorf("load the configuration: %w", err)
	}
	settings, err := env.LoadSignup(cfg.Issuer, cfg.InsecureIssuer())
	if err != nil {
		return nil, "", err
	}
	if len(settings.AltchaKey) < env.MinAltchaKeyLength {
		return nil, "", fmt.Errorf("ALTCHA_HMAC_KEY is missing or shorter than %d bytes", env.MinAltchaKeyLength)
	}
	return waitlist.New(db, []byte(settings.AltchaKey)), strings.TrimRight(cfg.Issuer, "/"), nil
}

// exportWaitlist writes one product's entries, or all of them, as CSV.
func exportWaitlist(ctx context.Context, out io.Writer, list *waitlist.Service, issuer, product string) error {
	rows, err := list.List(ctx, product)
	if err != nil {
		return err
	}
	records := [][]string{{"email", "product", "locale", "consent_version", "created_at", "unsubscribe_url"}}
	for _, row := range rows {
		link := issuer + UnsubscribePath + "?token=" + url.QueryEscape(list.Token(row.ID))
		records = append(records, []string{
			row.Email, row.Product, row.Locale, row.ConsentVersion, row.CreatedAt.UTC().Format(time.RFC3339), link,
		})
	}
	writer := csv.NewWriter(out)
	if err := writer.WriteAll(records); err != nil {
		return fmt.Errorf("write the CSV: %w", err)
	}
	return writer.Error()
}
