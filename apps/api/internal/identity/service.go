// Package identity owns the human half of the register: creating users and
// verifying a password against one.
//
// Verification goes through credentials.Verifier, so it sits under the same
// process-wide argon2 ceiling as every client-secret check, and the
// account-existence timing equalisation is folded in there rather than left
// to this package to remember.
package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrInvalid is what a wrong email, a wrong password and a disabled account
// all answer with. One error for all three is the point: distinguishing them
// for the caller distinguishes them for whoever is guessing.
var ErrInvalid = errors.New("identity: invalid credentials")

// Service owns user rows: creating them and verifying a password against one.
type Service struct {
	db       *gorm.DB
	verifier *credentials.Verifier
}

// New builds the service over db, hashing and verifying through verifier.
func New(db *gorm.DB, verifier *credentials.Verifier) *Service {
	return &Service{db: db, verifier: verifier}
}

// Create mints a user from an email and a plaintext password. The password is
// hashed here, under the ceiling; nothing stores a plaintext past this call.
func (s *Service) Create(ctx context.Context, email, password, name string, admin bool) (schemas.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return schemas.User{}, fmt.Errorf("identity: %q is not an email address", email)
	}
	hash, err := s.verifier.Hash(ctx, password)
	if err != nil {
		return schemas.User{}, fmt.Errorf("identity: hash the password: %w", err)
	}
	subject, err := ids.New()
	if err != nil {
		return schemas.User{}, fmt.Errorf("identity: mint a subject: %w", err)
	}
	row := schemas.User{
		ID: subject, Email: email, Name: name,
		PasswordHash: hash, Admin: admin,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schemas.User{}, fmt.Errorf("identity: create the user: %w", err)
	}
	return row, nil
}

// Verify answers whether email and password open an account, and which one.
//
// The empty-hash branch of Verifier.Matches already costs what a real check
// costs, so an unknown address takes as long as a wrong password. A disabled
// account verifies normally first and is refused after: refusing early on
// `disabled_at` would tell a guesser it has found a real address.
func (s *Service) Verify(ctx context.Context, email, password string) (schemas.User, error) {
	var row schemas.User
	err := s.db.WithContext(ctx).
		Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(email))).
		Take(&row).Error
	notFound := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !notFound {
		return schemas.User{}, fmt.Errorf("identity: read the user: %w", err)
	}
	hash := ""
	if !notFound {
		hash = row.PasswordHash
	}
	match, err := s.verifier.Matches(ctx, password, hash)
	if err != nil {
		return schemas.User{}, err
	}
	if notFound || !match || !row.Enabled() {
		return schemas.User{}, ErrInvalid
	}
	return row, nil
}

// ByID reads one user by subject.
func (s *Service) ByID(ctx context.Context, id string) (schemas.User, error) {
	var row schemas.User
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		return schemas.User{}, fmt.Errorf("identity: read the user: %w", err)
	}
	return row, nil
}

// PurgeAuthRequests drops browser flows that expired before cutoff.
func (s *Service) PurgeAuthRequests(ctx context.Context, cutoff time.Time) error {
	err := s.db.WithContext(ctx).
		Where("expires_at < ?", cutoff).
		Delete(&schemas.AuthRequest{}).Error
	if err != nil {
		return fmt.Errorf("identity: purge auth requests: %w", err)
	}
	return nil
}

// AuthRequestByID reads a pending browser flow for the login UI.
func (s *Service) AuthRequestByID(ctx context.Context, id string) (schemas.AuthRequest, error) {
	var row schemas.AuthRequest
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		return schemas.AuthRequest{}, fmt.Errorf("identity: read the auth request: %w", err)
	}
	return row, nil
}

// Complete marks a browser flow as authenticated by user. It updates only a
// row that is still pending and unexpired: two tabs sharing one login page,
// or a slow submit after the flow expired, must not authenticate the wrong
// request.
//
// The affected-rows check is deliberate — GORM returns no error when nothing
// matched, and an expired flow answering as if it had succeeded would then
// redirect into a callback that refuses it, which looks like registre broke.
func (s *Service) Complete(ctx context.Context, id, userID string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&schemas.AuthRequest{}).
		Where("id = ? AND user_id IS NULL AND expires_at > ?", id, at).
		Updates(map[string]any{"user_id": userID, "authenticated_at": at})
	if result.Error != nil {
		return fmt.Errorf("identity: complete the login: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrInvalid
	}
	return nil
}
