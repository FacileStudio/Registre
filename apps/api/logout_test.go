package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// firedLogouts records the apps each back-channel logout was fired to.
type firedLogouts chan []string

// Fire records one logout.
func (f firedLogouts) Fire(_ context.Context, _ string, clientIDs []string) error {
	f <- clientIDs
	return nil
}

// logoutFixture is the logout routes over a live session that reached opus.
type logoutFixture struct {
	router   chi.Router
	sessions *accounts.Service
	cookies  accounts.Cookies
	token    string
	fired    firedLogouts
}

// confirmPage stands in for the client build: it marks what it served.
const confirmPage = "confirm"

func newLogoutFixture(t *testing.T) logoutFixture {
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
	sessionID := openSession(t, db)
	ctx := context.Background()
	fired := make(firedLogouts, 8)
	sessions := accounts.New(db).WithNotifier(fired, nil)
	token, err := sessions.Issue(ctx, sessionID)
	if err != nil {
		t.Fatalf("issuing the cookie: %v", err)
	}
	cookies := accounts.NewCookies(false)
	router := chi.NewRouter()
	page := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(confirmPage)) })
	mountLogout(router, sessions, cookies, page)
	return logoutFixture{router, sessions, cookies, token, fired}
}

// openSession creates an account and a provider session that reached opus.
func openSession(t *testing.T, db *gorm.DB) string {
	t.Helper()
	ctx := context.Background()
	users := identity.New(db, credentials.New(1))
	user, err := users.Create(ctx, identity.NewUser{Email: "out@example.com", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	sessionID, err := users.StartSession(ctx, user.ID, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("opening the session: %v", err)
	}
	if err := db.Model(&schemas.Session{}).Where("id = ?", sessionID).Update("client_ids", schemas.StringList{"opus"}).Error; err != nil {
		t.Fatalf("recording the app: %v", err)
	}
	return sessionID
}

// do sends method to path with the session cookie and headers.
func (f logoutFixture) do(method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: f.cookies.Name(), Value: f.token})
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req)
	return recorder
}

// alive reports whether the session cookie still resolves.
func (f logoutFixture) alive() bool {
	_, _, err := f.sessions.Resolve(context.Background(), f.token)
	return err == nil
}

// firedOnce fails unless exactly one back-channel logout reached opus.
func (f logoutFixture) firedOnce(t *testing.T) {
	t.Helper()
	select {
	case clients := <-f.fired:
		if len(clients) != 1 || clients[0] != "opus" {
			t.Fatalf("fired to %v, want [opus]", clients)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the logout told no app")
	}
}

func TestACrossSiteLogoutLinkAsksInsteadOfSigningOut(t *testing.T) {
	f := newLogoutFixture(t)
	for _, site := range []string{"cross-site", "none", ""} {
		headers := map[string]string{accounts.FetchModeHeader: "navigate"}
		if site != "" {
			headers[accounts.FetchSiteHeader] = site
		}
		if page := f.do(http.MethodGet, "/logout", headers); page.Code != http.StatusOK || page.Body.String() != confirmPage {
			t.Fatalf("GET /logout with Sec-Fetch-Site %q answered %d %q, want the confirmation page", site, page.Code, page.Body.String())
		}
		if api := f.do(http.MethodGet, "/api/logout", headers); api.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET /api/logout with Sec-Fetch-Site %q answered %d, want 405", site, api.Code)
		}
	}
	image := map[string]string{accounts.FetchSiteHeader: "same-site", accounts.FetchModeHeader: "no-cors"}
	if page := f.do(http.MethodGet, "/logout", image); page.Body.String() != confirmPage {
		t.Fatalf("a same-site image answered %d %q, want the confirmation page", page.Code, page.Body.String())
	}
	for _, path := range []string{"/logout", "/api/logout"} {
		if forged := f.do(http.MethodPost, path, map[string]string{accounts.FetchSiteHeader: "same-origin"}); forged.Code != http.StatusForbidden {
			t.Fatalf("POST %s without the CSRF header answered %d, want 403", path, forged.Code)
		}
	}
	if !f.alive() {
		t.Fatal("a refused logout ended the session")
	}
	select {
	case clients := <-f.fired:
		t.Fatalf("a refused logout told %v", clients)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestASuiteAppSignOutRedirectEndsTheSession(t *testing.T) {
	f := newLogoutFixture(t)
	response := f.do(http.MethodGet, "/logout", map[string]string{accounts.FetchSiteHeader: "same-site", accounts.FetchModeHeader: "navigate"})
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/" {
		t.Fatalf("same-site GET /logout answered %d to %q, want 302 to /", response.Code, response.Header().Get("Location"))
	}
	if f.alive() {
		t.Fatal("a same-site logout left the session alive")
	}
	f.firedOnce(t)
	if again := f.do(http.MethodGet, "/logout", map[string]string{accounts.FetchSiteHeader: "same-origin", accounts.FetchModeHeader: "navigate"}); again.Code != http.StatusFound {
		t.Fatalf("logging out a dead cookie answered %d, want 302", again.Code)
	}
}

func TestACSRFPostEndsTheSession(t *testing.T) {
	f := newLogoutFixture(t)
	response := f.do(http.MethodPost, "/logout", map[string]string{accounts.CSRFHeader: "1"})
	if response.Code != http.StatusOK {
		t.Fatalf("POST /logout with the CSRF header answered %d, want 200", response.Code)
	}
	if f.alive() {
		t.Fatal("a CSRF POST left the session alive")
	}
	f.firedOnce(t)
}
