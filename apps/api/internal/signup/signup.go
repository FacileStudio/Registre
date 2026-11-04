// Package signup is public self-service account creation: "Create your
// Facile account".
//
// The flow is start, verify, finish. Start takes an address and an ALTCHA
// proof of work and always answers the same thing at the same speed,
// whether or not the address has an account: a known address is mailed "you
// already have an account, sign in instead", an unknown one a link. Both
// branches write one signup_challenges row (the known one born spent) and
// hand the mail to an asynchronous sender, so neither the database nor SMTP
// can tell them apart. The link carries a random token stored as SHA-256,
// good once, for thirty minutes. Finish creates the account behind a live
// token, with a passkey or a password, the person's name and language and
// their acceptance of the current CGU and privacy policy, in one
// transaction that also spends the token.
//
// It owns no routes; modules/signup does.
package signup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/address"
	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/pwned"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// LinkTTL is how long an emailed sign-up link stays good.
const LinkTTL = 30 * time.Minute

// ErrEmail is an address that is not shaped like one. It is decided from
// the string alone, so refusing it says nothing about who has an account.
var ErrEmail = errors.New("signup: not an email address")

// ErrThrottled is an address that already received MaxLinksPerHour mails in
// the last hour. It is counted from signup_challenges, which both branches
// write, so it says nothing about whether the address has an account, and
// only after a valid proof of work, so nobody can spend a stranger's budget
// with garbage requests.
var ErrThrottled = errors.New("signup: too many links for this address")

// MaxLinksPerHour is how many sign-up mails one address receives per hour.
const MaxLinksPerHour = 3

// ErrInvalid is an unknown, expired or spent link, under one name.
var ErrInvalid = errors.New("signup: invalid or expired link")

// Deps is what the service is built over.
type Deps struct {
	DB       *gorm.DB
	Verifier *credentials.Verifier
	Consents *consent.Service
	Passkeys *passkeys.Service
	Pwned    *pwned.Checker
	Mailer   recovery.Mailer
}

// Service runs the sign-up flow.
type Service struct {
	Deps
	key    string
	issuer string
	now    func() time.Time
}

// New builds the service. key signs ALTCHA challenges; issuer is where the
// mailed links point.
func New(deps Deps, key, issuer string) *Service {
	return &Service{
		Deps: deps, key: key, issuer: strings.TrimRight(issuer, "/"),
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Request is one start: the address, the proof of work, the language the
// mail is written in and the app login sign-up started from. The caller
// passes AuthRequest only once it has proven the starting browser holds
// that login's flow cookie (internal/flows); Finish checks the finishing
// browser again, so a link opened elsewhere lands on Registre's home.
type Request struct {
	Email       string
	Proof       string
	Locale      string
	AuthRequest string
}

// Start spends the proof of work, then writes one row and hands one mail to
// the sender whatever the address. Only ErrProof and ErrEmail are told to
// the caller, and neither depends on the address having an account.
func (s *Service) Start(ctx context.Context, in Request) error {
	if err := s.spend(ctx, in.Proof); err != nil {
		return err
	}
	email, ok := address.Clean(in.Email)
	if !ok {
		return ErrEmail
	}
	if err := s.underBudget(ctx, email); err != nil {
		return err
	}
	var existing schemas.User
	err := s.DB.WithContext(ctx).Where("lower(email) = ?", email).Take(&existing).Error
	known := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("signup: read the account: %w", err)
	}
	token, err := s.record(ctx, email, i18n.Normalize(in.Locale), in.AuthRequest, known)
	if err != nil {
		return err
	}
	if known {
		subject, body := i18n.Mail(i18n.MailSignupExisting, existing.Locale, s.issuer+"/login", nil)
		return s.Mailer.Send(ctx, existing.Email, subject, body)
	}
	link := s.issuer + "/signup/finish?token=" + url.QueryEscape(token)
	subject, body := i18n.Mail(i18n.MailSignupVerify, in.Locale, link, nil)
	return s.Mailer.Send(ctx, email, subject, body)
}

// record writes the link row. A row for a known address is spent at birth,
// so the insert both branches pay is the same insert.
func (s *Service) record(ctx context.Context, email, locale, authRequest string, spent bool) (string, error) {
	token, err := ids.Secret()
	if err != nil {
		return "", fmt.Errorf("signup: mint the token: %w", err)
	}
	id, err := ids.New()
	if err != nil {
		return "", fmt.Errorf("signup: mint the id: %w", err)
	}
	now := s.now()
	row := schemas.SignupChallenge{
		ID: id, Email: email, TokenHash: digest(token), Locale: locale,
		AuthRequestID: authRequest, ExpiresAt: now.Add(LinkTTL), CreatedAt: now,
	}
	if spent {
		row.UsedAt = &now
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("signup: record the link: %w", err)
	}
	return token, nil
}

// underBudget refuses an address that received MaxLinksPerHour links in the
// last hour.
func (s *Service) underBudget(ctx context.Context, email string) error {
	var recent int64
	err := s.DB.WithContext(ctx).Model(&schemas.SignupChallenge{}).
		Where("lower(email) = ? AND created_at > ?", email, s.now().Add(-time.Hour)).Count(&recent).Error
	if err != nil {
		return fmt.Errorf("signup: count recent links: %w", err)
	}
	if recent >= MaxLinksPerHour {
		return ErrThrottled
	}
	return nil
}

// Pending is what a live link says about the account it will create.
type Pending struct {
	Email  string `json:"email"`
	Locale string `json:"locale"`
}

// Verify answers what a live link is for, without spending it.
func (s *Service) Verify(ctx context.Context, token string) (Pending, error) {
	row, err := s.live(ctx, s.DB, token)
	if err != nil {
		return Pending{}, err
	}
	return Pending{Email: row.Email, Locale: row.Locale}, nil
}

// live reads a redeemable link whose address has not been taken since.
func (s *Service) live(ctx context.Context, tx *gorm.DB, token string) (schemas.SignupChallenge, error) {
	var row schemas.SignupChallenge
	if token == "" {
		return row, ErrInvalid
	}
	err := tx.WithContext(ctx).Where("token_hash = ?", digest(token)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ErrInvalid
	}
	if err != nil {
		return row, fmt.Errorf("signup: read the link: %w", err)
	}
	if !row.Redeemable(s.now()) {
		return row, ErrInvalid
	}
	var taken int64
	err = tx.WithContext(ctx).Model(&schemas.User{}).Where("lower(email) = ?", row.Email).Count(&taken).Error
	if err != nil {
		return row, fmt.Errorf("signup: read the account: %w", err)
	}
	if taken > 0 {
		return row, ErrInvalid
	}
	return row, nil
}

// Purge deletes spent proofs and links that expired before cutoff.
func (s *Service) Purge(ctx context.Context, cutoff time.Time) error {
	if err := s.DB.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.AltchaSpent{}).Error; err != nil {
		return fmt.Errorf("signup: purge proofs: %w", err)
	}
	if err := s.DB.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.SignupChallenge{}).Error; err != nil {
		return fmt.Errorf("signup: purge links: %w", err)
	}
	return nil
}

// digest is the stored form of a link token.
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
