package access

import (
	"context"
	"fmt"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// RequireResource answers ErrNoAccess when userID may not use the app that
// declares resource as its MCP server. A token for an app's MCP server is
// access to that app, whichever client holds it, so the app's policy applies
// to it as it does to a sign-in. An empty or undeclared resource restricts
// nothing: there is no app behind it to protect.
func (s *Service) RequireResource(ctx context.Context, resource, userID string) error {
	if resource == "" {
		return nil
	}
	var owners []string
	err := s.db.WithContext(ctx).Model(&schemas.Client{}).
		Where("mcp_resource = ?", resource).Pluck("client_id", &owners).Error
	if err != nil {
		return fmt.Errorf("access: read the resource owner: %w", err)
	}
	for _, owner := range owners {
		if err := s.Require(ctx, owner, userID); err != nil {
			return err
		}
	}
	return nil
}
