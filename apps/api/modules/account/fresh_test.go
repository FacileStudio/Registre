package account

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"

	"gorm.io/gorm"
)

// keysFor is a relying party for the fixture.
func keysFor(t *testing.T, db *gorm.DB) *passkeys.Service {
	t.Helper()
	keys, err := passkeys.New(db, passkeys.Config{RPID: "localhost", Origins: []string{"http://localhost"}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("passkeys: %v", err)
	}
	return keys
}

// TestAPasswordChangeCostsTheSecondFactorAndProvesNothing: with two-step
// sign-in on, the change wants a code, and the session it reissues is not
// fresh, so it cannot add a passkey or ask for deletion without proving a
// factor again.
func TestAPasswordChangeCostsTheSecondFactorAndProvesNothing(t *testing.T) {
	f := newFixture(t)
	codes := f.enrol(t)
	path := "/api/account/password"
	body := map[string]string{"current_password": holderPassword, "new_password": "a replacement password"}
	if response, _ := f.send(t, http.MethodPost, path, body); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a change without the code answered %d, want 401", response.StatusCode)
	}
	body["code"] = codes[0]
	response, answer := f.send(t, http.MethodPost, path, body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("a change with a recovery code answered %d: %v", response.StatusCode, answer)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == f.cookies.Name() && cookie.Value != "" {
			f.token = cookie.Value
		}
	}
	response, answer = f.send(t, http.MethodPost, "/api/account/passkeys/begin", nil)
	if response.StatusCode != http.StatusUnauthorized || answer["error"] != "reauth_required" {
		t.Fatalf("adding a passkey from the reissued session = %d %v, want 401 reauth_required", response.StatusCode, answer)
	}
}
