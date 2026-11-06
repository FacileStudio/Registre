package social

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Link is one external account a user has linked.
type Link struct {
	Provider    string    `json:"provider"`
	DisplayName string    `json:"display_name"`
	LinkedAt    time.Time `json:"linked_at"`
}

// Links lists the accounts userID has linked.
func (s *Service) Links(ctx context.Context, userID string) ([]Link, error) {
	var rows []schemas.SocialIdentity
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("provider").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("social: list the links: %w", err)
	}
	out := make([]Link, 0, len(rows))
	for _, row := range rows {
		out = append(out, Link{Provider: row.Provider, DisplayName: row.DisplayName, LinkedAt: row.CreatedAt})
	}
	return out, nil
}

// Connections is the provider-to-subject map the connections claim carries
// for clientID, such as {"discord": "123456789012345678"}. Only providers the
// client opted into through login_providers appear. A client that opted into
// none gets a nil map, which means the claim is not emitted at all; one that
// opted in and has nothing linked gets an empty map.
func (s *Service) Connections(ctx context.Context, userID, clientID string) (map[string]string, error) {
	allowed, err := s.ForClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if len(allowed) == 0 {
		return nil, nil
	}
	var rows []schemas.SocialIdentity
	err = s.db.WithContext(ctx).Where("user_id = ? AND provider IN ?", userID, allowed).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("social: read the connections: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Provider] = row.Subject
	}
	return out, nil
}

// LinkTo links profile to a signed-in user. An account another user already
// holds is ErrTaken; linking the same account again is a no-op that refreshes
// its name.
func (s *Service) LinkTo(ctx context.Context, userID, provider string, profile Profile) error {
	var existing schemas.SocialIdentity
	err := s.db.WithContext(ctx).Where("provider = ? AND subject = ?", provider, profile.Subject).Take(&existing).Error
	switch {
	case err == nil && existing.UserID != userID:
		return ErrTaken
	case err == nil:
		return s.db.WithContext(ctx).Model(&existing).Update("display_name", displayName(profile)).Error
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("social: read the identity: %w", err)
	}
	err = s.insertIdentity(ctx, s.db, userID, provider, profile)
	if errors.Is(err, errRaced) {
		return ErrTaken
	}
	return err
}

// Unlink removes userID's account of provider. It refuses when that would
// leave the user no way to sign in: no password, no passkey and no other
// linked account. The user row is locked for the check so two unlinks cannot
// each see the other's account still standing.
func (s *Service) Unlink(ctx context.Context, userID, provider string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user schemas.User
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).Take(&user).Error
		if err != nil {
			return fmt.Errorf("social: lock the user: %w", err)
		}
		var others, passkeys int64
		if err := tx.Model(&schemas.SocialIdentity{}).
			Where("user_id = ? AND provider <> ?", userID, provider).Count(&others).Error; err != nil {
			return fmt.Errorf("social: count the other links: %w", err)
		}
		if err := tx.Model(&schemas.Passkey{}).Where("user_id = ?", userID).Count(&passkeys).Error; err != nil {
			return fmt.Errorf("social: count the passkeys: %w", err)
		}
		if user.PasswordHash == "" && passkeys == 0 && others == 0 {
			return ErrLastFactor
		}
		result := tx.Where("user_id = ? AND provider = ?", userID, provider).Delete(&schemas.SocialIdentity{})
		if result.Error != nil {
			return fmt.Errorf("social: unlink: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotLinked
		}
		return nil
	})
}
