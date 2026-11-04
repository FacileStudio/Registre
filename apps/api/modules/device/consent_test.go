package device

import (
	"context"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
)

// TestApprovalWaitsForConsent holds a device approval exactly where a
// browser sign-in is held while the current terms are owed.
func TestApprovalWaitsForConsent(t *testing.T) {
	owed := true
	fixture := newFixtureWith(t, func(context.Context, string) error {
		if owed {
			return identity.ErrConsentRequired
		}
		return nil
	})
	fixture.signIn()
	fixture.start("BCDFGHJK")
	body, status := fixture.call(http.MethodPost, "/api/device/approve", `{"user_code":"bcdf-ghjk"}`)
	if status != http.StatusForbidden || body["error"] != "consent_required" {
		t.Fatalf("approve while consent is owed = %d %v, want 403 consent_required", status, body)
	}
	owed = false
	if _, status := fixture.call(http.MethodPost, "/api/device/approve", `{"user_code":"bcdf-ghjk"}`); status != http.StatusOK {
		t.Fatalf("approve after consent = %d, want 200", status)
	}
}
