package social

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

const maxNameLength = 100

var errRaced = errors.New("social: a concurrent sign-in won the race")

// Resolve turns a provider profile into the Registre user it signs in.
//
// The rules run in order and the first that holds wins. An identity already
// linked signs in its user. Otherwise a verified email equal to an existing
// user's address links the identity to that user: the provider vouched for the
// address, which is the proof the account's own sign-up asked for. Otherwise a
// verified email creates a new account, with no password, but only while
// public sign-up is open. A disabled user is refused on every path.
func (s *Service) Resolve(ctx context.Context, provider string, profile Profile) (schemas.User, error) {
	user, err := s.resolveOnce(ctx, provider, profile)
	if errors.Is(err, errRaced) {
		return s.resolveOnce(ctx, provider, profile)
	}
	return user, err
}

// resolveOnce is one pass of the rules.
func (s *Service) resolveOnce(ctx context.Context, provider string, profile Profile) (schemas.User, error) {
	user, found, err := s.linkedUser(ctx, provider, profile.Subject)
	if err != nil || found {
		return s.checked(user, err)
	}
	if !profile.EmailVerified || profile.Email == "" {
		return schemas.User{}, ErrUnverifiedEmail
	}
	existing, err := s.userByEmail(ctx, profile.Email)
	if err == nil {
		if !existing.Enabled() {
			return schemas.User{}, ErrDisabled
		}
		if err := s.insertIdentity(ctx, s.db, existing.ID, provider, profile); err != nil {
			return schemas.User{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.User{}, err
	}
	if !s.cfg.SignupOpen {
		return schemas.User{}, ErrSignupClosed
	}
	return s.create(ctx, provider, profile)
}

// checked refuses a disabled user and passes any earlier error through.
func (s *Service) checked(user schemas.User, err error) (schemas.User, error) {
	if err != nil {
		return schemas.User{}, err
	}
	if !user.Enabled() {
		return schemas.User{}, ErrDisabled
	}
	return user, nil
}

// linkedUser is the user an identity is linked to, and whether one is.
func (s *Service) linkedUser(ctx context.Context, provider, subject string) (schemas.User, bool, error) {
	var identity schemas.SocialIdentity
	err := s.db.WithContext(ctx).Where("provider = ? AND subject = ?", provider, subject).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.User{}, false, nil
	}
	if err != nil {
		return schemas.User{}, false, fmt.Errorf("social: read the identity: %w", err)
	}
	var user schemas.User
	if err := s.db.WithContext(ctx).Where("id = ?", identity.UserID).Take(&user).Error; err != nil {
		return schemas.User{}, false, fmt.Errorf("social: read the linked user: %w", err)
	}
	return user, true, nil
}

// userByEmail reads a user by address, ignoring case.
func (s *Service) userByEmail(ctx context.Context, email string) (schemas.User, error) {
	var user schemas.User
	err := s.db.WithContext(ctx).Where("lower(email) = ?", strings.ToLower(email)).Take(&user).Error
	return user, err
}

// create writes a new passwordless account and its identity in one
// transaction. The address is verified because the provider vouched for it.
func (s *Service) create(ctx context.Context, provider string, profile Profile) (schemas.User, error) {
	id, err := ids.New()
	if err != nil {
		return schemas.User{}, fmt.Errorf("social: mint a subject: %w", err)
	}
	now := s.now()
	user := schemas.User{
		ID: id, Email: profile.Email, Name: displayName(profile),
		EmailVerified: true, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := s.insertIdentity(ctx, tx, user.ID, provider, profile); err != nil {
			return err
		}
		return audit.Write(tx.WithContext(ctx), user.ID, "account.created_social", audit.Target{Type: "user", ID: user.ID}, map[string]any{"provider": provider})
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, errRaced) {
			return schemas.User{}, errRaced
		}
		return schemas.User{}, fmt.Errorf("social: create the account: %w", err)
	}
	return user, nil
}

// insertIdentity links profile to userID. A subject someone else linked in the
// meantime is errRaced so the caller reads it again; a user who already holds
// another account of the provider is ErrAlreadyLinked.
func (s *Service) insertIdentity(
	ctx context.Context, db *gorm.DB, userID, provider string, profile Profile,
) error {
	row := schemas.SocialIdentity{
		Provider: provider, Subject: profile.Subject, UserID: userID,
		DisplayName: displayName(profile), CreatedAt: s.now(),
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return audit.Write(tx, userID, "account.connection_added", audit.Target{Type: "user", ID: userID}, map[string]any{"provider": provider})
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return s.duplicateCause(ctx, userID, provider, profile.Subject)
	}
	return fmt.Errorf("social: link the identity: %w", err)
}

// duplicateCause says which unique index an insert hit. It reads outside the
// failed statement's transaction, which Postgres has aborted: another request
// linking the subject first is errRaced, and a user already holding another
// account of the provider is ErrAlreadyLinked.
func (s *Service) duplicateCause(ctx context.Context, userID, provider, subject string) error {
	var taken int64
	err := s.db.WithContext(ctx).Model(&schemas.SocialIdentity{}).
		Where("provider = ? AND subject = ?", provider, subject).Count(&taken).Error
	if err != nil {
		return fmt.Errorf("social: read the identity: %w", err)
	}
	if taken > 0 {
		return errRaced
	}
	return ErrAlreadyLinked
}

// displayName is the name shown for a profile: its name, or the part of the
// address before the @, cut to the length the account page allows.
func displayName(profile Profile) string {
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name, _, _ = strings.Cut(profile.Email, "@")
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		name = string([]rune(name)[:maxNameLength])
	}
	return name
}
