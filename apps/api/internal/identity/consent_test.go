package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestALoginWaitsForConsentToTheCurrentTerms holds Complete until the
// person accepts, and again after a version they accepted is replaced.
func TestALoginWaitsForConsentToTheCurrentTerms(t *testing.T) {
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
	consents := consent.New(db, []byte("k"))
	svc := New(db, credentials.New(2)).WithConsent(consents)
	ctx := context.Background()
	db.Create(&schemas.User{ID: "u", Email: "u@example.com"})
	pending := func(id string) {
		db.Create(&schemas.AuthRequest{ID: id, ClientID: "app", RedirectURI: "https://app/cb", ExpiresAt: time.Now().Add(time.Hour)})
	}
	pending("a1")
	if err := svc.Complete(ctx, "a1", "u", "s", time.Now()); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("Complete without consent = %v, want ErrConsentRequired", err)
	}
	if err := consents.Record(ctx, nil, "u", "en", "198.51.100.1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := svc.Complete(ctx, "a1", "u", "s", time.Now()); err != nil {
		t.Fatalf("Complete after consent = %v", err)
	}
	db.Model(&schemas.Consent{}).Where("user_id = ?", "u").Update("version", "2000-01-01")
	pending("a2")
	if err := svc.Complete(ctx, "a2", "u", "s", time.Now()); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("Complete after a version bump = %v, want ErrConsentRequired", err)
	}
}
