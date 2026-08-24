package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// Create mints a client row from a partially-filled declaration — ClientID,
// Name, Kind and any GrantTypes or Scopes — and returns the stored row with
// the plaintext secret, which crosses this call exactly once — the same
// discipline the seed follows of naming a credential rather than storing it.
func (s *Service) Create(ctx context.Context, declared schemas.Client) (schemas.Client, string, error) {
	secret, err := ids.Secret()
	if err != nil {
		return schemas.Client{}, "", fmt.Errorf("clients: mint the secret: %w", err)
	}
	hashed, err := s.HashSecret(ctx, secret)
	if err != nil {
		return schemas.Client{}, "", fmt.Errorf("clients: hash the secret: %w", err)
	}
	subject, err := ids.New()
	if err != nil {
		return schemas.Client{}, "", fmt.Errorf("clients: mint a subject: %w", err)
	}
	now := time.Now().UTC()
	row := declared
	row.Subject, row.SecretHash = subject, hashed
	row.CreatedAt, row.UpdatedAt = now, now
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schemas.Client{}, "", fmt.Errorf("clients: create %s: %w", row.ClientID, err)
	}
	return row, secret, nil
}

// RotateSecret replaces a client's secret and returns the new plaintext,
// shown once. The rotation goes through Upsert's declared columns, so nothing
// but secret_hash and updated_at moves.
func (s *Service) RotateSecret(ctx context.Context, clientID string) (schemas.Client, string, error) {
	existing, err := s.rawByClientID(ctx, clientID)
	if err != nil {
		return schemas.Client{}, "", err
	}
	secret, err := ids.Secret()
	if err != nil {
		return schemas.Client{}, "", fmt.Errorf("clients: mint the secret: %w", err)
	}
	hashed, err := s.HashSecret(ctx, secret)
	if err != nil {
		return schemas.Client{}, "", fmt.Errorf("clients: hash the secret: %w", err)
	}
	existing.SecretHash = hashed
	row, err := s.Upsert(ctx, existing)
	return row, secret, err
}
