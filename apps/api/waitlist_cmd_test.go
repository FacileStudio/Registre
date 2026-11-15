package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/internal/waitlist"
)

func TestWaitlistArgs(t *testing.T) {
	if _, ok := parseWaitlistArgs("x", []string{"--product", "field"}, false); ok {
		t.Error("an unknown product was accepted")
	}
	if _, ok := parseWaitlistArgs("x", nil, true); ok {
		t.Error("delete without --email was accepted")
	}
	if _, ok := parseWaitlistArgs("x", []string{"--email", "a@example.com"}, false); ok {
		t.Error("export with --email was accepted")
	}
	if got, ok := parseWaitlistArgs("x", []string{"--email", "a@example.com", "--product", "scribe"}, true); !ok || got.product != "scribe" {
		t.Errorf("delete args: %+v %v", got, ok)
	}
}

// filledList is a list over a fresh schema holding one Scribe and one
// Pastille entry.
func filledList(t *testing.T) *waitlist.Service {
	t.Helper()
	dsn, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	list := waitlist.New(db, []byte("an-altcha-hmac-key-of-at-least-32-bytes"))
	three := 3.0
	for _, in := range []waitlist.Entry{
		{Email: "a@example.com", Product: "scribe", Locale: "de", Consent: "2026-11-14"},
		{Email: "b@example.com", Product: "pastille", Locale: "fr", Consent: "2026-11-14", Quantity: &three},
	} {
		if err := list.Join(context.Background(), in); err != nil {
			t.Fatalf("Join: %v", err)
		}
	}
	return list
}

func TestExportCarriesWorkingUnsubscribeLinks(t *testing.T) {
	list := filledList(t)
	ctx := context.Background()
	var out bytes.Buffer
	if err := exportWaitlist(ctx, &out, list, "https://sso.example.com", "scribe"); err != nil {
		t.Fatalf("export: %v", err)
	}
	records, err := csv.NewReader(&out).ReadAll()
	if err != nil || len(records) != 2 || records[1][0] != "a@example.com" || records[1][2] != "de" {
		t.Fatalf("records %v, %v", records, err)
	}
	if records[0][4] != "quantity" || records[1][4] != "" {
		t.Fatalf("scribe quantity column: header %q, cell %q", records[0][4], records[1][4])
	}
	link, err := url.Parse(records[1][6])
	if err != nil || !strings.HasPrefix(records[1][6], "https://sso.example.com"+UnsubscribePath+"?token=") {
		t.Fatalf("link %q", records[1][6])
	}
	if err := list.Unsubscribe(ctx, link.Query().Get("token")); err != nil {
		t.Fatalf("the exported link does not unsubscribe: %v", err)
	}
	if rows, _ := list.List(ctx, "scribe"); len(rows) != 0 {
		t.Fatal("the entry survived its unsubscribe link")
	}
}

func TestExportShowsThePastilleQuantity(t *testing.T) {
	list := filledList(t)
	var out bytes.Buffer
	if err := exportWaitlist(context.Background(), &out, list, "https://sso.example.com", "pastille"); err != nil {
		t.Fatalf("export: %v", err)
	}
	records, err := csv.NewReader(&out).ReadAll()
	if err != nil || len(records) != 2 || records[1][0] != "b@example.com" || records[1][4] != "3" {
		t.Fatalf("records %v, %v", records, err)
	}
}
