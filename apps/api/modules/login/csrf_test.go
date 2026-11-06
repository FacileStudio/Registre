package login

import (
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestEverySignInEndpointWantsTheCSRFHeader refuses a sign-in a cross-site
// page could submit, before any credential is read.
func TestEverySignInEndpointWantsTheCSRFHeader(t *testing.T) {
	f := newFixture(t)
	f.user("human@example.com", "correct horse battery staple")
	for _, path := range []string{"/api/login", "/api/login/totp", "/api/login/passkey/finish"} {
		response, err := http.Post(f.server.URL+path, "application/json",
			strings.NewReader(`{"email":"human@example.com","password":"correct horse battery staple"}`))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Errorf("POST %s without the header = %d, want 403", path, response.StatusCode)
		}
	}
}

// TestARefusedAppLeavesNoSession: a login an access policy refuses opens no
// session row that back-channel logout would later announce for nothing.
func TestARefusedAppLeavesNoSession(t *testing.T) {
	f := newFixture(t)
	user := f.user("human@example.com", "correct horse battery staple")
	f.db.Exec(`INSERT INTO clients (client_id, subject, kind, access_policy) VALUES ('app', 'app-sub', 'application', 'assigned')`)
	answer, response := f.login(f.authRequest(), "human@example.com", "correct horse battery staple")
	if response.StatusCode != http.StatusForbidden || answer["error"] != "no_access" {
		t.Fatalf("login to a restricted app = %d %v, want 403 no_access", response.StatusCode, answer)
	}
	var sessions int64
	f.db.Model(&schemas.Session{}).Where("user_id = ?", user.ID).Count(&sessions)
	if sessions != 0 {
		t.Fatalf("a refused login left %d session rows", sessions)
	}
}
