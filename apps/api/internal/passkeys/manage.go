package passkeys

import (
	"context"
	"errors"
	"fmt"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrLastFactor refuses removing the only way into an account: its last
// passkey when it has no password.
var ErrLastFactor = errors.New("passkeys: this is the account's last way to sign in")

// ErrNotFound is a passkey id the account does not hold.
var ErrNotFound = errors.New("passkeys: no such passkey")

// List answers the account's passkeys, oldest first.
func (s *Service) List(ctx context.Context, userID string) ([]schemas.Passkey, error) {
	var rows []schemas.Passkey
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at, id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("passkeys: list: %w", err)
	}
	return rows, nil
}

// Rename relabels one of the account's passkeys.
func (s *Service) Rename(ctx context.Context, userID, id, name string) error {
	result := s.db.WithContext(ctx).Model(&schemas.Passkey{}).
		Where("id = ? AND user_id = ?", id, userID).Update("name", CleanName(name))
	if result.Error != nil {
		return fmt.Errorf("passkeys: rename: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}

// Remove deletes one of the account's passkeys, unless it is the last one on
// an account without a password. The account row is locked for the check, so
// two removals racing for the last two passkeys cannot both pass it.
func (s *Service) Remove(ctx context.Context, userID, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := keepsAFactor(tx, userID); err != nil {
			return err
		}
		result := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&schemas.Passkey{})
		if result.Error != nil {
			return fmt.Errorf("passkeys: remove: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		return nil
	})
}

// keepsAFactor locks the account and refuses when removing one passkey
// would leave it with no way in.
func keepsAFactor(tx *gorm.DB, userID string) error {
	var user schemas.User
	if err := tx.Raw("SELECT * FROM users WHERE id = ? FOR UPDATE", userID).Scan(&user).Error; err != nil {
		return fmt.Errorf("passkeys: lock the account: %w", err)
	}
	var count int64
	if err := tx.Model(&schemas.Passkey{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("passkeys: count: %w", err)
	}
	if count <= 1 && user.PasswordHash == "" {
		return ErrLastFactor
	}
	return nil
}

// Count answers how many passkeys the account holds.
func (s *Service) Count(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&schemas.Passkey{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("passkeys: count: %w", err)
	}
	return count, nil
}
