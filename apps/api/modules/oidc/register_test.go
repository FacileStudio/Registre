package oidc

import (
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// registerClient posts a registration body and decodes the answer.
func (f *fixture) registerClient(body string) (map[string]any, int) {
	f.t.Helper()
	request, _ := http.NewRequest(http.MethodPost, f.issuer+RegistrationPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return decode[map[string]any](f.t, request)
}

func TestDynamicRegistrationMakesAPublicThirdPartyClient(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	answer, status := fixture.registerClient(`{"client_name":"Cursor","redirect_uris":["cursor://anysphere.cursor-retrieval/oauth/callback"]}`)
	if status != http.StatusCreated {
		t.Fatalf("POST /register = %d %v", status, answer)
	}
	if answer["token_endpoint_auth_method"] != "none" || answer["application_type"] != "native" || answer["client_secret"] != nil {
		t.Errorf("registration answered %v", answer)
	}
	clientID, _ := answer["client_id"].(string)
	row, err := fixture.register.ByClientID(t.Context(), clientID)
	if err != nil || !row.Public || row.Registration != schemas.RegistrationDynamic || row.AccessTokenTTL != 600 {
		t.Errorf("stored row = %+v (%v)", row, err)
	}
	doc := fetchDocument(t, fixture.issuer+MetadataPath)
	if doc["registration_endpoint"] != fixture.issuer+RegistrationPath || doc["client_id_metadata_document_supported"] != true {
		t.Errorf("metadata advertises registration %v and CIMD %v", doc["registration_endpoint"], doc["client_id_metadata_document_supported"])
	}
}

func TestDynamicRegistrationRefusesWhatItCannotHonour(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	for body, want := range map[string]string{
		`not json`: "invalid_client_metadata",
		`{"redirect_uris":["https://a.test/cb"],"token_endpoint_auth_method":"client_secret_basic"}`: "invalid_client_metadata",
		`{"redirect_uris":["http://evil.test/cb"]}`:                                                  "invalid_redirect_uri",
		`{"redirect_uris":[]}`: "invalid_redirect_uri",
		`{"redirect_uris":["https://a.test/cb"],"grant_types":["client_credentials"]}`: "invalid_client_metadata",
	} {
		answer, status := fixture.registerClient(body)
		if status != http.StatusBadRequest || answer["error"] != want {
			t.Errorf("%s = %d %v, want %s", body, status, answer, want)
		}
		if description, _ := answer["error_description"].(string); strings.Contains(description, "\n") {
			t.Errorf("the description %q spans lines", description)
		}
	}
}
