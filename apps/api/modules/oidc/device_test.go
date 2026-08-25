package oidc

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// deviceGrant is the grant type's name on the wire. It is spelled out here
// rather than taken from the library so that a test would notice the day the
// constant changed under it.
const deviceGrant = "urn:ietf:params:oauth:grant-type:device_code"

// publicClient writes a client that holds no secret into the register: the
// shape a CLI has to have, because anything compiled into a binary is readable
// by whoever holds the binary.
func (f *fixture) publicClient(clientID string, grants, scopes []string) {
	f.t.Helper()
	subject, err := ids.New()
	if err != nil {
		f.t.Fatalf("minting a subject: %v", err)
	}
	row := schemas.Client{
		ClientID: clientID, Subject: subject, Name: "Facile CLI",
		Kind: schemas.KindApplication, Public: true,
		GrantTypes: schemas.StringList(grants),
		Scopes:     schemas.StringList(scopes),
		Managed:    true,
	}
	if _, err := f.register.Upsert(context.Background(), row); err != nil {
		f.t.Fatalf("seeding %s: %v", clientID, err)
	}
}

// startDeviceFlow calls the device authorization endpoint the way a CLI does:
// a form post, a client id, no credential.
func (f *fixture) startDeviceFlow(clientID, scope string) map[string]any {
	f.t.Helper()
	body, status := f.postForm("/device_authorization", url.Values{
		"client_id": {clientID}, "scope": {scope},
	})
	if status != http.StatusOK {
		f.t.Fatalf("POST /device_authorization = %d: %v", status, body)
	}
	return body
}

// pollDeviceToken redeems, or tries to redeem, a device code at the token
// endpoint.
func (f *fixture) pollDeviceToken(clientID, deviceCode string) (map[string]any, int) {
	f.t.Helper()
	return f.postForm("/oauth/token", url.Values{
		"grant_type": {deviceGrant}, "device_code": {deviceCode}, "client_id": {clientID},
	})
}

// TestTheDeviceGrantIssuesATokenAfterAHumanApproves is the whole feature end
// to end over real HTTP against real Postgres: a CLI with no browser and no
// secret asks for a pair of codes, is told to wait, and receives a token the
// moment a named human approves the short code somewhere else.
//
// The loopback flow it replaces cannot do this at all. `facile login` on a
// server opens 127.0.0.1:<port> and redirects the browser there, which on the
// laptop reading the screen is the laptop — so the code lands nowhere and
// expires unused, with nothing in any log to say why.
func TestTheDeviceGrantIssuesATokenAfterAHumanApproves(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	fixture.publicClient("facile-cli", []string{deviceGrant, "refresh_token"},
		[]string{"openid", "profile", "email", "offline_access"})
	fixture.human("yann@facile.studio", "correct horse battery staple")
	user, err := fixture.users.ByEmail(context.Background(), "yann@facile.studio")
	if err != nil {
		t.Fatalf("reading the human back: %v", err)
	}

	started := fixture.startDeviceFlow("facile-cli", "openid profile email offline_access")
	deviceCode, _ := started["device_code"].(string)
	userCode, _ := started["user_code"].(string)
	assertDeviceResponse(t, fixture, started)

	pending, status := fixture.pollDeviceToken("facile-cli", deviceCode)
	if status == http.StatusOK || pending["error"] != "authorization_pending" {
		t.Fatalf("polling before approval = %d %v, want authorization_pending", status, pending)
	}

	approval := devices.Approval{UserID: user.ID, AMR: []string{"pwd"}, AuthTime: fixture.clock.now()}
	if err := fixture.devices.Approve(context.Background(), userCode, approval); err != nil {
		t.Fatalf("approving %s: %v", userCode, err)
	}

	granted, status := fixture.pollDeviceToken("facile-cli", deviceCode)
	if status != http.StatusOK {
		t.Fatalf("polling after approval = %d: %v", status, granted)
	}
	assertDeviceToken(t, fixture, granted, user.ID)
}

// assertDeviceResponse checks the half of RFC 8628 §3.2 a CLI has to print or
// obey: the code pair, where to send the human, and how often to come back.
func assertDeviceResponse(t *testing.T, fixture *fixture, started map[string]any) {
	t.Helper()
	deviceCode, _ := started["device_code"].(string)
	userCode, _ := started["user_code"].(string)
	if deviceCode == "" || userCode == "" {
		t.Fatalf("the device authorization response is missing a code: %v", started)
	}
	if devices.Normalise(userCode) == "" {
		t.Errorf("user_code %q carries nothing from the alphabet", userCode)
	}
	if want := fixture.issuer + devices.VerificationPath; started["verification_uri"] != want {
		t.Errorf("verification_uri = %v, want %s", started["verification_uri"], want)
	}
	complete, _ := started["verification_uri_complete"].(string)
	if !strings.Contains(complete, "user_code="+userCode) {
		t.Errorf("verification_uri_complete = %q, want it to carry the user code", complete)
	}
	if started["interval"] != float64(devices.PollInterval.Seconds()) {
		t.Errorf("interval = %v, want %v", started["interval"], devices.PollInterval.Seconds())
	}
	if started["expires_in"] != float64(devices.Lifetime.Seconds()) {
		t.Errorf("expires_in = %v, want %v", started["expires_in"], devices.Lifetime.Seconds())
	}
}

// assertDeviceToken checks that what came back is a real, verifiable token for
// the human who approved the flow, and that offline_access brought a renewable
// grant with it — a CLI re-approved by hand every hour is a CLI nobody uses.
func assertDeviceToken(t *testing.T, fixture *fixture, granted map[string]any, subject string) {
	t.Helper()
	accessToken, _ := granted["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("no access_token in %v", granted)
	}
	claims, err := fixture.verify(accessToken)
	if err != nil {
		t.Fatalf("the access token does not verify against the JWKS: %v", err)
	}
	if claims["sub"] != subject {
		t.Errorf("the token's sub is %v, want the human who approved it (%s)", claims["sub"], subject)
	}
	if idToken, _ := granted["id_token"].(string); idToken == "" {
		t.Error("openid was granted but no id_token came back")
	} else if idClaims, err := fixture.verify(idToken); err != nil {
		t.Errorf("the id_token does not verify: %v", err)
	} else if idClaims["email"] != "yann@facile.studio" {
		t.Errorf("the id_token's email is %v, want yann@facile.studio", idClaims["email"])
	}
	if granted["refresh_token"] == nil {
		t.Error("offline_access was granted but no refresh_token came back")
	}
}

// TestASpentDeviceCodeIsRefused is the replay guard seen from the outside.
// zitadel/oidc never tells the storage that a token was issued, so without the
// spend in internal/devices a captured device code would keep minting
// independent tokens for the rest of its ten minutes.
func TestASpentDeviceCodeIsRefused(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	fixture.publicClient("facile-cli", []string{deviceGrant}, []string{"openid"})
	fixture.human("yann@facile.studio", "correct horse battery staple")
	user, err := fixture.users.ByEmail(context.Background(), "yann@facile.studio")
	if err != nil {
		t.Fatalf("reading the human back: %v", err)
	}

	started := fixture.startDeviceFlow("facile-cli", "openid")
	deviceCode, _ := started["device_code"].(string)
	userCode, _ := started["user_code"].(string)
	approval := devices.Approval{UserID: user.ID, AMR: []string{"pwd"}, AuthTime: fixture.clock.now()}
	if err := fixture.devices.Approve(context.Background(), userCode, approval); err != nil {
		t.Fatalf("approving: %v", err)
	}
	if _, status := fixture.pollDeviceToken("facile-cli", deviceCode); status != http.StatusOK {
		t.Fatalf("the first redemption = %d", status)
	}

	replayed, status := fixture.pollDeviceToken("facile-cli", deviceCode)
	if status == http.StatusOK {
		t.Fatalf("the device code minted a second token: %v", replayed)
	}
	if replayed["error"] != "access_denied" {
		t.Errorf("the replay was refused as %v, want access_denied", replayed["error"])
	}
}

// TestADeniedDeviceFlowStopsThePolling checks the answer a human gives when
// they see a code they did not ask for. It has to reach the CLI as a refusal
// rather than as silence, or the program sits polling a flow that will never
// be approved until it times out.
func TestADeniedDeviceFlowStopsThePolling(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	fixture.publicClient("facile-cli", []string{deviceGrant}, []string{"openid"})

	started := fixture.startDeviceFlow("facile-cli", "openid")
	deviceCode, _ := started["device_code"].(string)
	userCode, _ := started["user_code"].(string)
	if err := fixture.devices.Deny(context.Background(), userCode); err != nil {
		t.Fatalf("denying: %v", err)
	}

	refused, status := fixture.pollDeviceToken("facile-cli", deviceCode)
	if status == http.StatusOK {
		t.Fatalf("a denied flow still minted a token: %v", refused)
	}
	if refused["error"] != "access_denied" {
		t.Errorf("a denied flow answers %v, want access_denied", refused["error"])
	}
}

// TestAClientWithoutTheDeviceGrantIsRefused pins the register's half of the
// decision. The endpoint is unauthenticated, so the only thing standing
// between any caller and a device flow is what the register says that client
// may ask for.
func TestAClientWithoutTheDeviceGrantIsRefused(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	fixture.application("sablier", "http://127.0.0.1:9999/callback")

	body, status := fixture.postForm("/device_authorization", url.Values{"client_id": {"sablier"}})
	if status == http.StatusOK {
		t.Fatalf("a client with no device_code grant started a flow: %v", body)
	}
	if body["error"] != "unauthorized_client" {
		t.Errorf("the refusal is %v, want unauthorized_client", body["error"])
	}
}

// TestThePublicClientRenewsWithoutASecret is what makes the grant usable past
// the first hour. A public client authenticates with nothing, so the refresh
// grant has to work with nothing — and the register still has to refuse the
// scope it was never given.
func TestThePublicClientRenewsWithoutASecret(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	fixture.publicClient("facile-cli", []string{deviceGrant, "refresh_token"},
		[]string{"openid", "offline_access"})
	fixture.human("yann@facile.studio", "correct horse battery staple")
	user, err := fixture.users.ByEmail(context.Background(), "yann@facile.studio")
	if err != nil {
		t.Fatalf("reading the human back: %v", err)
	}

	started := fixture.startDeviceFlow("facile-cli", "openid offline_access")
	deviceCode, _ := started["device_code"].(string)
	userCode, _ := started["user_code"].(string)
	approval := devices.Approval{UserID: user.ID, AMR: []string{"pwd"}, AuthTime: fixture.clock.now()}
	if err := fixture.devices.Approve(context.Background(), userCode, approval); err != nil {
		t.Fatalf("approving: %v", err)
	}
	granted, status := fixture.pollDeviceToken("facile-cli", deviceCode)
	if status != http.StatusOK {
		t.Fatalf("redeeming = %d: %v", status, granted)
	}
	refreshToken, _ := granted["refresh_token"].(string)
	if refreshToken == "" {
		t.Fatalf("no refresh token to renew with: %v", granted)
	}

	renewed, status := fixture.postForm("/oauth/token", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {"facile-cli"},
	})
	if status != http.StatusOK {
		t.Fatalf("renewing without a secret = %d: %v", status, renewed)
	}
	if renewed["access_token"] == nil {
		t.Errorf("the renewal carried no access token: %v", renewed)
	}
}
