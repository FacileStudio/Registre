package signup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// maxNameLength caps the display name.
const maxNameLength = 100

// ErrForm is a finish request missing a name, a supported locale, the
// acceptance or a password.
var ErrForm = errors.New("signup: incomplete form")

// ErrBreached is a password found in a known breach.
var ErrBreached = errors.New("signup: password appears in a known breach")

// Account is the finish form.
type Account struct {
	Token    string
	Name     string
	Locale   string
	Accepted bool
	IP       string
	Password string
}

// Created is the new account and the app login it may finish, which the
// caller completes only for the browser bound to it.
type Created struct {
	User        schemas.User
	AuthRequest string
}

// Finish validates the form, screens and hashes the password and creates
// the account, spending the link in the same transaction. A passkey is added
// later from the account page, never here.
func (s *Service) Finish(ctx context.Context, in Account) (Created, error) {
	if err := validate(in); err != nil {
		return Created{}, err
	}
	if _, err := s.live(ctx, s.DB, in.Token); err != nil {
		return Created{}, err
	}
	user, err := s.account(ctx, in)
	if err != nil {
		return Created{}, err
	}
	var created Created
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := s.redeem(ctx, tx, in.Token)
		if err != nil {
			return err
		}
		user.Email = row.Email
		created = Created{User: user, AuthRequest: row.AuthRequestID}
		return s.create(ctx, tx, &created.User, in)
	})
	if err != nil {
		return Created{}, err
	}
	return created, nil
}

// account screens and hashes the password and answers the account row to
// create, with a fresh subject.
func (s *Service) account(ctx context.Context, in Account) (schemas.User, error) {
	now := s.now()
	user := schemas.User{
		Name: strings.TrimSpace(in.Name), Locale: in.Locale,
		EmailVerified: true, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if s.Pwned.Breached(ctx, in.Password) {
		return user, ErrBreached
	}
	hash, err := s.Verifier.Hash(ctx, in.Password)
	if err != nil {
		return user, fmt.Errorf("signup: hash the password: %w", err)
	}
	user.PasswordHash = hash
	if user.ID, err = ids.New(); err != nil {
		return user, fmt.Errorf("signup: mint a subject: %w", err)
	}
	return user, nil
}

// redeem spends a live link inside tx. The conditional UPDATE is the
// single-use guarantee: two tabs finishing the same link see one row.
func (s *Service) redeem(ctx context.Context, tx *gorm.DB, token string) (schemas.SignupChallenge, error) {
	row, err := s.live(ctx, tx, token)
	if err != nil {
		return row, err
	}
	now := s.now()
	spent := tx.WithContext(ctx).Model(&schemas.SignupChallenge{}).
		Where("id = ? AND used_at IS NULL AND expires_at > ?", row.ID, now).
		Update("used_at", now)
	if spent.Error != nil {
		return row, fmt.Errorf("signup: spend the link: %w", spent.Error)
	}
	if spent.RowsAffected != 1 {
		return row, ErrInvalid
	}
	return row, nil
}

// create writes the account and its consents.
func (s *Service) create(ctx context.Context, tx *gorm.DB, user *schemas.User, in Account) error {
	if err := tx.WithContext(ctx).Create(user).Error; err != nil {
		if strings.Contains(err.Error(), "users_one_email") {
			return ErrInvalid
		}
		return fmt.Errorf("signup: create the account: %w", err)
	}
	return s.Consents.Record(ctx, tx, user.ID, in.Locale, in.IP)
}

// validate checks the form before any factor work happens.
func validate(in Account) error {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || !in.Accepted || !i18n.Supported(in.Locale) {
		return ErrForm
	}
	if in.Password == "" {
		return ErrForm
	}
	return credentials.CheckLength(in.Password)
}
