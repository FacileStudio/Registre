package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// TestUserEraseTakesOneExactAddress pins the argument guard: anything but a
// single bare --email is refused before the database is touched.
func TestUserEraseTakesOneExactAddress(t *testing.T) {
	refused := [][]string{
		nil,
		{},
		{"--email"},
		{"--email", ""},
		{"--email", "a@example.com", "b@example.com"},
		{"--email", "a@example.com", "--email", "b@example.com", "extra"},
		{"--email", "a@example.com,b@example.com"},
		{"--email", "Somebody <a@example.com>"},
		{"--email", " a@example.com"},
		{"--email", "a@example.com\n"},
		{"--email", "*"},
		{"--name", "a@example.com"},
		{"--help"},
		{"a@example.com"},
	}
	for _, args := range refused {
		if email, ok := eraseTarget(args); ok {
			t.Errorf("eraseTarget(%q) accepted %q", args, email)
		}
	}
	for _, args := range [][]string{{"--email", "a@example.com"}, {"--email=a@example.com"}, {"-email", "a@example.com"}} {
		if email, ok := eraseTarget(args); !ok || email != "a@example.com" {
			t.Errorf("eraseTarget(%q) = %q, %v", args, email, ok)
		}
	}
}

// eraseFixture is a fresh schema holding two accounts and one suite app.
func eraseFixture(t *testing.T) (*gorm.DB, *identity.Service, *gdpr.Service) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil || testdb.Truncate(db) != nil {
		t.Fatalf("preparing the test database: %v", err)
	}
	for _, user := range []schemas.User{{ID: "gone", Email: "smoke@example.com"}, {ID: "kept", Email: "smoke2@example.com"}} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	if err := db.Create(&schemas.Client{ClientID: "app", Subject: "app", Kind: "application"}).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	users, rights := eraseServices(db, wireRoot(), wireConfig("https://sso.example.com"), services{verifier: credentials.New(1), log: log})
	return db, users, rights
}

// TestUserEraseErasesNowAndQueuesNotices erases one account through the
// sweeper's path, leaves its neighbour alone and answers no address.
func TestUserEraseErasesNowAndQueuesNotices(t *testing.T) {
	db, users, rights := eraseFixture(t)
	ctx := context.Background()
	if status, err := eraseUser(ctx, users, rights, "Smoke@example.com"); err != nil || status != "erased" {
		t.Fatalf("eraseUser = %q, %v", status, err)
	}
	var left []string
	db.Model(&schemas.User{}).Order("id").Pluck("id", &left)
	if strings.Join(left, ",") != "kept" {
		t.Fatalf("users after erase: %v", left)
	}
	var notices int64
	db.Table("deletion_notices").Where("subject = ? AND client_id = ?", "gone", "app").Count(&notices)
	if notices != 1 {
		t.Fatalf("deletion notices for the erased subject: %d, want 1", notices)
	}
	if status, err := eraseUser(ctx, users, rights, "smoke@example.com"); err == nil || status != "not found" {
		t.Fatalf("second eraseUser = %q, %v", status, err)
	}
}
