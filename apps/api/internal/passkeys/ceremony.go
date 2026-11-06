package passkeys

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// open stores a ceremony's session data and answers the bearer the browser
// sends back with its answer.
func (s *Service) open(ctx context.Context, purpose, userID, boundTo string, session *webauthn.SessionData) (string, error) {
	token, err := ids.Secret()
	if err != nil {
		return "", fmt.Errorf("passkeys: mint the ceremony token: %w", err)
	}
	id, err := ids.New()
	if err != nil {
		return "", fmt.Errorf("passkeys: mint the ceremony id: %w", err)
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("passkeys: encode the session: %w", err)
	}
	var data schemas.JSONMap
	if err := json.Unmarshal(encoded, &data); err != nil {
		return "", fmt.Errorf("passkeys: encode the session: %w", err)
	}
	row := schemas.Ceremony{
		ID: id, TokenHash: digest(token), Purpose: purpose, Session: data, BoundTo: boundTo,
		ExpiresAt: s.now().Add(CeremonyTTL), CreatedAt: s.now(),
	}
	if userID != "" {
		row.UserID = &userID
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("passkeys: record the ceremony: %w", err)
	}
	return token, nil
}

// claim spends a live ceremony of the given purpose, bound to boundTo,
// exactly once and answers its session data. The conditional UPDATE is the replay guard: two answers
// racing the same ceremony see one row between them.
func (s *Service) claim(ctx context.Context, token, purpose, boundTo string) (schemas.Ceremony, webauthn.SessionData, error) {
	var row schemas.Ceremony
	err := s.db.WithContext(ctx).Where("token_hash = ?", digest(token)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || token == "" {
		return row, webauthn.SessionData{}, ErrInvalid
	}
	if err != nil {
		return row, webauthn.SessionData{}, fmt.Errorf("passkeys: read the ceremony: %w", err)
	}
	now := s.now()
	spent := s.db.WithContext(ctx).Model(&schemas.Ceremony{}).
		Where("id = ? AND purpose = ? AND bound_to = ? AND consumed_at IS NULL AND expires_at > ?", row.ID, purpose, boundTo, now).
		Update("consumed_at", now)
	if spent.Error != nil {
		return row, webauthn.SessionData{}, fmt.Errorf("passkeys: spend the ceremony: %w", spent.Error)
	}
	if spent.RowsAffected != 1 {
		return row, webauthn.SessionData{}, ErrInvalid
	}
	var session webauthn.SessionData
	encoded, err := json.Marshal(row.Session)
	if err == nil {
		err = json.Unmarshal(encoded, &session)
	}
	if err != nil {
		return row, webauthn.SessionData{}, fmt.Errorf("passkeys: decode the session: %w", err)
	}
	return row, session, nil
}

// Purge deletes ceremonies that expired before cutoff.
func (s *Service) Purge(ctx context.Context, cutoff time.Time) error {
	err := s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.Ceremony{}).Error
	if err != nil {
		return fmt.Errorf("passkeys: purge ceremonies: %w", err)
	}
	return nil
}

// digest is the stored form of a ceremony bearer.
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
