package oidc

import (
	"context"
	"errors"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// resourceParam is the RFC 8707 request parameter naming the server a token
// is for.
const resourceParam = "resource"

type resourceKey struct{}

// withResources hands the resource indicators /authorize was called with to
// CreateAuthRequest through the request context. zitadel's request struct has
// no field for them, and its decoder drops unknown parameters.
func withResources(r *http.Request) *http.Request {
	if err := r.ParseForm(); err != nil {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), resourceKey{}, r.Form[resourceParam]))
}

// requestedResources is what withResources stored, or nothing.
func requestedResources(ctx context.Context) []string {
	values, _ := ctx.Value(resourceKey{}).([]string)
	return values
}

// refusedResource is the one answer for every resource registre will not
// mint for: unknown, disabled, several at once, or scopes it does not carry.
func refusedResource() *zoidc.Error {
	return zoidc.ErrInvalidTarget().WithDescription("the resource cannot be requested")
}

// refusedChallengeMethod refuses every PKCE method but S256. plain sends the
// verifier itself through the front channel, which is the leak PKCE exists to
// close, and OAuth 2.1 and the MCP authorization spec both forbid it.
func refusedChallengeMethod() *zoidc.Error {
	return zoidc.ErrInvalidRequest().WithDescription("code_challenge_method must be S256")
}

// checkChallengeMethod accepts S256 and the empty method a client sends with
// no challenge at all. An empty method next to a challenge is read as S256
// by GetCodeChallenge, so a plain verifier never matches.
func checkChallengeMethod(request *zoidc.AuthRequest) error {
	switch request.CodeChallengeMethod {
	case "", zoidc.CodeChallengeMethodS256:
		return nil
	}
	return refusedChallengeMethod()
}

// resolveResource validates the resource a browser flow asked for and
// returns it, or empty when none was asked. At most one resource is minted
// for: a token valid at two servers is the passthrough the audience binding
// exists to prevent. Every non-OIDC scope in the request must be one the
// resource declares, so a token for one app never carries another's scopes.
func (s *Storage) resolveResource(ctx context.Context, request *zoidc.AuthRequest) (string, error) {
	asked := requestedResources(ctx)
	if len(asked) == 0 {
		return "", s.refuseStrayResourceScopes(ctx, request.ClientID, request.Scopes)
	}
	if len(asked) > 1 {
		return "", refusedResource()
	}
	owner, err := s.svc.Register.ByResource(ctx, asked[0])
	if errors.Is(err, clients.ErrUnknownResource) {
		return "", refusedResource()
	}
	if err != nil {
		return "", zoidc.ErrServerError().WithParent(err)
	}
	if !resourceCarries(owner, request.Scopes) {
		return "", refusedResource()
	}
	return owner.MCPResource, nil
}

// resourceCarries reports whether every scope beyond the OIDC ones is a scope
// owner's resource declares.
func resourceCarries(owner schemas.Client, scopes []string) bool {
	for _, scope := range scopes {
		if !oidcScope(scope) && !owner.MCPScopes.Contains(scope) {
			return false
		}
	}
	return true
}

// oidcScope reports whether scope is one of the standard OIDC scopes, which
// ride along with any resource because they shape the ID token, not the
// access token.
func oidcScope(scope string) bool {
	switch scope {
	case zoidc.ScopeOpenID, zoidc.ScopeProfile, zoidc.ScopeEmail, zoidc.ScopeOfflineAccess:
		return true
	}
	return false
}
