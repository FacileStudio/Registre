// Package invites owns the single-use tokens that carry SPEC §10 q1: a human
// is covered by an invitation, never by a seeded password hash. The token is
// returned once at issue and only its SHA-256 is stored, exactly like an
// authorization code; redemption sets the password through the argon2
// ceiling, so a redemption costs what a login costs.
package invites

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrInvalid is every refusal — unknown token, expired, already redeemed,
// bad password — under one name. Distinguishing them distinguishes them for
// whoever is guessing.
var ErrInvalid = errors.New("invites: invalid or expired invitation")

// ErrTooShort is a password under the floor every path that sets one shares.
// It is separate from ErrInvalid because the caller may safely say this one
// out loud: it describes what the person just typed, not whether their
// invitation exists.
var ErrTooShort = errors.New("invites: the password is too short")

// DefaultTTL is how long an invitation stays redeemable. Short by design:
// it is a credential in whatever channel carried it.
const DefaultTTL = 72 * time.Hour

// Service issues and redeems invitations.
type Service struct {
	db       *gorm.DB
	verifier *credentials.Verifier
	screen   credentials.Screen
	now      func() time.Time
}

// WithBreachCheck screens redeemed passwords against known breaches.
func (s *Service) WithBreachCheck(screen credentials.Screen) *Service {
	s.screen = screen
	return s
}

// New builds the service over db, hashing through verifier.
func New(db *gorm.DB, verifier *credentials.Verifier) *Service {
	return &Service{db: db, verifier: verifier, now: func() time.Time { return time.Now().UTC() }}
}

// Issue mints an invitation for email created by operatorID and returns the
// plaintext token. The plaintext crosses this call exactly once; only its
// digest is stored.
func (s *Service) Issue(ctx context.Context, email, createdBy string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return "", fmt.Errorf("invites: %q is not an email address", email)
	}
	var existing schemas.User
	err := s.db.WithContext(ctx).Where("lower(email) = ?", email).Take(&existing).Error
	if err == nil && !existing.Enabled() {
		return "", fmt.Errorf("invites: %s is disabled", email)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("invites: read the account: %w", err)
	}
	token, err := ids.Secret()
	if err != nil {
		return "", fmt.Errorf("invites: mint the token: %w", err)
	}
	digest, err := hash(token)
	if err != nil {
		return "", err
	}
	row := schemas.InviteToken{
		ID: newID(), Email: email, TokenHash: digest, CreatedBy: createdBy,
		ExpiresAt: s.now().Add(DefaultTTL), CreatedAt: s.now(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("invites: record the invitation: %w", err)
	}
	return token, nil
}

// List returns every invitation, newest first.
func (s *Service) List(ctx context.Context) ([]schemas.InviteToken, error) {
	var rows []schemas.InviteToken
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("invites: list: %w", err)
	}
	return rows, nil
}

// Redeem sets the password behind a valid, unredeemed, unexpired invitation.
//
// The password is hashed before anything branches on whether the token was
// real, so an unknown token costs what a known one costs. The account the
// address names is created when absent — the invitation is the proof the
// address belongs to whoever holds the token — and marked verified for the
// same reason. A disabled account keeps its disabled_at: an invitation does
// not resurrect anyone.
func (s *Service) Redeem(ctx context.Context, token, password string) error {
	if token == "" {
		password = ""
	}
	digest, err := hash(token)
	if err != nil {
		return err
	}
	var row schemas.InviteToken
	err = s.db.WithContext(ctx).Where("token_hash = ?", digest).Take(&row).Error
	notFound := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !notFound {
		return fmt.Errorf("invites: read the invitation: %w", err)
	}
	hashed, err := s.verifier.Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("invites: hash the password: %w", err)
	}
	if notFound || row.RedeemedAt != nil || !row.ExpiresAt.After(s.now()) {
		return ErrInvalid
	}
	if credentials.CheckLength(password) != nil {
		return ErrTooShort
	}
	if err := credentials.CheckNew(ctx, s.screen, password); err != nil {
		return err
	}
	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := setPassword(tx, row.Email, hashed); err != nil {
			return err
		}
		result := tx.Model(&schemas.InviteToken{}).
			Where("id = ? AND redeemed_at IS NULL AND expires_at > ?", row.ID, now).
			Update("redeemed_at", now)
		if result.Error != nil {
			return fmt.Errorf("invites: redeem the invitation: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrInvalid
		}
		return nil
	})
	if errors.Is(err, ErrInvalid) {
		return ErrInvalid
	}
	return err
}

func newID() string {
	id, _ := ids.New()
	return id
}

func setPassword(tx *gorm.DB, email, hash string) error {
	var existing schemas.User
	err := tx.Where("lower(email) = ?", email).Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("invites: read the account: %w", err)
	}
	if err == nil && !existing.Enabled() {
		return ErrInvalid
	}
	result := tx.Model(&schemas.User{}).
		Where("lower(email) = ?", email).
		Updates(map[string]any{"password_hash": hash, "email_verified": true})
	if result.Error != nil {
		return fmt.Errorf("invites: set the password: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}
	row := schemas.User{
		ID: newID(), Email: email, Name: localPart(email),
		PasswordHash: hash, EmailVerified: true,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := tx.Create(&row).Error; err != nil {
		return fmt.Errorf("invites: create the user: %w", err)
	}
	return nil
}

func localPart(email string) string {
	if name, _, found := strings.Cut(email, "@"); found && name != "" {
		return name
	}
	return email
}

func hash(token string) (string, error) {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), nil
}
