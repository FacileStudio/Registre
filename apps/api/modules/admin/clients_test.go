package admin

import (
	"net/http"
	"testing"
)

// TestAnApplicationIsRegisteredWithEverythingItNeedsToWork is the regression
// behind a gap found by driving the dashboard end to end.
//
// createClient used to accept only client_id, name, kind, grant_types and
// scopes, so every application registered through the dashboard arrived with
// no redirect URI and /authorize refused it with "the requested redirect_uri
// is missing in the client configuration". The row looked fine in the list and
// the app could never log anybody in. The same handler still drops nothing
// else that breaks an app quietly: visible_roles and backchannel_logout_uri
// ride along here for the same reason.
func TestAnApplicationIsRegisteredWithEverythingItNeedsToWork(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "sablier", "name": "Sablier", "kind": "application",
		"redirect_uris":          []string{"https://sablier.facile.studio/auth/oidc/callback"},
		"visible_roles":          []string{"sablier-admin"},
		"backchannel_logout_uri": "https://sablier.facile.studio/auth/backchannel-logout",
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status %d, want 201: %v", response.StatusCode, body)
	}

	client, ok := body["client"].(map[string]any)
	if !ok {
		t.Fatalf("no client in the response: %v", body)
	}
	uris, _ := client["redirect_uris"].([]any)
	if len(uris) != 1 {
		t.Errorf("the stored client has %d redirect URIs, want 1: %v", len(uris), client)
	}
	if client["backchannel_logout_uri"] != "https://sablier.facile.studio/auth/backchannel-logout" {
		t.Errorf("the back-channel logout URI did not survive: %v", client)
	}
	roles, _ := client["visible_roles"].([]any)
	if len(roles) != 1 {
		t.Errorf("visible_roles did not survive: %v", client)
	}
}

// TestAnApplicationWithoutARedirectURIIsRefused stops the dashboard minting a
// row that cannot finish a login, which is the shape of the bug above.
func TestAnApplicationWithoutARedirectURIIsRefused(t *testing.T) {
	f := newFixture(t)

	response, body := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "broken", "name": "Broken", "kind": "application",
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %v", response.StatusCode, body)
	}
	if reason, _ := body["error"].(string); reason == "" {
		t.Error("the refusal does not say what is missing")
	}
}

// TestAServiceAccountWithARedirectURIIsRefused catches the opposite mistake,
// a browser flow declared for a principal that never opens a browser.
func TestAServiceAccountWithARedirectURIIsRefused(t *testing.T) {
	f := newFixture(t)

	response, _ := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "agent", "name": "Agent", "kind": "service_account",
		"redirect_uris": []string{"https://example.test/callback"},
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", response.StatusCode)
	}
}

// TestGrantsDefaultToTheKindWhenTheCallerNamesNone covers the second half of
// the bug the fields above fixed. A client registered with no grant types was
// refused at /authorize with "the requested response type is missing in the
// client configuration", which names neither the row nor the field.
func TestGrantsDefaultToTheKindWhenTheCallerNamesNone(t *testing.T) {
	f := newFixture(t)

	_, body := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "defaults", "name": "Defaults", "kind": "application",
		"redirect_uris": []string{"https://defaults.example.test/callback"},
	}, true)
	client, ok := body["client"].(map[string]any)
	if !ok {
		t.Fatalf("no client in the response: %v", body)
	}
	grants := stringsOf(client["grant_types"])
	if len(grants) != 2 || grants[0] != "authorization_code" || grants[1] != "refresh_token" {
		t.Errorf("grants defaulted to %v, want the seed's authorization_code and refresh_token", grants)
	}
	if scopes := stringsOf(client["scopes"]); len(scopes) != 3 {
		t.Errorf("scopes defaulted to %v, want the seed's openid profile email", scopes)
	}
}

// TestAGrantTheKindCannotPerformIsRefused stops a caller registering a row the
// seed could never produce, which fails at /authorize exactly as an absent
// grant does.
func TestAGrantTheKindCannotPerformIsRefused(t *testing.T) {
	f := newFixture(t)

	response, _ := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "mismatched", "name": "Mismatched", "kind": "application",
		"redirect_uris": []string{"https://mismatched.example.test/callback"},
		"grant_types":   []string{"client_credentials"},
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status %d, want 400", response.StatusCode)
	}
}

// TestAMalformedURIIsRefused guards the field SPEC §6 lists as breaking
// silently: a typo in backchannel_logout_uri means deactivation quietly stops
// propagating and nothing errors anywhere.
func TestAMalformedURIIsRefused(t *testing.T) {
	f := newFixture(t)

	response, _ := f.call(t, http.MethodPost, "/api/admin/clients", map[string]any{
		"client_id": "typo", "name": "Typo", "kind": "application",
		"redirect_uris":          []string{"https://typo.example.test/callback"},
		"backchannel_logout_uri": "htps//typo.example.test/logout",
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status %d, want 400", response.StatusCode)
	}
}

// stringsOf reads a JSON array of strings out of a decoded response.
func stringsOf(raw any) []string {
	items, _ := raw.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
