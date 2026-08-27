package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"

	"github.com/go-chi/chi/v5"
)

type cookieFixture struct {
	router      chi.Router
	adminToken  string
	normalToken string
}

func makeUserSession(t *testing.T, u *identity.Service, s *accounts.Service, email string, admin bool) string {
	t.Helper()
	user, err := u.Create(context.Background(), identity.NewUser{Email: email, Password: "pw", Admin: admin})
	if err != nil {
		t.Fatalf("creating user %s: %v", email, err)
	}
	sid, err := u.StartSession(context.Background(), user.ID, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("starting session for %s: %v", email, err)
	}
	token, err := s.Issue(context.Background(), sid)
	if err != nil {
		t.Fatalf("issuing token for %s: %v", email, err)
	}
	return token
}

func setupCookieFixture(t *testing.T) cookieFixture {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	verifier := credentials.New(2)
	users := identity.New(db, verifier)
	sessions := accounts.New(db)
	keyring := keys.New(db, root, keys.Config{
		RotateEvery: time.Hour, PublishDelay: 10 * time.Minute, Retention: 2 * time.Hour,
	}, discard())
	if err := keyring.Ensure(context.Background()); err != nil {
		t.Fatalf("ensuring keyring: %v", err)
	}
	service := New(Services{
		Register: clients.New(db, verifier), Users: users, Keys: keyring,
		Invites: invites.New(db, verifier), Sessions: sessions, Cookies: accounts.NewCookies(false),
	}, discard())
	router := chi.NewRouter()
	Mount(router, service)

	adminToken := makeUserSession(t, users, sessions, "admin@example.com", true)
	normalToken := makeUserSession(t, users, sessions, "user@example.com", false)

	return cookieFixture{router: router, adminToken: adminToken, normalToken: normalToken}
}

func TestAdminCookieAuthGet(t *testing.T) {
	f := setupCookieFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: accounts.DevCookieName, Value: f.adminToken})
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/users with admin cookie = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: accounts.DevCookieName, Value: f.normalToken})
	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/admin/users with normal cookie = %d, want 401", rec.Code)
	}
}

func TestAdminCookieAuthPost(t *testing.T) {
	f := setupCookieFixture(t)
	body, _ := json.Marshal(map[string]string{"email": "invited@example.com"})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/invites", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: accounts.DevCookieName, Value: f.adminToken})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /api/admin/invites without CSRF = %d, want 403", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/invites", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: accounts.DevCookieName, Value: f.adminToken})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(accounts.CSRFHeader, "1")
	rec = httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/admin/invites with CSRF = %d, want 201", rec.Code)
	}
}
