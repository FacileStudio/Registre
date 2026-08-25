package oidc

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The device grant is in the required list rather than merely tolerated,
// because zitadel/oidc decides whether to advertise it by type-asserting the
// storage for op.DeviceAuthorizationStorage. Rename either method and the
// advertisement disappears with no compile error anywhere: this assertion is
// the only thing that would say so.
func TestDiscoveryAdvertisesOnlyServedGrants(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())

	response, err := http.Get(fixture.issuer + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("fetching discovery: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("discovery answered %d", response.StatusCode)
	}
	var doc struct {
		GrantTypesSupported []string `json:"grant_types_supported"`
	}
	if err := json.NewDecoder(response.Body).Decode(&doc); err != nil {
		t.Fatalf("decoding the discovery document: %v", err)
	}
	served := make(map[string]bool)
	for _, grant := range doc.GrantTypesSupported {
		if !servedGrants[grant] {
			t.Errorf("discovery advertises %q, which no mounted endpoint honours", grant)
		}
		served[grant] = true
	}
	for _, grant := range []string{"authorization_code", "refresh_token", "client_credentials", deviceGrant} {
		if !served[grant] {
			t.Errorf("discovery omits %q, which registre serves", grant)
		}
	}
}
