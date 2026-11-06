package spaces

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/billing"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"

	"github.com/go-chi/chi/v5"
)

// token is the shared bearer the fake Ardoise expects.
const token = "a-billing-token-that-is-long-enough-to-count"

// ardoise is a fake billing API. redirect is what checkout and portal
// answer; checkouts records every checkout body it received.
type ardoise struct {
	mu        sync.Mutex
	redirect  string
	down      bool
	catalogs  int
	checkouts []billing.CheckoutRequest
	portals   []map[string]string
}

// ServeHTTP answers the four routes registre calls.
func (a *ardoise) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.down || r.Header.Get("Authorization") != "Bearer "+token {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	switch r.URL.Path {
	case "/api/catalog":
		a.catalogs++
		_, _ = w.Write([]byte(`{"plans":[{"key":"pro","name":"Pro","prices":[{"interval":"month","amount":1200,"currency":"eur","lookup_key":"plan_pro_month"}]}]}`))
	case "/billing/checkout":
		var body billing.CheckoutRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		a.checkouts = append(a.checkouts, body)
		_ = json.NewEncoder(w).Encode(map[string]string{"url": a.redirect})
	case "/billing/portal":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		a.portals = append(a.portals, body)
		_ = json.NewEncoder(w).Encode(map[string]string{"url": a.redirect})
	default:
		_, _ = w.Write([]byte(`{"plan":"pro","status":"active","comped":false}`))
	}
}

// set changes the fake under its lock.
func (a *ardoise) set(change func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	change()
}

// world is one space owned by yann with bob as a member, behind a router
// whose plan section talks to the fake.
type world struct {
	on, off    chi.Router
	fake       *ardoise
	space      string
	yann, bob  string
	cookieName string
}

func newWorld(t *testing.T) *world {
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
	yannID, yann := signIn(t, users, sessions, "yann@facile.studio")
	bobID, bob := signIn(t, users, sessions, "bob@facile.studio")
	space, _ := teams.Create(context.Background(), yannID, "Alpha")
	joinAs(t, teams, yannID, space.ID, bobID)
	fake := &ardoise{redirect: "https://checkout.stripe.com/c/pay/cs_test"}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	w := &world{fake: fake, space: space.ID, yann: yann, bob: bob,
		cookieName: cookies.Name(), on: chi.NewRouter(), off: chi.NewRouter()}
	Mount(w.on, New(teams, sessions, cookies, logs).WithBilling(billing.New(server.URL, token), "https://sso.test"))
	Mount(w.off, New(teams, sessions, cookies, logs))
	return w
}

// signIn creates a user and answers their subject and a live cookie value.
func signIn(t *testing.T, users *identity.Service, sessions *accounts.Service, email string) (string, string) {
	t.Helper()
	ctx := context.Background()
	user, err := users.Create(ctx, identity.NewUser{Email: email, Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("creating %s: %v", email, err)
	}
	sessionID, _ := users.StartSession(ctx, user.ID, "", time.Now().UTC())
	cookie, err := sessions.Issue(ctx, sessionID)
	if err != nil {
		t.Fatalf("signing %s in: %v", email, err)
	}
	return user.ID, cookie
}

// request builds what the page sends: the cookie and the CSRF header.
func (w *world) request(method, path, cookie string, body any) *http.Request {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, "/api/spaces/"+w.space+path, bytes.NewReader(raw))
	req.AddCookie(&http.Cookie{Name: w.cookieName, Value: cookie})
	req.Header.Set(accounts.CSRFHeader, "1")
	return req
}

// serve runs req and answers the status and the decoded body.
func serve(router chi.Router, req *http.Request) (int, map[string]any) {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	var out map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &out)
	return recorder.Code, out
}

// call sends one page request through router.
func (w *world) call(router chi.Router, method, path, cookie string, body any) (int, map[string]any) {
	return serve(router, w.request(method, path, cookie, body))
}

// joinAs invites bob@facile.studio into spaceID as admin and accepts as
// bobID.
func joinAs(t *testing.T, teams *directory.Service, ownerID, spaceID, bobID string) {
	t.Helper()
	if err := teams.Invite(context.Background(), ownerID, spaceID, "bob@facile.studio", "admin"); err != nil {
		t.Fatalf("inviting bob: %v", err)
	}
	invited, _ := teams.Invited(context.Background(), bobID)
	if len(invited) != 1 {
		t.Fatalf("bob has %d invitations, want 1", len(invited))
	}
	if _, err := teams.Accept(context.Background(), bobID, invited[0].ID); err != nil {
		t.Fatalf("bob accepting: %v", err)
	}
}
