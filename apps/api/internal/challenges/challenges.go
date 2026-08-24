// Package challenges owns the state between a password that checked out and
// the second factor still owed on it.
//
// The state has to live somewhere, and the browser is the wrong somewhere:
// handing the client "this password was right" is handing it the first
// factor, and then the second one guards nothing. So it is a row, keyed by a
// bearer token whose SHA-256 is all that is stored, exactly like an
// invitation or an authorization code. The plaintext crosses Issue once and
// comes back in a JSON body, never a URL. SPEC §7.
//
// It owns no routes, so it lives under internal/ by the same rule that keeps
// keys and clients there. modules/login owns its HTTP.
package challenges

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

// ErrInvalid is every refusal this package makes: an unknown token, an
// expired one, one already spent, one out of attempts. One error for all of
// them, because distinguishing them for the caller distinguishes them for
// whoever is guessing.
var ErrInvalid = errors.New("challenges: invalid or expired challenge")

// DefaultTTL is how long a challenge stays answerable. Five minutes is the
// window a human needs to read six digits off a phone, and nothing more: the
// row is a password already proven, so it is a credential in its own right.
const DefaultTTL = 5 * time.Minute

// MaxAttempts is how many codes may be tried against one challenge. A
// six-digit code inside totp's skew window is roughly one guess in 3x10^5, so
// an unbounded challenge is a guessable one; five attempts leaves the
// arithmetic hopeless and still forgives a typo.
const MaxAttempts = 5

// Service issues, claims and consumes login challenges.
type Service struct {
	db  *gorm.DB
	now func() time.Time
}

// New builds the service over db.
func New(db *gorm.DB) *Service {
	return &Service{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// Issue mints a challenge for userID against authRequestID and returns the
// plaintext token, which crosses this call exactly once.
//
// The challenge is bound to the browser flow it was minted for, so a
// challenge earned on one login cannot complete another that happens to be
// pending in a second tab.
func (s *Service) Issue(ctx context.Context, userID, authRequestID string) (string, error) {
	token, err := ids.Secret()
	if err != nil {
		return "", fmt.Errorf("challenges: mint the token: %w", err)
	}
	id, err := ids.New()
	if err != nil {
		return "", fmt.Errorf("challenges: mint the challenge id: %w", err)
	}
	row := schemas.LoginChallenge{
		ID: id, AuthRequestID: authRequestID, UserID: userID,
		TokenHash: hash(token), ExpiresAt: s.now().Add(DefaultTTL), CreatedAt: s.now(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("challenges: record the challenge: %w", err)
	}
	return token, nil
}

// Claim burns one attempt and returns the challenge if it is still
// answerable.
//
// The attempt is charged by a conditional update that carries the whole
// answerable test in its WHERE clause, and the row is read back only after
// the charge landed. Reading first and writing second is the bug this shape
// exists to avoid: a guesser running requests in parallel would have every
// one of them read the same attempts value and pass the budget check, so a
// budget of five would buy as many tries as the attacker had connections.
// Postgres serialises the counter for us, and RowsAffected reports whether
// this caller is the one who got it.
func (s *Service) Claim(ctx context.Context, token string) (schemas.LoginChallenge, error) {
	if token == "" {
		return schemas.LoginChallenge{}, ErrInvalid
	}
	digest := hash(token)
	result := s.db.WithContext(ctx).Model(&schemas.LoginChallenge{}).
		Where("token_hash = ? AND consumed_at IS NULL AND expires_at > ? AND attempts < ?",
			digest, s.now(), MaxAttempts).
		Update("attempts", gorm.Expr("attempts + 1"))
	if result.Error != nil {
		return schemas.LoginChallenge{}, fmt.Errorf("challenges: charge the attempt: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return schemas.LoginChallenge{}, ErrInvalid
	}
	var row schemas.LoginChallenge
	if err := s.db.WithContext(ctx).Where("token_hash = ?", digest).Take(&row).Error; err != nil {
		return schemas.LoginChallenge{}, fmt.Errorf("challenges: read the challenge: %w", err)
	}
	return row, nil
}

// Consume marks a challenge spent so it cannot be answered twice.
//
// The affected-rows check is what makes that true: two tabs answering the
// same challenge with the same good code both reach here, and only the one
// whose update moved a row may go on to open a session.
func (s *Service) Consume(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Model(&schemas.LoginChallenge{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Update("consumed_at", s.now())
	if result.Error != nil {
		return fmt.Errorf("challenges: consume the challenge: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrInvalid
	}
	return nil
}

// PurgeExpired drops challenges that expired before cutoff. They are spent or
// dead either way, and a table of proven passwords is not one to keep.
func (s *Service) PurgeExpired(ctx context.Context, cutoff time.Time) error {
	err := s.db.WithContext(ctx).
		Where("expires_at < ?", cutoff).
		Delete(&schemas.LoginChallenge{}).Error
	if err != nil {
		return fmt.Errorf("challenges: purge the challenges: %w", err)
	}
	return nil
}

// hash digests a challenge token for storage and lookup.
func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// purgeEvery is how often the sweep runs. It matches modules/oidc's cadence:
// nothing here is urgent, and a table this small does not need attention more
// often than the token tables do.
const purgeEvery = time.Hour

// StartPurge sweeps expired challenges until ctx ends.
//
// Without it the table grows by one row for every second-factor login and
// never shrinks, which is the quiet kind of leak nobody notices until a
// database is bigger than it should be. The rows are worth nothing once they
// expire: each one records only that a password was proven for a flow that is
// over.
func (s *Service) StartPurge(ctx context.Context, retention time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(purgeEvery)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.PurgeExpired(ctx, time.Now().UTC().Add(-retention)); err != nil && ctx.Err() == nil {
					logger.Error("registre: login challenge purge failed", slog.Any("error", err))
				}
			}
		}
	}()
}
