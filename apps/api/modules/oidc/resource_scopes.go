package oidc

import (
	"context"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// resourceScopes is every scope some registered MCP resource declares. A
// client may ask for any of them at /authorize; resolveResource then insists
// they arrive next to the resource that declares them.
func (s *Storage) resourceScopes(ctx context.Context) (map[string]bool, error) {
	owners, err := s.svc.Register.Resources(ctx)
	if err != nil {
		return nil, err
	}
	scopes := map[string]bool{}
	for _, owner := range owners {
		for _, scope := range owner.MCPScopes {
			scopes[scope] = true
		}
	}
	return scopes, nil
}

// refuseStrayResourceScopes refuses a flow that asks for a resource scope
// without the resource. The scope would land in a token whose audience is the
// client itself, where it means nothing and reads as if it did.
func (s *Storage) refuseStrayResourceScopes(ctx context.Context, clientID string, scopes []string) error {
	row, err := s.svc.Register.ByClientID(ctx, clientID)
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	extra, err := s.resourceScopes(ctx)
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	for _, scope := range scopes {
		if extra[scope] && !row.Scopes.Contains(scope) {
			return zoidc.ErrInvalidScope().WithDescription("the scope %q needs its resource", scope)
		}
	}
	return nil
}

// requireGrantAccess refuses a refresh when the user may no longer use the app
// the grant was issued to, or the app behind the MCP resource it is bound to.
// Revoking someone's access to an app therefore also ends a third-party
// client's hold on that app's MCP server, within one access token lifetime.
func requireGrantAccess(ctx context.Context, gate *access.Service, row schemas.RefreshToken) error {
	if err := gate.Require(ctx, row.ClientID, row.UserID); err != nil {
		return err
	}
	for _, audience := range row.Audiences {
		if err := gate.RequireResource(ctx, audience, row.UserID); err != nil {
			return err
		}
	}
	return nil
}
