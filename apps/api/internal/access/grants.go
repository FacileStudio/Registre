package access

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
	"gorm.io/gorm"
)

// Grants lists every grant of clientID.
func (s *Service) Grants(ctx context.Context, clientID string) ([]schemas.AppGrant, error) {
	var rows []schemas.AppGrant
	err := s.db.WithContext(ctx).Where("client_id = ?", clientID).Order("granted_at").Find(&rows).Error
	return rows, err
}

// UserGrants lists every grant userID holds.
func (s *Service) UserGrants(ctx context.Context, userID string) ([]schemas.AppGrant, error) {
	var rows []schemas.AppGrant
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("client_id").Find(&rows).Error
	return rows, err
}

func revokeRefresh(tx *gorm.DB, clientID, where string, args ...any) error {
	err := tx.Table("refresh_tokens").
		Where("client_id = ? AND revoked_at IS NULL", clientID).Where(where, args...).
		Update("revoked_at", time.Now().UTC()).Error
	if err != nil {
		return fmt.Errorf("access: revoke refresh tokens: %w", err)
	}
	return nil
}
