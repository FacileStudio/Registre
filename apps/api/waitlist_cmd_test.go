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
	for _, in := range []waitlist.Entry{
		{Email: "a@example.com", Product: "scribe", Locale: "de", Consent: "2026-11-14"},
		{Email: "b@example.com", Product: "pastille", Locale: "fr", Consent: "2026-11-14"},
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
	link, err := url.Parse(records[1][5])
	if err != nil || !strings.HasPrefix(records[1][5], "https://sso.example.com"+UnsubscribePath+"?token=") {
		t.Fatalf("link %q", records[1][5])
	}
	if err := list.Unsubscribe(ctx, link.Query().Get("token")); err != nil {
		t.Fatalf("the exported link does not unsubscribe: %v", err)
	}
	if rows, _ := list.List(ctx, "scribe"); len(rows) != 0 {
		t.Fatal("the entry survived its unsubscribe link")
	}
}
