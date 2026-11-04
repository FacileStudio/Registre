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
	consents ConsentChecker
}

// New builds the service over db, hashing and verifying through verifier.
func New(db *gorm.DB, verifier *credentials.Verifier) *Service {
	return &Service{db: db, verifier: verifier}
}

// Create mints a user from an email and a plaintext password. The password is
// hashed here, under the ceiling; nothing stores a plaintext past this call.
//
// The account is marked email-verified on creation: today every account is
// minted by an operator, who attests the address by creating it, and truthful
// emission of the claim is worth nothing if the only truth it can tell is no.
// Self-service signup, when it arrives, creates unverified accounts instead.

// NewUser carries the fields for a fresh account.
type NewUser struct {
	Email    string
	Password string
	Name     string
	Admin    bool
}

func (s *Service) Create(ctx context.Context, in NewUser) (schemas.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return schemas.User{}, fmt.Errorf("identity: %q is not an email address", email)
	}
	hash, err := s.verifier.Hash(ctx, in.Password)
	if err != nil {
		return schemas.User{}, fmt.Errorf("identity: hash the password: %w", err)
	}
	subject, err := ids.New()
	if err != nil {
		return schemas.User{}, fmt.Errorf("identity: mint a subject: %w", err)
	}
	row := schemas.User{
		ID: subject, Email: email, Name: in.Name,
		PasswordHash: hash, Admin: in.Admin,
		EmailVerified: true,
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

// ByEmail reads one user by address, case-insensitively.
func (s *Service) ByEmail(ctx context.Context, email string) (schemas.User, error) {
	var row schemas.User
	err := s.db.WithContext(ctx).
		Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(email))).
		Take(&row).Error
	if err != nil {
		return schemas.User{}, fmt.Errorf("identity: read the user: %w", err)
	}
	return row, nil
}

func (s *Service) Disable(ctx context.Context, userID string, at time.Time) ([]string, error) {
	result := s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ? AND disabled_at IS NULL", userID).
		Update("disabled_at", at)
	if result.Error != nil {
		return nil, fmt.Errorf("identity: disable the user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := s.db.WithContext(ctx).Model(&schemas.User{}).
			Where("id = ?", userID).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("identity: read the user: %w", err)
		}
		if count == 0 {
			return nil, fmt.Errorf("identity: no such user %q", userID)
		}
	}
	return s.EndSessions(ctx, userID, at)
}

func (s *Service) Enable(ctx context.Context, userID string) error {
	result := s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{"disabled_at": nil, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("identity: enable the user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("identity: no such user %q", userID)
	}
	return nil
}

func (s *Service) SetPassword(ctx context.Context, userID, password string) error {
	hash, err := s.verifier.Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("identity: hash the password: %w", err)
	}
	result := s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{"password_hash": hash, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("identity: store the password: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("identity: no such user %q", userID)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, userID, name string, admin *bool) (schemas.User, error) {
	updates := map[string]any{
		"name":       name,
		"updated_at": time.Now().UTC(),
	}
	if admin != nil {
		updates["admin"] = *admin
	}
	result := s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ?", userID).
		Updates(updates)
	if result.Error != nil {
		return schemas.User{}, fmt.Errorf("identity: update the user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return schemas.User{}, fmt.Errorf("identity: no such user %q", userID)
	}
	return s.ByID(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	if _, err := s.EndSessions(ctx, userID, now); err != nil {
		return fmt.Errorf("identity: end sessions: %w", err)
	}
	_ = s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&schemas.AuthRequest{}).Error
	result := s.db.WithContext(ctx).Where("id = ?", userID).Delete(&schemas.User{})
	if result.Error != nil {
		return fmt.Errorf("identity: delete user: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("identity: no such user %q", userID)
	}
	return nil
}
