// Package breakglass restores an operator account from outside the front
// door, for the day the front door is the problem.
//
// SPEC §7 offered two shapes: a local administrator that does not federate,
// or Authentik kept warm as a fallback issuer. registre is the issuer, so it
// federates to nothing and the second shape has nothing to fall back to. The
// local administrator is therefore the only coherent answer, and this package
// is how one is restored: an offline operation, run by whoever already holds
// the database, that puts a named address back into a state it can log in
// from.
//
// What it does is deliberately narrow and deliberately loud. It re-enables
// the account, clears the second factor and every recovery code with it, sets
// a password, grants the admin role, and ends every live session. What it
// does not do is reach the network, mint a token, or leave a way in that
// outlives the call.
//
// It owns no routes, and it must never grow any. A break-glass path reachable
// over HTTP is not a break-glass path, it is a back door with a nicer name.
package breakglass

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

// MinPasswordLength is the floor for a break-glass password. It is higher
// than an ordinary one because this password is set under duress, by someone
// improvising, on the account that can reach everything.
const MinPasswordLength = 16

// Outcome reports what the operation changed, so the operator can see it
// happened rather than trusting that it did.
type Outcome struct {
	UserID string
	Email  string

	// Created is true when no account held the address and one was minted.
	Created bool

	// Reenabled is true when the account had been disabled.
	Reenabled bool

	// FactorCleared is true when a second factor was removed.
	FactorCleared bool

	// PromotedToAdmin is true when the account did not already hold admin.
	PromotedToAdmin bool

	// SessionsEnded counts the live sessions this closed.
	SessionsEnded int64
}

// Service performs the restore.
type Service struct {
	db       *gorm.DB
	verifier *credentials.Verifier
	now      func() time.Time
}

// New builds the service over db, hashing through verifier so a break-glass
// password is sealed exactly like one set through the login page.
func New(db *gorm.DB, verifier *credentials.Verifier) *Service {
	return &Service{db: db, verifier: verifier, now: func() time.Time { return time.Now().UTC() }}
}

// Restore puts email into a state it can sign in from and returns what
// changed.
//
// Every write happens in one transaction. A half-applied restore is the worst
// outcome available here: an account that is enabled with its factor cleared
// but its password unchanged is an account somebody else may now be able to
// reach, and the operator would have no way to tell from the error which half
// landed.
func (s *Service) Restore(ctx context.Context, email, password string) (Outcome, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return Outcome{}, fmt.Errorf("breakglass: %q is not an email address", email)
	}
	if len(password) < MinPasswordLength {
		return Outcome{}, fmt.Errorf(
			"breakglass: the password must be at least %d characters: this is the account that reaches "+
				"every app, set under pressure, and nobody improvises a good short one",
			MinPasswordLength)
	}
	hash, err := s.verifier.Hash(ctx, password)
	if err != nil {
		return Outcome{}, fmt.Errorf("breakglass: hash the password: %w", err)
	}
	var outcome Outcome
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		outcome, err = s.restore(tx, email, hash)
		return err
	})
	if err != nil {
		return Outcome{}, err
	}
	return outcome, nil
}

// restore applies the whole operation inside the caller's transaction.
func (s *Service) restore(tx *gorm.DB, email, hash string) (Outcome, error) {
	at := s.now()
	user, created, err := s.account(tx, email, hash, at)
	if err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{
		UserID: user.ID, Email: user.Email, Created: created,
		Reenabled: !created && !user.Enabled(), FactorCleared: !created && user.TOTPEnrolled(),
		PromotedToAdmin: !created && !user.Admin,
	}
	if !created {
		if err := clearAccount(tx, user.ID, hash); err != nil {
			return Outcome{}, err
		}
	}
	if err := tx.Where("user_id = ?", user.ID).Delete(&schemas.RecoveryCode{}).Error; err != nil {
		return Outcome{}, fmt.Errorf("breakglass: clear the recovery codes: %w", err)
	}
	ended, err := endSessions(tx, user.ID, at)
	if err != nil {
		return Outcome{}, err
	}
	outcome.SessionsEnded = ended
	return outcome, nil
}

// account reads the row behind email, minting one when the address holds no
// account at all. A restore that refused an unknown address would be useless
// in the case it exists for: the last administrator deleted.
func (s *Service) account(tx *gorm.DB, email, hash string, at time.Time) (schemas.User, bool, error) {
	var user schemas.User
	err := tx.Where("lower(email) = ?", email).Take(&user).Error
	if err == nil {
		return user, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.User{}, false, fmt.Errorf("breakglass: read the account: %w", err)
	}
	id, err := ids.New()
	if err != nil {
		return schemas.User{}, false, fmt.Errorf("breakglass: mint a subject: %w", err)
	}
	row := schemas.User{
		ID: id, Email: email, Name: email, PasswordHash: hash,
		Admin: true, EmailVerified: true, CreatedAt: at, UpdatedAt: at,
	}
	if err := tx.Create(&row).Error; err != nil {
		return schemas.User{}, false, fmt.Errorf("breakglass: create the account: %w", err)
	}
	return row, true, nil
}

// clearAccount sets the password, grants admin, lifts any disablement and
// removes the second factor in one statement.
func clearAccount(tx *gorm.DB, userID, hash string) error {
	err := tx.Model(&schemas.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password_hash": hash, "admin": true, "disabled_at": nil,
		"totp_secret": nil, "totp_nonce": nil,
		"totp_confirmed_at": nil, "totp_last_step": nil,
	}).Error
	if err != nil {
		return fmt.Errorf("breakglass: restore the account: %w", err)
	}
	return nil
}

// endSessions closes every live session the account holds, so a restore does
// not leave whoever caused the incident still signed in.
func endSessions(tx *gorm.DB, userID string, at time.Time) (int64, error) {
	err := tx.Model(&schemas.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
	if err != nil {
		return 0, fmt.Errorf("breakglass: revoke the refresh grants: %w", err)
	}
	result := tx.Model(&schemas.Session{}).
		Where("user_id = ? AND ended_at IS NULL", userID).
		Update("ended_at", at)
	if result.Error != nil {
		return 0, fmt.Errorf("breakglass: end the sessions: %w", result.Error)
	}
	return result.RowsAffected, nil
}
