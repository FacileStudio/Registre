package oidc

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestAMachineTokenVerifiesAgainstThePublishedKeySet is SPEC §4 phase 0 in one
// assertion: a service account authenticates with its own credentials and gets
// a JWT that verifies against the JWKS registre publishes, carrying the subject
// and audience an app would authorize on.
func TestAMachineTokenVerifiesAgainstThePublishedKeySet(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.account("agent-courrier", []string{"courrier.read"}, []string{"courrier"})

	granted, status := fixture.token("agent-courrier", secret, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s: %s", status, granted.Error, granted.Description)
	}
	if granted.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", granted.TokenType)
	}

	claims, err := fixture.verify(granted.AccessToken)
	if err != nil {
		t.Fatalf("the published key set does not verify the token registre just signed: %v", err)
	}
	if claims["iss"] != fixture.issuer {
		t.Errorf("iss = %v, want %s", claims["iss"], fixture.issuer)
	}
	if claims["client_id"] != "agent-courrier" {
		t.Errorf("client_id = %v, want agent-courrier", claims["client_id"])
	}
	if audience := only(claims["aud"]); audience != "courrier" {
		t.Errorf("aud = %v, want courrier", claims["aud"])
	}
	if claims["sub"] == claims["client_id"] {
		t.Error("sub is the client id; a service account must carry a subject of its own")
	}
}

// TestTheAudienceIsTheRegistersNotTheRequests proves an agent cannot name its
// own audience. The register decides which apps a credential works at, and a
// token good everywhere is the thing service accounts exist to avoid.
func TestTheAudienceIsTheRegistersNotTheRequests(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.account("agent-narrow", []string{"courrier.read"}, []string{"courrier"})

	form := url.Values{"grant_type": {"client_credentials"}, "audience": {"casier"}, "resource": {"casier"}}
	request, err := http.NewRequest(http.MethodPost, fixture.issuer+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth("agent-narrow", secret)
	granted, status := decode[tokenResponse](t, request)
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s: %s", status, granted.Error, granted.Description)
	}

	claims, err := fixture.verify(granted.AccessToken)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if audience := only(claims["aud"]); audience != "courrier" {
		t.Errorf("aud = %v, want courrier — the request asked for casier", claims["aud"])
	}
}

func TestTheTokenEndpointRefusesABadCredential(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.account("agent-real", []string{"courrier.read"}, []string{"courrier"})

	for name, attempt := range map[string][2]string{
		"a wrong secret":    {"agent-real", "not-the-secret"},
		"an unknown client": {"agent-imaginary", secret},
		"an empty secret":   {"agent-real", ""},
	} {
		t.Run(name, func(t *testing.T) {
			granted, status := fixture.token(attempt[0], attempt[1], "")
			if status == http.StatusOK {
				t.Fatalf("%s was issued a token", name)
			}
			if granted.AccessToken != "" {
				t.Errorf("%s got an access token alongside status %d", name, status)
			}
		})
	}
}

func TestADisabledClientStopsGettingTokensWithoutARestart(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.account("agent-retired", []string{"courrier.read"}, []string{"courrier"})

	if _, status := fixture.token("agent-retired", secret, ""); status != http.StatusOK {
		t.Fatalf("the account could not get a token before being disabled: %d", status)
	}
	disabled := fixture.clock.now()
	err := fixture.db.Model(&schemas.Client{}).
		Where("client_id = ?", "agent-retired").Update("disabled_at", disabled).Error
	if err != nil {
		t.Fatalf("disabling: %v", err)
	}
	if _, status := fixture.token("agent-retired", secret, ""); status == http.StatusOK {
		t.Fatal("a disabled account was still issued a token")
	}
}

func TestAScopeTheRegisterDoesNotGrantIsRefusedRatherThanDropped(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.account("agent-reader", []string{"courrier.read"}, []string{"courrier"})

	granted, status := fixture.token("agent-reader", secret, "courrier.write")
	if status == http.StatusOK {
		t.Fatalf("courrier.write was granted to a read-only account, with scope %q", granted.Scope)
	}
	if granted.Error != "invalid_scope" {
		t.Errorf("error = %q, want invalid_scope", granted.Error)
	}
}

func TestAnUnscopedRequestGetsExactlyWhatTheRegisterGrants(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	secret := fixture.account("agent-both", []string{"courrier.read", "courrier.write"}, []string{"courrier"})

	granted, status := fixture.token("agent-both", secret, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s", status, granted.Error)
	}
	if granted.Scope != "courrier.read courrier.write" {
		t.Errorf("scope = %q, want both granted scopes", granted.Scope)
	}
}

// TestDiscoveryAdvertisesOnlyMountedEndpoints catches the mistake that costs a
// whole afternoon: an endpoint listed in the discovery document that the router
// never registered, so every relying party finds it and gets a 404 from the SPA
// handler instead of an OIDC error.
func TestDiscoveryAdvertisesOnlyMountedEndpoints(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())

	request, err := http.NewRequest(http.MethodGet, fixture.issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	document, status := decode[map[string]any](t, request)
	if status != http.StatusOK {
		t.Fatalf("discovery = %d", status)
	}
	if document["issuer"] != fixture.issuer {
		t.Errorf("issuer = %v, want %s", document["issuer"], fixture.issuer)
	}

	mounted := map[string]bool{}
	for _, path := range Paths(fixture.provider) {
		mounted[fixture.issuer+path] = true
	}
	for key, value := range document {
		if !strings.HasSuffix(key, "_endpoint") && key != "jwks_uri" {
			continue
		}
		advertised, ok := value.(string)
		if !ok || advertised == "" {
			continue
		}
		if !mounted[advertised] {
			t.Errorf("%s advertises %s, which Mount does not register", key, advertised)
		}
	}
	for _, required := range []string{"token_endpoint", "jwks_uri", "authorization_endpoint"} {
		if document[required] == nil {
			t.Errorf("%s is missing from discovery", required)
		}
	}
}

// only reads a claim that is one value, whether the encoder wrote it as a
// string or as a one-element array. aud is both in the wild, so a test that
// assumed either would pass or fail for the wrong reason.
func only(claim any) string {
	switch typed := claim.(type) {
	case string:
		return typed
	case []any:
		if len(typed) == 1 {
			text, _ := typed[0].(string)
			return text
		}
	}
	return ""
}

// TestNoAdvertisedEndpointAnswersWithAServerError is the honesty check on the
// half of the provider phase 0 has not built. Every path discovery names is
// mounted, so none of them 404s; and every one of them refuses with an OIDC
// error rather than a 500, because "registre broke" and "registre does not do
// that yet" are different facts and only one of them should page anybody.
func TestNoAdvertisedEndpointAnswersWithAServerError(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())

	for _, path := range Paths(fixture.provider) {
		request, err := http.NewRequest(http.MethodPost, fixture.issuer+path, nil)
		if err != nil {
			t.Fatalf("building a request for %s: %v", path, err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		_ = response.Body.Close()
		if response.StatusCode == http.StatusNotFound {
			t.Errorf("POST %s = 404: discovery names it and Mount does not register it", path)
		}
		if response.StatusCode >= http.StatusInternalServerError {
			t.Errorf("POST %s = %d: an unimplemented flow must refuse, not fail", path, response.StatusCode)
		}
	}
}
