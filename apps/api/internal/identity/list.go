package identity

import (
	"context"
	"fmt"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// List returns every account, oldest first. The dashboard reads whole rows
// but never ships a PasswordHash over the wire; that discipline lives with
// the handler, which builds its own view of the row.
func (s *Service) List(ctx context.Context) ([]schemas.User, error) {
	var rows []schemas.User
	if err := s.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("identity: list: %w", err)
	}
	return rows, nil
}
