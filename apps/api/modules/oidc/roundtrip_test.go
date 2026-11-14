package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/seed"

	jose "github.com/go-jose/go-jose/v4"
	"log/slog"
)

// TestASeededServiceAccountCompletesTheTokenRoundTrip drives ROADMAP "Next"
// item 2 end to end through the production entry points: a service account
// that exists because the YAML seed put it there, a client_credentials grant
// over real HTTP, and a JWT verified offline against the published JWKS — the
// same path a relying party takes, with no introspection call.
//
// The account is deliberately created through internal/seed rather than the
// fixture's direct upsert helper: the round trip should prove that what the
// boot-time seed writes is what the token endpoint accepts.
func TestASeededServiceAccountCompletesTheTokenRoundTrip(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())

	secret := "seed-secret-for-the-round-trip-test"
	t.Setenv("ROUNDTRIP_TEST_SECRET", secret)
	document := `service_accounts:
  - client_id: agent-roundtrip
    name: Round trip test account
    secret_env: ROUNDTRIP_TEST_SECRET
    scopes: [courrier.read]
    audiences: [courrier]
`
	path := filepath.Join(t.TempDir(), "seed.yaml")
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatalf("writing the seed file: %v", err)
	}
	file, err := seed.Load(path)
	if err != nil {
		t.Fatalf("loading the seed: %v", err)
	}
	if err := seed.Apply(context.Background(), fixture.register, file, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("applying the seed: %v", err)
	}

	granted, status := fixture.token("agent-roundtrip", secret, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s: %s", status, granted.Error, granted.Description)
	}

	claims := mustVerifyAndCheck(t, fixture, granted.AccessToken, "agent-roundtrip", "courrier")

	mustCheckExpiry(t, claims, time.Hour)
	if iat, ok := claims["iat"].(float64); !ok || time.Unix(int64(iat), 0).After(time.Now().Add(time.Minute)) {
		t.Errorf("iat = %v, want a timestamp at or just before now", claims["iat"])
	}
}

// TestALiveRegistreIssuesAVerifiableToken extends porte's discovery check
// (TestNewBootsAgainstRegistre) into the full round trip against whatever
// issuer REGISTRE_ROUNDTRIP_ISSUER names — in practice the deployed instance.
// It needs real credentials, so it skips loudly without them:
//
//	REGISTRE_ROUNDTRIP_ISSUER=https://sso.facile.studio \
//	REGISTRE_ROUNDTRIP_CLIENT_ID=... REGISTRE_ROUNDTRIP_CLIENT_SECRET=... \
//	go test ./modules/oidc -run LiveRegistre
//
// Discovery is fetched rather than assumed, so the test also re-proves what
// porte's does: the document parses and carries a usable jwks_uri and
// token_endpoint.
func TestALiveRegistreIssuesAVerifiableToken(t *testing.T) {
	issuer := os.Getenv("REGISTRE_ROUNDTRIP_ISSUER")
	clientID := os.Getenv("REGISTRE_ROUNDTRIP_CLIENT_ID")
	clientSecret := os.Getenv("REGISTRE_ROUNDTRIP_CLIENT_SECRET")
	if issuer == "" || clientID == "" || clientSecret == "" {
		t.Skip("REGISTRE_ROUNDTRIP_ISSUER, _CLIENT_ID and _CLIENT_SECRET unset; " +
			"the live round trip needs real service-account credentials")
	}

	doc := fetchDiscovery(t, issuer)
	granted := requestClientCredentials(t, doc.TokenEndpoint, clientID, clientSecret)
	claims := verifyAgainstRemoteJWKS(t, doc.JWKSURI, granted.AccessToken)

	if claims["iss"] != strings.TrimSuffix(issuer, "/") {
		t.Errorf("iss = %v, want %s", claims["iss"], issuer)
	}
	if claims["client_id"] != clientID {
		t.Errorf("client_id = %v, want %s", claims["client_id"], clientID)
	}
	if only(claims["aud"]) == "" {
		t.Errorf("aud = %v, want the register-declared audience", claims["aud"])
	}
	if sub, _ := claims["sub"].(string); sub == "" {
		t.Errorf("sub = %v, want the service account's own subject", claims["sub"])
	}
	mustCheckExpiry(t, claims, time.Hour)
}

// discoveryDocument is the slice of the discovery document the round trip needs.
type discoveryDocument struct {
	TokenEndpoint string `json:"token_endpoint"`
	JWKSURI       string `json:"jwks_uri"`
}

// fetchDiscovery reads the provider's discovery document, proving along the way
// what porte's TestNewBootsAgainstRegistre does: it parses and names both
// endpoints the round trip is about to use.
func fetchDiscovery(t *testing.T, issuer string) discoveryDocument {
	t.Helper()
	response, err := http.Get(strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("fetching discovery: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var doc discoveryDocument
	if err := json.NewDecoder(response.Body).Decode(&doc); err != nil {
		t.Fatalf("parsing discovery: %v", err)
	}
	if doc.TokenEndpoint == "" || doc.JWKSURI == "" {
		t.Fatalf("discovery carries token_endpoint=%q jwks_uri=%q; both are required", doc.TokenEndpoint, doc.JWKSURI)
	}
	return doc
}

// requestClientCredentials POSTs a client_credentials grant and returns the
// token response, failing on anything but a 200.
func requestClientCredentials(t *testing.T, endpoint, clientID, clientSecret string) tokenResponse {
	t.Helper()
	form := url.Values{"grant_type": {"client_credentials"}}
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("building the token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(clientID, clientSecret)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("POST %s: %v", endpoint, err)
	}
	defer func() { _ = response.Body.Close() }()
	var granted tokenResponse
	if err := json.NewDecoder(response.Body).Decode(&granted); err != nil {
		t.Fatalf("decoding the token response (%d): %v", response.StatusCode, err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("token endpoint = %d %s: %s", response.StatusCode, granted.Error, granted.Description)
	}
	return granted
}

// verifyAgainstRemoteJWKS verifies a JWT against the key set published at
// jwksURI and returns its claims.
func verifyAgainstRemoteJWKS(t *testing.T, jwksURI, token string) map[string]any {
	t.Helper()
	signature, err := jose.ParseSignedCompact(token, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatalf("the access token is not a signed RS256 JWT: %v", err)
	}
	response, err := http.Get(jwksURI)
	if err != nil {
		t.Fatalf("fetching the JWKS: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var set jose.JSONWebKeySet
	if err := json.NewDecoder(response.Body).Decode(&set); err != nil {
		t.Fatalf("parsing the JWKS: %v", err)
	}
	payload, err := signature.Verify(&set)
	if err != nil {
		t.Fatalf("the published key set does not verify the minted token: %v", err)
	}
	claims := map[string]any{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("decoding the claims: %v", err)
	}
	return claims
}

// mustVerifyAndCheck verifies a token against the fixture's live JWKS and
// asserts the claims every relying party is told to authorize on.
func mustVerifyAndCheck(t *testing.T, fixture *fixture, token, clientID, audience string) map[string]any {
	t.Helper()
	claims, err := fixture.verify(token)
	if err != nil {
		t.Fatalf("the published key set does not verify the minted token: %v", err)
	}
	if claims["iss"] != fixture.issuer {
		t.Errorf("iss = %v, want %s", claims["iss"], fixture.issuer)
	}
	if claims["client_id"] != clientID {
		t.Errorf("client_id = %v, want %s", claims["client_id"], clientID)
	}
	if got := only(claims["aud"]); got != audience {
		t.Errorf("aud = %v, want %s", claims["aud"], audience)
	}
	if sub, _ := claims["sub"].(string); sub == "" || sub == clientID {
		t.Errorf("sub = %v, want a subject of the account's own", claims["sub"])
	}
	return claims
}

// mustCheckExpiry asserts exp exists, is in the future and inside ttl.
func mustCheckExpiry(t *testing.T, claims map[string]any, ttl time.Duration) {
	t.Helper()
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp = %v (%T), want a numeric timestamp", claims["exp"], claims["exp"])
	}
	exp := time.Unix(int64(expFloat), 0)
	now := time.Now()
	if exp.Before(now) || exp.After(now.Add(ttl+time.Minute)) {
		t.Errorf("exp = %s, want within now..now+%s of %s",
			exp.Format(time.RFC3339), ttl, now.Format(time.RFC3339))
	}
}
