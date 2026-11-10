package spaces

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// invited is yann's space with one invitation to carol pending, and eve
// signed in beside her.
type invited struct {
	router      chi.Router
	db          *gorm.DB
	cookieName  string
	carol, eve  string
	carolID, id string
	space       string
}

func newInvited(t *testing.T) invited {
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
	users, sessions := identity.New(db, credentials.New(2)), accounts.New(db)
	logs := slog.New(slog.DiscardHandler)
	teams := directory.New(db, nil, nil, logs)
	cookies := accounts.NewCookies(false)
	yannID, _ := signIn(t, users, sessions, "yann@facile.studio")
	carolID, carol := signIn(t, users, sessions, "carol@facile.studio")
	_, eve := signIn(t, users, sessions, "eve@facile.studio")
	space, _ := teams.Create(context.Background(), yannID, "Alpha")
	if err := teams.Invite(context.Background(), yannID, space.ID, "carol@facile.studio", "admin"); err != nil {
		t.Fatal(err)
	}
	mine, _ := teams.Invited(context.Background(), carolID)
	if len(mine) != 1 {
		t.Fatalf("carol has %d invitations", len(mine))
	}
	router := chi.NewRouter()
	Mount(router, New(teams, sessions, cookies, logs))
	return invited{router: router, db: db, cookieName: cookies.Name(), carol: carol, eve: eve,
		carolID: carolID, id: mine[0].ID, space: space.ID}
}

// hit sends method to path with cookie, and the CSRF header when csrf.
func (w invited) hit(method, path, cookie string, csrf bool) (int, map[string]any) {
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: w.cookieName, Value: cookie})
	}
	if csrf {
		req.Header.Set(accounts.CSRFHeader, "1")
	}
	return serve(w.router, req)
}

// members counts carol's rows in the space.
func (w invited) members() int64 {
	var n int64
	w.db.Model(&schemas.SpaceMember{}).Where("space_id = ? AND user_id = ?", w.space, w.carolID).Count(&n)
	return n
}

// TestNoGETAcceptsAnInvitation follows the invitation's links the way a mail
// scanner would, signed in as the invitee, and expects her still outside the
// space with the invitation still pending.
func TestNoGETAcceptsAnInvitation(t *testing.T) {
	w := newInvited(t)
	base := "/api/spaces/invitations/" + w.id
	status, body := w.hit(http.MethodGet, base, w.carol, false)
	if status != http.StatusOK || body["space_name"] != "Alpha" || body["role"] != "admin" || body["invited_by"] != "yann@facile.studio" {
		t.Fatalf("GET the invitation = %d %v", status, body)
	}
	for _, path := range []string{base + "/accept", base + "/decline"} {
		if status, _ := w.hit(http.MethodGet, path, w.carol, true); status != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 405", path, status)
		}
	}
	if status, _ := w.hit(http.MethodPost, base+"/accept", w.carol, false); status != http.StatusForbidden {
		t.Errorf("POST accept without the CSRF header = %d, want 403", status)
	}
	if w.members() != 0 {
		t.Fatal("a GET or a forged POST made carol a member")
	}
	if status, body := w.hit(http.MethodGet, "/api/spaces/invitations", w.carol, false); status != http.StatusOK ||
		len(body["invitations"].([]any)) != 1 {
		t.Fatalf("the invitation is no longer pending: %d %v", status, body)
	}
	if status, _ := w.hit(http.MethodPost, base+"/accept", w.carol, true); status != http.StatusOK || w.members() != 1 {
		t.Fatalf("POST accept = %d, members %d", status, w.members())
	}
}

// TestTheInvitationPageStates answers each state with its own status, and
// nothing about the space to anyone but the invitee.
func TestTheInvitationPageStates(t *testing.T) {
	w := newInvited(t)
	base := "/api/spaces/invitations/" + w.id
	if status, _ := w.hit(http.MethodGet, base, "", false); status != http.StatusUnauthorized {
		t.Errorf("signed out = %d, want 401", status)
	}
	status, body := w.hit(http.MethodGet, base, w.eve, false)
	if status != http.StatusForbidden || body["space_name"] != nil {
		t.Errorf("the wrong account = %d %v, want 403 and no details", status, body)
	}
	if status, _ := w.hit(http.MethodPost, base+"/accept", w.eve, true); status != http.StatusNotFound || w.members() != 0 {
		t.Errorf("the wrong account accepting = %d, want 404", status)
	}
	w.db.Model(&schemas.SpaceInvitation{}).Where("id = ?", w.id).Update("expires_at", time.Now().Add(-time.Minute))
	if status, _ := w.hit(http.MethodGet, base, w.carol, false); status != http.StatusGone {
		t.Errorf("expired = %d, want 410", status)
	}
	if status, _ := w.hit(http.MethodPost, base+"/accept", w.carol, true); status != http.StatusNotFound || w.members() != 0 {
		t.Errorf("accepting an expired invitation = %d, want 404", status)
	}
	w.db.Where("id = ?", w.id).Delete(&schemas.SpaceInvitation{})
	if status, _ := w.hit(http.MethodGet, base, w.carol, false); status != http.StatusNotFound {
		t.Errorf("withdrawn = %d, want 404", status)
	}
}
