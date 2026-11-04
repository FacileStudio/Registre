package signup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// maxNameLength caps the display name.
const maxNameLength = 100

// ErrForm is a finish request missing a name, a supported locale, the
// acceptance, or exactly one way to sign in.
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

	// Password, or Ceremony and Credential for a passkey: exactly one.
	Password   string
	Ceremony   string
	Credential json.RawMessage
}

// Created is the new account and the app login it may finish, which the
// caller completes only for the browser bound to it.
type Created struct {
	User        schemas.User
	AuthRequest string
}

// BeginPasskey starts registering the passkey the account will be created
// with. The subject is minted now and travels inside the ceremony, so the
// account Finish creates is the one the passkey was bound to.
func (s *Service) BeginPasskey(ctx context.Context, token string) (passkeys.Options, error) {
	row, err := s.live(ctx, s.DB, token)
	if err != nil {
		return passkeys.Options{}, err
	}
	subject, err := ids.New()
	if err != nil {
		return passkeys.Options{}, fmt.Errorf("signup: mint a subject: %w", err)
	}
	return s.Passkeys.BeginRegistration(ctx, passkeys.PurposeSignup, subject, row.Email)
}

// Finish validates the form, proves the chosen factor and creates the
// account, spending the link in the same transaction.
func (s *Service) Finish(ctx context.Context, in Account) (Created, error) {
	if err := validate(in); err != nil {
		return Created{}, err
	}
	if _, err := s.live(ctx, s.DB, in.Token); err != nil {
		return Created{}, err
	}
	user, registered, err := s.factor(ctx, in)
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
		return s.create(ctx, tx, &created.User, registered, in)
	})
	if err != nil {
		return Created{}, err
	}
	return created, nil
}

// factor proves the passkey or screens and hashes the password, and answers
// the account row to create with its subject and credential set.
func (s *Service) factor(ctx context.Context, in Account) (schemas.User, *passkeys.Registered, error) {
	now := s.now()
	user := schemas.User{
		Name: strings.TrimSpace(in.Name), Locale: in.Locale,
		EmailVerified: true, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if in.Ceremony != "" {
		registered, err := s.Passkeys.FinishRegistration(ctx, in.Ceremony, passkeys.PurposeSignup, in.Credential)
		if err != nil {
			return user, nil, err
		}
		user.ID = registered.UserID
		return user, &registered, nil
	}
	if s.Pwned.Breached(ctx, in.Password) {
		return user, nil, ErrBreached
	}
	hash, err := s.Verifier.Hash(ctx, in.Password)
	if err != nil {
		return user, nil, fmt.Errorf("signup: hash the password: %w", err)
	}
	user.PasswordHash = hash
	if user.ID, err = ids.New(); err != nil {
		return user, nil, fmt.Errorf("signup: mint a subject: %w", err)
	}
	return user, nil, nil
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

// create writes the account, its consents and its passkey.
func (s *Service) create(ctx context.Context, tx *gorm.DB, user *schemas.User, reg *passkeys.Registered, in Account) error {
	if err := tx.WithContext(ctx).Create(user).Error; err != nil {
		if strings.Contains(err.Error(), "users_one_email") {
			return ErrInvalid
		}
		return fmt.Errorf("signup: create the account: %w", err)
	}
	if err := s.Consents.Record(ctx, tx, user.ID, in.Locale, in.IP); err != nil {
		return err
	}
	if reg == nil {
		return nil
	}
	_, err := s.Passkeys.Store(ctx, tx, *reg, "")
	return err
}

// validate checks the form before any factor work happens.
func validate(in Account) error {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || !in.Accepted || !i18n.Supported(in.Locale) {
		return ErrForm
	}
	hasPasskey := in.Ceremony != "" && len(in.Credential) > 0
	if hasPasskey == (in.Password != "") {
		return ErrForm
	}
	if hasPasskey {
		return nil
	}
	return credentials.CheckLength(in.Password)
}
