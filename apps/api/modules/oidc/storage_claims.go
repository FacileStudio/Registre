package oidc

import (
	"context"
	"fmt"
	"github.com/FacileStudio/Registre/apps/api/internal/access"
)

// GetPrivateClaimsFromScopes returns the claims a scope adds to an access
// token. The only one today is roles, and it is emitted filtered per client —
// SPEC §6 requires it, and porte's role checks deny silently when the claim
// arrives unfiltered or not at all.
func (s *Storage) GetPrivateClaimsFromScopes(
	ctx context.Context, userID, clientID string, scopes []string,
) (map[string]any, error) {
	roles, err := s.rolesForClient(ctx, userID, clientID)
	if err != nil {
		return nil, err
	}
	if !contains(scopes, scopeRoles) || len(roles) == 0 {
		return nil, nil
	}
	return map[string]any{"roles": roles}, nil
}

// rolesForClient answers the roles the user's grant to this app carries,
// intersected with the roles the app declared visible. An app that named no
// roles sees none, whatever its tokens ask for: visibility is opt-in per
// client, because a role learned by an app that never needed it is a
// permission leak nobody notices until it matters.
func (s *Storage) rolesForClient(ctx context.Context, userID, clientID string) ([]string, error) {
	roles, err := access.New(s.db, "").Roles(ctx, clientID, userID)
	if err != nil {
		return nil, fmt.Errorf("oidc: read the roles: %w", err)
	}
	return roles, nil
}

// contains reports whether scopes names name.
func contains(scopes []string, name string) bool {
	for _, scope := range scopes {
		if scope == name {
			return true
		}
	}
	return false
}
