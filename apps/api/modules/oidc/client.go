package oidc

import (
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// loginPath is where a browser is sent to authenticate. Nothing serves it in
// phase 0 — CreateAuthRequest refuses before a redirect is ever built — but the
// client contract requires the method, and a placeholder that points somewhere
// real is cheaper to correct than one that points nowhere.
const loginPath = "/login?auth_request="

// client adapts a register row to the client contract zitadel/oidc enforces.
type client struct {
	row schemas.Client
	ttl time.Duration
}

// GetID returns the id the client authenticates under.
func (c client) GetID() string { return c.row.ClientID }

// RedirectURIs returns the exact-match redirect URIs. Phase 1 fills them in;
// until then a client with none can start no browser flow, which is correct.
func (c client) RedirectURIs() []string { return nil }

// PostLogoutRedirectURIs returns where a client may be sent after logout.
func (c client) PostLogoutRedirectURIs() []string { return nil }

// ApplicationType reports the client as confidential: it holds a secret.
func (c client) ApplicationType() op.ApplicationType { return op.ApplicationTypeWeb }

// AuthMethod reports how the client proves who it is at the token endpoint.
func (c client) AuthMethod() zoidc.AuthMethod { return zoidc.AuthMethodBasic }

// ResponseTypes returns the response types this client may ask for.
func (c client) ResponseTypes() []zoidc.ResponseType { return nil }

// GrantTypes returns the grants the register row allows.
func (c client) GrantTypes() []zoidc.GrantType {
	grants := make([]zoidc.GrantType, 0, len(c.row.GrantTypes))
	for _, name := range c.row.GrantTypes {
		grants = append(grants, zoidc.GrantType(name))
	}
	return grants
}

// LoginURL is where the browser goes to authenticate a pending request.
func (c client) LoginURL(requestID string) string { return loginPath + requestID }

// AccessTokenType reports that access tokens are signed JWTs.
//
// This is the decision that lets an app verify a machine token against the
// JWKS without a round trip to registre, and it is required for
// client_credentials: an opaque token would carry no subject, no audience and
// no expiry that a relying party could read.
func (c client) AccessTokenType() op.AccessTokenType { return op.AccessTokenTypeJWT }

// IDTokenLifetime is how long an ID token stays valid.
func (c client) IDTokenLifetime() time.Duration { return c.ttl }

// DevMode reports whether the redirect URI rules are relaxed. They never are.
func (c client) DevMode() bool { return false }

// RestrictAdditionalIdTokenScopes narrows what reaches an ID token.
func (c client) RestrictAdditionalIdTokenScopes() func([]string) []string {
	return func(scopes []string) []string { return scopes }
}

// RestrictAdditionalAccessTokenScopes narrows what reaches an access token.
func (c client) RestrictAdditionalAccessTokenScopes() func([]string) []string {
	return func(scopes []string) []string { return scopes }
}

// IsScopeAllowed reports whether the register row grants this scope.
func (c client) IsScopeAllowed(scope string) bool { return c.row.Scopes.Contains(scope) }

// IDTokenUserinfoClaimsAssertion reports whether profile claims ride in the ID
// token as well as at the userinfo endpoint. They do not: an ID token is
// logged, cached and forwarded far more often than a userinfo response.
func (c client) IDTokenUserinfoClaimsAssertion() bool { return false }

// ClockSkew is the tolerance granted to a client's clock. Zero: a skew here
// extends every token's life for everyone, and a machine whose clock is wrong
// enough to matter has a problem that hiding it does not fix.
func (c client) ClockSkew() time.Duration { return 0 }

// tokenRequest is what a granted client_credentials request becomes. It
// carries the client id as well, which the op.TokenRequest contract does not,
// because the row recording the issued token has to name who asked.
type tokenRequest struct {
	clientID string
	subject  string
	audience []string
	scopes   []string
	lifetime time.Duration
}

// GetSubject returns the principal the token speaks for.
func (r tokenRequest) GetSubject() string { return r.subject }

// GetAudience returns the apps the token is valid at.
func (r tokenRequest) GetAudience() []string { return r.audience }

// GetScopes returns the scopes granted to this token.
func (r tokenRequest) GetScopes() []string { return r.scopes }
