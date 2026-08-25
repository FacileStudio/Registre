package oidc

import (
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// loginPath is where a browser is sent to authenticate. The login module
// serves it and resumes the pending flow from the auth_request parameter.
const loginPath = "/login?auth_request="

// client adapts a register row to the client contract zitadel/oidc enforces.
type client struct {
	row schemas.Client
	ttl time.Duration
}

// GetID returns the id the client authenticates under.
func (c client) GetID() string { return c.row.ClientID }

// RedirectURIs returns the exact-match redirect URIs. The library compares
// byte for byte, which is the whole defence against an open redirector.
func (c client) RedirectURIs() []string { return c.row.RedirectURIs }

// PostLogoutRedirectURIs returns where a client may be sent after logout.
func (c client) PostLogoutRedirectURIs() []string { return nil }

// ApplicationType reports the client as confidential: it holds a secret.
func (c client) ApplicationType() op.ApplicationType { return op.ApplicationTypeWeb }

// AuthMethod reports how the client proves who it is at the token endpoint.
func (c client) AuthMethod() zoidc.AuthMethod { return zoidc.AuthMethodBasic }

// ResponseTypes returns the response types this client may ask for. An
// application client that may start a browser flow gets "code"; a service
// account starts none and needs none.
func (c client) ResponseTypes() []zoidc.ResponseType {
	if !c.row.GrantTypes.Contains(string(zoidc.GrantTypeCode)) {
		return nil
	}
	return []zoidc.ResponseType{zoidc.ResponseTypeCode}
}

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
// token as well as at the userinfo endpoint. They do.
//
// This was false, on the reasoning that an ID token is logged, cached and
// forwarded far more often than a userinfo response. That reasoning still
// holds; it just loses to a hard requirement. porte reads its login claims
// from the ID token and nowhere else, and rejects a login whose address is
// empty, so a false here fails every browser login in the suite with "the
// identity provider returned no email". Authentik carried the same claims in
// its ID tokens, so this restores the contract the apps were written against
// rather than widening it.
//
// SetUserinfoFromScopes still gates each claim on its scope, so an app that
// asks for neither profile nor email receives neither.
func (c client) IDTokenUserinfoClaimsAssertion() bool { return true }

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
