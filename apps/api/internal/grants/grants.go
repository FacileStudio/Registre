// Package grants records what a person allowed a third-party client: which
// resource it may reach on their behalf and with which scopes. A live grant
// is what lets a third-party flow skip the consent screen, and revoking one
// is how a person disconnects an app.
package grants

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrUnknown is a grant that does not exist, is not the caller's, or is
// already revoked. The three answer alike.
var ErrUnknown = errors.New("grants: no such live grant")

// Service reads and writes client_grants.
type Service struct {
	db *gorm.DB
}

// New builds the grants service.
func New(db *gorm.DB) *Service { return &Service{db: db} }

// Live returns the person's live grant to client for resource, if any.
func (s *Service) Live(ctx context.Context, userID, clientID, resource string) (schemas.ClientGrant, bool, error) {
	var row schemas.ClientGrant
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND client_id = ? AND resource = ? AND revoked_at IS NULL", userID, clientID, resource).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.ClientGrant{}, false, nil
	}
	if err != nil {
		return schemas.ClientGrant{}, false, fmt.Errorf("grants: read: %w", err)
	}
	return row, true, nil
}

// Record allows scopes on top of whatever the live grant already allows, or
// creates the grant. Asking again for more never narrows what was given.
func (s *Service) Record(ctx context.Context, userID, clientID, resource string, scopes []string) error {
	existing, found, err := s.Live(ctx, userID, clientID, resource)
	if err != nil {
		return err
	}
	if found {
		merged := slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(existing.Scopes), scopes...))))
		return s.db.WithContext(ctx).Model(&schemas.ClientGrant{}).Where("id = ?", existing.ID).
			Update("scopes", schemas.StringList(merged)).Error
	}
	id, err := ids.New()
	if err != nil {
		return fmt.Errorf("grants: mint an id: %w", err)
	}
	row := schemas.ClientGrant{
		ID: id, UserID: userID, ClientID: clientID, Resource: resource,
		Scopes: schemas.StringList(slices.Sorted(slices.Values(scopes))), CreatedAt: time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("grants: record: %w", err)
	}
	return nil
}

// ForUser lists the person's live grants, most recently created first.
func (s *Service) ForUser(ctx context.Context, userID string) ([]schemas.ClientGrant, error) {
	var rows []schemas.ClientGrant
	err := s.db.WithContext(ctx).Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("grants: list: %w", err)
	}
	return rows, nil
}

// Touch stamps the grant behind a token just issued.
func (s *Service) Touch(ctx context.Context, userID, clientID, resource string, at time.Time) error {
	return s.db.WithContext(ctx).Model(&schemas.ClientGrant{}).
		Where("user_id = ? AND client_id = ? AND resource = ? AND revoked_at IS NULL", userID, clientID, resource).
		Update("last_used_at", at).Error
}

// Revoke ends the person's grant id and, in the same transaction, every
// refresh token and recorded access token the client holds for them. The
// client's access tokens already in flight stay valid until they expire,
// which for a third party is at most ten minutes.
func (s *Service) Revoke(ctx context.Context, userID, id string, at time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row schemas.ClientGrant
		err := tx.Where("id = ? AND user_id = ? AND revoked_at IS NULL", id, userID).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnknown
		}
		if err != nil {
			return fmt.Errorf("grants: read: %w", err)
		}
		if err := tx.Model(&row).Update("revoked_at", at).Error; err != nil {
			return fmt.Errorf("grants: revoke: %w", err)
		}
		return revokeTokens(tx, row, at)
	})
}

// revokeTokens ends the client's refresh and access tokens for the person.
func revokeTokens(tx *gorm.DB, row schemas.ClientGrant, at time.Time) error {
	if err := tx.Model(&schemas.RefreshToken{}).
		Where("client_id = ? AND user_id = ? AND revoked_at IS NULL", row.ClientID, row.UserID).
		Update("revoked_at", at).Error; err != nil {
		return fmt.Errorf("grants: revoke the refresh tokens: %w", err)
	}
	if err := tx.Model(&schemas.AccessToken{}).
		Where("client_id = ? AND subject = ? AND revoked_at IS NULL", row.ClientID, row.UserID).
		Update("revoked_at", at).Error; err != nil {
		return fmt.Errorf("grants: revoke the access tokens: %w", err)
	}
	return nil
}
