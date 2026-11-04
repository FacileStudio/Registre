// Package recovery owns the emailed way back in for a human who has
// forgotten their password, under SPEC §7: "No enumeration in recovery. Same
// response and same timing whether the address is known."
//
// Request therefore answers nil either way and pays one argon2 operation
// through credentials.Verifier before anything branches on whether the
// address exists, so the branch that does nothing costs what the branch that
// mints a reset costs. Only the SHA-256 of the token is stored, exactly like
// an invitation; the plaintext crosses Request once, into the mail body.
//
// A reset is deliberately not an invitation. invites.Redeem creates the
// account and marks the address verified, because the invitation is the proof
// the address belongs to whoever holds the token. A reset proves only that
// the address still reaches someone who already owns an enabled row, so it
// sets password_hash and nothing else.
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrInvalid is every refusal Redeem can make — unknown token, expired,
// already spent, account gone or disabled — under one name. Distinguishing
// them for the caller distinguishes them for whoever is guessing.
var ErrInvalid = errors.New("recovery: invalid or expired reset")

// DefaultTTL is how long a reset link stays redeemable. Short by design: it
// is a credential sitting in an inbox.
const DefaultTTL = time.Hour

// Subject is the English subject line of the reset mail; the mail itself is
// sent in the account's locale through internal/i18n.
const Subject = "Reset your password"

// Mailer is the transport seam, kept to one method so a test needs no SMTP
// server and a caller can wrap a slow sender without implementing anything
// wide. facteur's Send takes a mailer.Message rather than these four
// arguments, so a caller adapts it with MailerFunc.
type Mailer interface {
	Send(ctx context.Context, to, subject, html string) error
}

// MailerFunc adapts a plain function to Mailer.
type MailerFunc func(ctx context.Context, to, subject, html string) error

// Send calls f.
func (f MailerFunc) Send(ctx context.Context, to, subject, html string) error {
	return f(ctx, to, subject, html)
}

// Service issues and redeems password resets.
type Service struct {
	db       *gorm.DB
	verifier *credentials.Verifier
	mailer   Mailer
	issuer   string
	now      func() time.Time
}

// New builds the service over db, hashing through verifier, mailing through
// mailer, and pointing its links at issuer.
func New(db *gorm.DB, verifier *credentials.Verifier, mailer Mailer, issuer string) *Service {
	return &Service{
		db: db, verifier: verifier, mailer: mailer,
		issuer: strings.TrimRight(issuer, "/"),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// Request mails a reset link to email when an enabled account holds it, and
// answers nil either way.
//
// The unconditional Verifier call is the equalisation: it costs the same
// argon2 work a login costs and sits before the branch, so an unknown address
// cannot answer in a microsecond where a known one spends sixty milliseconds.
// That cost also swamps the one INSERT the known path adds. What it cannot
// swamp is the mailer, so a caller whose Mailer blocks on an SMTP round trip
// must hand in an asynchronous one; that is why the interface is one method
// wide. A returned error is for the log only. The HTTP handler must answer
// the same thing whatever this returns, or the error is the oracle the
// timing is not.
func (s *Service) Request(ctx context.Context, email string) error {
	address := strings.ToLower(strings.TrimSpace(email))
	token, err := ids.Secret()
	if err != nil {
		return fmt.Errorf("recovery: mint the token: %w", err)
	}
	var row schemas.User
	err = s.db.WithContext(ctx).Where("lower(email) = ?", address).Take(&row).Error
	notFound := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !notFound {
		return fmt.Errorf("recovery: read the account: %w", err)
	}
	if _, err := s.verifier.Matches(ctx, token, ""); err != nil {
		return err
	}
	if notFound || !row.Enabled() {
		return nil
	}
	reset := schemas.PasswordReset{
		ID: newID(), UserID: row.ID, TokenHash: hash(token),
		ExpiresAt: s.now().Add(DefaultTTL), CreatedAt: s.now(),
	}
	if err := s.db.WithContext(ctx).Create(&reset).Error; err != nil {
		return fmt.Errorf("recovery: record the reset: %w", err)
	}
	subject, body := i18n.Mail(i18n.MailRecovery, row.Locale, s.link(token), nil)
	return s.mailer.Send(ctx, row.Email, subject, body)
}

// Redeem sets a new password behind a valid, unspent, unexpired token.
//
// The password is hashed before anything branches on whether the token was
// real, so an unknown token costs what a known one costs. The reset is spent
// first and the password written second, both inside one transaction: two
// browsers holding the same link race on the conditional UPDATE, and exactly
// one of them sees a row. Every other outstanding reset for the account is
// spent in the same transaction, because a password change is the moment the
// older links stop being wanted. Nothing here touches email_verified and
// nothing creates a missing account: a reset proves the address still
// reaches its owner, not that the address is theirs.
func (s *Service) Redeem(ctx context.Context, token, password string) error {
	var row schemas.PasswordReset
	err := s.db.WithContext(ctx).Where("token_hash = ?", hash(token)).Take(&row).Error
	notFound := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !notFound {
		return fmt.Errorf("recovery: read the reset: %w", err)
	}
	hashed, err := s.verifier.Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("recovery: hash the password: %w", err)
	}
	now := s.now()
	if notFound || token == "" || !row.Redeemable(now) {
		return ErrInvalid
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return redeem(tx, row, hashed, now)
	})
	if errors.Is(err, ErrInvalid) {
		return ErrInvalid
	}
	return err
}

// redeem spends the reset, writes the password and retires the account's
// other outstanding links, in the order that makes a lost race harmless: the
// loser fails on the conditional UPDATE before any password is written.
func redeem(tx *gorm.DB, row schemas.PasswordReset, hashed string, now time.Time) error {
	spent := tx.Model(&schemas.PasswordReset{}).
		Where("id = ? AND used_at IS NULL AND expires_at > ?", row.ID, now).
		Update("used_at", now)
	if spent.Error != nil {
		return fmt.Errorf("recovery: spend the reset: %w", spent.Error)
	}
	if spent.RowsAffected != 1 {
		return ErrInvalid
	}
	if err := setPassword(tx, row.UserID, hashed, now); err != nil {
		return err
	}
	retired := tx.Model(&schemas.PasswordReset{}).
		Where("user_id = ? AND used_at IS NULL", row.UserID).
		Update("used_at", now)
	if retired.Error != nil {
		return fmt.Errorf("recovery: retire the other resets: %w", retired.Error)
	}
	return endSessions(tx, row.UserID, now)
}

// endSessions closes every session and refresh grant the account holds, in
// the same transaction as the new password.
//
// A reset is what somebody does when they believe another person has their
// account, so leaving that person's sessions live is the reset failing at the
// one job it was asked to do. The new password would otherwise govern only
// the next login, which is the login the other person no longer needs.
//
// Back-channel logout is deliberately not fired from here. This package owns
// no routes and no keyring, and a notification is a side effect that must not
// sit inside the transaction that writes the password. The apps learn on
// their next token exchange, which is the same guarantee an expired session
// already gives them.
func endSessions(tx *gorm.DB, userID string, now time.Time) error {
	err := tx.Model(&schemas.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
	if err != nil {
		return fmt.Errorf("recovery: revoke the refresh grants: %w", err)
	}
	err = tx.Model(&schemas.Session{}).
		Where("user_id = ? AND ended_at IS NULL", userID).
		Update("ended_at", now).Error
	if err != nil {
		return fmt.Errorf("recovery: end the sessions: %w", err)
	}
	return nil
}

// setPassword writes the hash onto an account that already exists and is
// enabled. A missing or disabled row affects nothing and is refused: recovery
// does not create accounts and does not resurrect anyone.
func setPassword(tx *gorm.DB, userID, hashed string, now time.Time) error {
	result := tx.Model(&schemas.User{}).
		Where("id = ? AND disabled_at IS NULL", userID).
		Updates(map[string]any{"password_hash": hashed, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("recovery: set the password: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrInvalid
	}
	return nil
}

// link is the reset URL the mail carries, in the account's own language. The
// link is the whole message: a reset mail that says anything about the
// account it belongs to says it to whoever reads the inbox.
func (s *Service) link(token string) string {
	return s.issuer + "/recover/reset?token=" + url.QueryEscape(token)
}

func newID() string {
	id, _ := ids.New()
	return id
}

func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
