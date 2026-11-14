package waitlist

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
)

func newService(t *testing.T) *Service {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return New(db, []byte("a-waitlist-test-key-of-32-bytes!!"))
}

func entry(email string) Entry {
	return Entry{Email: email, Product: "scribe", Locale: "fr", Consent: "2026-11-14"}
}

func TestValidateRefusesEachBadField(t *testing.T) {
	cases := []struct {
		name string
		in   Entry
		want error
	}{
		{"empty email", entry(""), ErrEmail},
		{"no at", entry("person.example.com"), ErrEmail},
		{"no dot in domain", entry("person@localhost"), ErrEmail},
		{"display name", entry("Person <person@example.com>"), ErrEmail},
		{"too long", entry(strings.Repeat("a", 250) + "@example.com"), ErrEmail},
		{"wrong product", Entry{Email: "p@example.com", Product: "field", Locale: "fr", Consent: "v1"}, ErrProduct},
		{"wrong locale", Entry{Email: "p@example.com", Product: "scribe", Locale: "it", Consent: "v1"}, ErrLocale},
		{"no consent", Entry{Email: "p@example.com", Product: "scribe", Locale: "fr"}, ErrConsent},
	}
	for _, tc := range cases {
		if _, err := Validate(tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
	got, err := Validate(entry("  Person@Example.COM "))
	if err != nil || got.Email != "person@example.com" {
		t.Fatalf("normalised: got %q, %v", got.Email, err)
	}
}

func TestJoinKeepsOneRowPerAddressAndProduct(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	for _, email := range []string{"person@example.com", "PERSON@example.com"} {
		if err := svc.Join(ctx, entry(email)); err != nil {
			t.Fatalf("Join(%s): %v", email, err)
		}
	}
	other := entry("person@example.com")
	other.Product = "pastille"
	if err := svc.Join(ctx, other); err != nil {
		t.Fatalf("Join pastille: %v", err)
	}
	scribe, _ := svc.List(ctx, "scribe")
	all, _ := svc.List(ctx, "")
	if len(scribe) != 1 || len(all) != 2 {
		t.Fatalf("got %d scribe rows and %d in all, want 1 and 2", len(scribe), len(all))
	}
}

func TestUnsubscribeDeletesTheEntryAndIsRepeatable(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	if err := svc.Join(ctx, entry("person@example.com")); err != nil {
		t.Fatalf("Join: %v", err)
	}
	rows, _ := svc.List(ctx, "scribe")
	token := svc.Token(rows[0].ID)
	for range 2 {
		if err := svc.Unsubscribe(ctx, token); err != nil {
			t.Fatalf("Unsubscribe: %v", err)
		}
	}
	if rows, _ = svc.List(ctx, ""); len(rows) != 0 {
		t.Fatalf("%d rows left", len(rows))
	}
}

func TestUnsubscribeRefusesForgedTokens(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	if err := svc.Join(ctx, entry("person@example.com")); err != nil {
		t.Fatalf("Join: %v", err)
	}
	rows, _ := svc.List(ctx, "scribe")
	other := New(svc.db, []byte("another-key-another-key-another!!"))
	for _, token := range []string{"", rows[0].ID, rows[0].ID + ".", other.Token(rows[0].ID), strings.Repeat("a", 300)} {
		if err := svc.Unsubscribe(ctx, token); !errors.Is(err, ErrToken) {
			t.Errorf("token %q: got %v, want ErrToken", token, err)
		}
	}
	if rows, _ = svc.List(ctx, ""); len(rows) != 1 {
		t.Fatalf("%d rows left, want 1", len(rows))
	}
}

func TestDeleteByAddress(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()
	other := entry("person@example.com")
	other.Product = "pastille"
	for _, in := range []Entry{entry("person@example.com"), other} {
		if err := svc.Join(ctx, in); err != nil {
			t.Fatalf("Join: %v", err)
		}
	}
	if n, err := svc.Delete(ctx, " Person@example.com", "scribe"); err != nil || n != 1 {
		t.Fatalf("Delete scribe: %d, %v", n, err)
	}
	if n, err := svc.Delete(ctx, "person@example.com", ""); err != nil || n != 1 {
		t.Fatalf("Delete all: %d, %v", n, err)
	}
}
