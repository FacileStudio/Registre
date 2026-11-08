package oidc

import (
	"context"
	"errors"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"

	"github.com/zitadel/oidc/v3/pkg/op"
)

var _ op.CanGetPrivateClaimsFromRequest = (*Storage)(nil)

// GetPrivateClaimsFromRequest is GetPrivateClaimsFromScopes plus the scope
// claim of a token bound to an MCP resource.
//
// zitadel prefers this method over GetPrivateClaimsFromScopes whenever the
// storage has it, so the roles claim is still computed here exactly as before.
// The scope claim (RFC 9068 §2.2.3, space separated) lists only the scopes the
// resource declares, because that is all the resource server can enforce, and
// it is emitted only when the token's single audience is a registered
// resource: a plain access token keeps the shape every suite app was written
// against.
func (s *Storage) GetPrivateClaimsFromRequest(
	ctx context.Context, request op.TokenRequest, scopes []string,
) (map[string]any, error) {
	claims, err := s.GetPrivateClaimsFromScopes(ctx, request.GetSubject(), grantPrincipal(request).clientID, scopes)
	if err != nil {
		return nil, err
	}
	scope, err := s.resourceScope(ctx, request)
	if err != nil || scope == nil {
		return claims, err
	}
	if claims == nil {
		claims = map[string]any{}
	}
	claims["scope"] = strings.Join(scope, " ")
	return claims, nil
}

// resourceScope is the scope claim's content for a token whose only audience
// is a registered MCP resource, and nil for every other token.
func (s *Storage) resourceScope(ctx context.Context, request op.TokenRequest) ([]string, error) {
	audience := request.GetAudience()
	if len(audience) != 1 || !clients.LooksLikeResource(audience[0]) {
		return nil, nil
	}
	owner, err := s.svc.Register.ByResource(ctx, audience[0])
	if errors.Is(err, clients.ErrUnknownResource) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	granted := []string{}
	for _, scope := range request.GetScopes() {
		if owner.MCPScopes.Contains(scope) {
			granted = append(granted, scope)
		}
	}
	return granted, nil
}

// GetClientID returns the machine principal that asked.
func (r tokenRequest) GetClientID() string { return r.clientID }
