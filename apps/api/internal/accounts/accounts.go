// Package accounts is the session a human holds with registre itself.
//
// Every other app in the suite gets its session from registre. registre
// cannot, because it is the issuer, so the account pages (enrol a second
// factor, regenerate recovery codes, change a password) need a session of
// their own. This is that session, and it is deliberately thin: a random
// bearer stored as a digest, hung off the provider session row that login
// already creates.
//
// Hanging it off that row is the whole design. end_session, `registre
// user-disable` and back-channel logout all end sessions, and because the
// cookie is a column on the row they end, all three close the account pages
// too. A session table of registre's own would have been a second thing to
// remember to revoke, and the revocation nobody remembers is the one that
// matters.
//
// It owns no routes. modules/account owns its HTTP.
package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrNoSession is an absent, unknown, ended or expired cookie, under one
// name. The caller answers all of them with the same 401.
var ErrNoSession = errors.New("accounts: no live session")

// CookieName is the account session cookie. The prefix is not decoration:
// __Host- is refused by the browser unless the cookie is Secure, path "/" and
// carries no Domain, so a misconfiguration that would have widened it to a
// sibling hostname fails at the browser instead of shipping.
const CookieName = "__Host-registre_account"

// DevCookieName is what a plain-http development issuer gets, because
// __Host- requires Secure and a development instance has no certificate.
const DevCookieName = "registre_account"

// CSRFHeader must be present on every mutating account request. It is the
// same header porte requires, for the same reason: a cookie is sent by the
// browser whether or not the page that triggered the request was ours, and a
// custom header cannot be set cross-origin without a preflight we do not
// answer.
const CSRFHeader = "X-Facile-CSRF"

// Lifetime is how long an account cookie stays good. It is short because the
// account pages are where a second factor is turned off: a stale laptop
// session should not still be able to do that a week later.
const Lifetime = 12 * time.Hour

// Notifier tells the apps a session reached that it has ended.
// logout.Service is the production one.
type Notifier interface {
	Fire(ctx context.Context, userID string, clientIDs []string) error
}

// Service mints and resolves account sessions.
type Service struct {
	db       *gorm.DB
	now      func() time.Time
	notifier Notifier
	logger   *slog.Logger
}

// New builds the service over db.
func New(db *gorm.DB) *Service {
	return &Service{db: db, now: func() time.Time { return time.Now().UTC() }, logger: slog.Default()}
}

// WithNotifier makes SignOut and Replace tell the apps a session reached.
// Without one they end the session and tell nobody.
func (s *Service) WithNotifier(notifier Notifier, logger *slog.Logger) *Service {
	s.notifier = notifier
	if logger != nil {
		s.logger = logger
	}
	return s
}

// Issue attaches a fresh cookie value to an existing provider session and
// returns the plaintext, which crosses this call exactly once.
func (s *Service) Issue(ctx context.Context, sessionID string) (string, error) {
	token, err := ids.Secret()
	if err != nil {
		return "", fmt.Errorf("accounts: mint the session token: %w", err)
	}
	digest := hash(token)
	result := s.db.WithContext(ctx).Model(&schemas.Session{}).
		Where("id = ? AND ended_at IS NULL", sessionID).
		Update("token_hash", digest)
	if result.Error != nil {
		return "", fmt.Errorf("accounts: attach the session token: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return "", ErrNoSession
	}
	return token, nil
}

// Resolve returns the human behind a cookie value, or ErrNoSession.
//
// It refuses a session older than Lifetime and an account that has since been
// disabled. The disabled check is here rather than left to callers because
// every account route needs it and one that forgets is a route a disabled
// human can still reach.
func (s *Service) Resolve(ctx context.Context, token string) (schemas.User, schemas.Session, error) {
	if token == "" {
		return schemas.User{}, schemas.Session{}, ErrNoSession
	}
	var session schemas.Session
	err := s.db.WithContext(ctx).
		Where("token_hash = ? AND ended_at IS NULL", hash(token)).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.User{}, schemas.Session{}, ErrNoSession
	}
	if err != nil {
		return schemas.User{}, schemas.Session{}, fmt.Errorf("accounts: read the session: %w", err)
	}
	if !session.CreatedAt.Add(Lifetime).After(s.now()) {
		return schemas.User{}, schemas.Session{}, ErrNoSession
	}
	var user schemas.User
	err = s.db.WithContext(ctx).Where("id = ?", session.UserID).Take(&user).Error
	if err != nil {
		return schemas.User{}, schemas.Session{}, fmt.Errorf("accounts: read the user: %w", err)
	}
	if !user.Enabled() {
		return schemas.User{}, schemas.Session{}, ErrNoSession
	}
	return user, session, nil
}

// hash digests a session token for storage and lookup.
func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
