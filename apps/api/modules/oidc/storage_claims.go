package oidc

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
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

// rolesForClient intersects a human's roles with the whitelist their app
// declared. An app that named no roles sees none, whatever its tokens ask
// for: visibility is opt-in per client, because a role learned by an app that
// never needed it is a permission leak nobody notices until it matters.
func (s *Storage) rolesForClient(ctx context.Context, userID, clientID string) ([]string, error) {
	user, err := s.svc.Users.ByID(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("oidc: read the user for roles: %w", err)
	}
	row, err := s.svc.Register.ByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("oidc: read the client for roles: %w", err)
	}
	visible := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		if row.VisibleRoles.Contains(role) {
			visible = append(visible, role)
		}
	}
	return visible, nil
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
