// Package clients is the register itself: who may ask registre for a token,
// what they authenticate with, and the ceiling on what they may ask for.
//
// In phase 0 every row is a service account — a machine principal with a
// subject of its own, which is the point of the exercise. An agent on a
// human's credential *is* that human in every audit trail, and revoking one
// revokes both.
package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrUnknownClient is returned for a client that does not exist, is disabled,
// or presented the wrong secret. It is deliberately one error for all three:
// saying which of the three it was is an enumeration oracle.
var ErrUnknownClient = errors.New("clients: unknown client or bad secret")

// Service reads and writes the register.
type Service struct {
	db          *gorm.DB
	credentials *credentials.Verifier
}

// New builds the service over db, hashing and verifying through verifier.
func New(db *gorm.DB, verifier *credentials.Verifier) *Service {
	return &Service{db: db, credentials: verifier}
}

// HashSecret seals a client secret for storage. The register owns client
// credentials, so it owns the one path that turns a plaintext into a hash.
func (s *Service) HashSecret(ctx context.Context, secret string) (string, error) {
	return s.credentials.Hash(ctx, secret)
}

// ByClientID returns an enabled client by the id it authenticates under.
func (s *Service) ByClientID(ctx context.Context, clientID string) (schemas.Client, error) {
	var row schemas.Client
	err := s.db.WithContext(ctx).Where("client_id = ?", clientID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.Client{}, ErrUnknownClient
	}
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: read %s: %w", clientID, err)
	}
	if !row.Enabled() {
		return schemas.Client{}, ErrUnknownClient
	}
	return row, nil
}

// Authenticate verifies a client secret and returns the client behind it.
//
// Every outcome runs the same argon2 verification. A missing client, a
// disabled one and a wrong secret all arrive here holding an empty hash, and
// an empty hash costs what a real one costs — the verifier guarantees that, so
// this function has no early return to forget.
func (s *Service) Authenticate(ctx context.Context, clientID, secret string) (schemas.Client, error) {
	row, err := s.ByClientID(ctx, clientID)
	if err != nil && !errors.Is(err, ErrUnknownClient) {
		return schemas.Client{}, err
	}
	matched, verifyErr := s.credentials.Matches(ctx, secret, row.SecretHash)
	if verifyErr != nil {
		return schemas.Client{}, verifyErr
	}
	if !matched {
		return schemas.Client{}, ErrUnknownClient
	}
	return row, nil
}

// List returns every client, oldest first.
func (s *Service) List(ctx context.Context) ([]schemas.Client, error) {
	var rows []schemas.Client
	if err := s.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("clients: list: %w", err)
	}
	return rows, nil
}

// declared names the columns a declaration owns, and is the whole of what an
// update writes.
//
// The list is explicit and the default is to leave a column alone, because the
// opposite default already produced a bug: writing the whole row cleared
// disabled_at, so an account disabled by hand came back on the next boot with
// its old secret. A revocation that undoes itself is worse than no revocation,
// having been reported as done. Adding a column to the model must not silently
// add it to what a re-apply overwrites.
func declared(row schemas.Client) map[string]any {
	fields := map[string]any{
		"name":             row.Name,
		"kind":             row.Kind,
		"grant_types":      row.GrantTypes,
		"scopes":           row.Scopes,
		"audiences":        row.Audiences,
		"access_token_ttl": row.AccessTokenTTL,
		"managed":          row.Managed,
		"updated_at":       time.Now().UTC(),
	}
	if row.SecretHash != "" {
		fields["secret_hash"] = row.SecretHash
	}
	return fields
}

// Upsert writes the declarative half of a client, keying on its client id.
//
// A row that already exists keeps its subject, because that is what every app
// matched the account on and re-keying it orphans the client everywhere it has
// ever authenticated. It keeps its disabled_at for the reason above, and its
// secret when the caller supplies none.
func (s *Service) Upsert(ctx context.Context, row schemas.Client) (schemas.Client, error) {
	existing, err := s.rawByClientID(ctx, row.ClientID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now().UTC()
		row.CreatedAt, row.UpdatedAt = now, now
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return schemas.Client{}, fmt.Errorf("clients: create %s: %w", row.ClientID, err)
		}
		return row, nil
	}
	if err != nil {
		return schemas.Client{}, err
	}

	err = s.db.WithContext(ctx).Model(&schemas.Client{}).
		Where("id = ?", existing.ID).Updates(declared(row)).Error
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: update %s: %w", row.ClientID, err)
	}
	return s.rawByClientID(ctx, row.ClientID)
}

// rawByClientID reads a row without the enabled filter, which is what a write
// path needs: a disabled client still exists and must not be created twice.
func (s *Service) rawByClientID(ctx context.Context, clientID string) (schemas.Client, error) {
	var row schemas.Client
	err := s.db.WithContext(ctx).Where("client_id = ?", clientID).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.Client{}, fmt.Errorf("clients: read %s: %w", clientID, err)
	}
	return row, err
}
