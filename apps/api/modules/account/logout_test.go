package account

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// logouts records the apps each back-channel logout was fired to.
type logouts chan []string

// Fire records one logout.
func (l logouts) Fire(_ context.Context, _ string, clientIDs []string) error {
	l <- clientIDs
	return nil
}

func TestLoggingOutTellsEveryAppTheSessionReachedOnce(t *testing.T) {
	f := newFixture(t)
	err := f.db.Model(&schemas.Session{}).Where("user_id = ?", f.user).
		Update("client_ids", schemas.StringList{"opus"}).Error
	if err != nil {
		t.Fatalf("recording the app: %v", err)
	}

	for attempt := range 2 {
		response, _ := f.send(t, http.MethodPost, "/api/account/logout", nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("logout %d answered %d, want 200", attempt+1, response.StatusCode)
		}
	}
	select {
	case clients := <-f.fired:
		if len(clients) != 1 || clients[0] != "opus" {
			t.Fatalf("fired to %v, want [opus]", clients)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("logging out told no app")
	}
	select {
	case clients := <-f.fired:
		t.Fatalf("the dead cookie fired a second logout to %v", clients)
	case <-time.After(200 * time.Millisecond):
	}
}

// alive reports whether the fixture's cookie still opens the account.
func (f fixture) alive(t *testing.T) bool {
	t.Helper()
	response, _ := f.do(t, f.cookie(request(t, http.MethodGet, "/api/account/", nil)))
	return response.StatusCode == http.StatusOK
}

// quiet fails when any back-channel logout fired.
func (f fixture) quiet(t *testing.T) {
	t.Helper()
	select {
	case clients := <-f.fired:
		t.Fatalf("a refused logout told %v", clients)
	case <-time.After(200 * time.Millisecond):
	}
}

// reach records that the fixture's session signed in to opus.
func (f fixture) reach(t *testing.T) {
	t.Helper()
	err := f.db.Model(&schemas.Session{}).Where("user_id = ?", f.user).
		Update("client_ids", schemas.StringList{"opus"}).Error
	if err != nil {
		t.Fatalf("recording the app: %v", err)
	}
}

func TestALogoutFromAnotherSiteEndsNothing(t *testing.T) {
	f := newFixture(t)
	f.reach(t)
	for _, site := range []string{"cross-site", "none", ""} {
		req := f.cookie(request(t, http.MethodGet, "/api/account/logout", nil))
		req.Header.Set(accounts.FetchModeHeader, "navigate")
		if site != "" {
			req.Header.Set(accounts.FetchSiteHeader, site)
		}
		response, _ := f.do(t, req)
		if response.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("GET with Sec-Fetch-Site %q answered %d, want 405", site, response.StatusCode)
		}
	}
	response, _ := f.do(t, f.cookie(request(t, http.MethodPost, "/api/account/logout", nil)))
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("POST without the CSRF header answered %d, want 403", response.StatusCode)
	}
	if !f.alive(t) {
		t.Fatal("a refused logout ended the session")
	}
	f.quiet(t)
}

func TestALogoutFromASuiteAppEndsTheSession(t *testing.T) {
	f := newFixture(t)
	f.reach(t)
	req := f.cookie(request(t, http.MethodGet, "/api/account/logout", nil))
	req.Header.Set(accounts.FetchSiteHeader, "same-site")
	req.Header.Set(accounts.FetchModeHeader, "navigate")
	if response, _ := f.do(t, req); response.StatusCode != http.StatusOK {
		t.Fatalf("same-site GET answered %d, want 200", response.StatusCode)
	}
	if f.alive(t) {
		t.Fatal("a same-site logout left the session alive")
	}
	select {
	case clients := <-f.fired:
		if len(clients) != 1 || clients[0] != "opus" {
			t.Fatalf("fired to %v, want [opus]", clients)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a same-site logout told no app")
	}
}
