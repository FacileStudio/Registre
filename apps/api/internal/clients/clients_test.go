package clients

import (
	"context"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// register opens a test database and returns the register over it.
func register(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return New(db, credentials.New(2)), db
}

// TestUpsertWithAPartialRowBlanksTheDeclarativeLists documents the contract,
// on purpose: Upsert replaces the whole declarative half of the row, so a
// caller that skips the read-modify-write and hands back a half-built struct
// loses redirect_uris, backchannel_logout_uri and visible_roles — the same
// failure class as the historical disabled_at wipe, guarded here by stating
// rather than fixing the behavior. The secret is the deliberate exception:
// an empty SecretHash means "keep the existing one", so a careless partial
// update must not lock every app out of its credential.
func TestUpsertWithAPartialRowBlanksTheDeclarativeLists(t *testing.T) {
	store, db := register(t)
	ctx := context.Background()

	hash, err := store.HashSecret(ctx, "the-real-secret")
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	full := schemas.Client{
		ClientID: "courrier", Subject: "sub-courrier", Name: "Courrier",
		Kind:                 schemas.KindApplication,
		SecretHash:           hash,
		GrantTypes:           schemas.StringList{"authorization_code"},
		Scopes:               schemas.StringList{"openid"},
		Audiences:            schemas.StringList{"courrier"},
		RedirectURIs:         schemas.StringList{"https://courrier.facile.studio/callback"},
		BackchannelLogoutURI: "https://courrier.facile.studio/logout",
		VisibleRoles:         schemas.StringList{"admin", "member"},
	}
	if _, err := store.Upsert(ctx, full); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	careless := full
	careless.SecretHash = ""
	careless.GrantTypes = nil
	careless.Scopes = nil
	careless.Audiences = nil
	careless.RedirectURIs = nil
	careless.BackchannelLogoutURI = ""
	careless.VisibleRoles = nil
	if _, err := store.Upsert(ctx, careless); err != nil {
		t.Fatalf("partial upsert: %v", err)
	}

	row, err := store.rawByClientID(ctx, "courrier")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if _, authErr := store.Authenticate(ctx, "courrier", "the-real-secret"); authErr != nil {
		t.Errorf("the partial upsert destroyed the secret: %v", authErr)
	}
	if len(row.RedirectURIs) != 0 || len(row.VisibleRoles) != 0 || len(row.Scopes) != 0 {
		t.Errorf("expected the declarative lists to be blanked per the contract, got "+
			"redirect_uris %v visible_roles %v scopes %v",
			row.RedirectURIs, row.VisibleRoles, row.Scopes)
	}
	if row.BackchannelLogoutURI != "" {
		t.Errorf("backchannel_logout_uri = %q, expected blanked per the contract", row.BackchannelLogoutURI)
	}

	var raw schemas.Client
	if err := db.Where("client_id = ?", "courrier").Take(&raw).Error; err != nil {
		t.Fatalf("reading the raw row: %v", err)
	}
	if raw.Subject != "sub-courrier" {
		t.Errorf("the subject moved to %q across a re-apply", raw.Subject)
	}
}
