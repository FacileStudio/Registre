// Package throttle slows guessing against one account without ever locking it.
//
// Every attempt is counted when it starts, not when it fails. Counting at the
// start is what makes the check atomic: a hundred parallel guesses race for
// one row, the database serialises them, and only as many get through as the
// budget allows. A success deletes the row, so the failures are the attempts
// that never got that far.
//
// The first Free attempts cost nothing. After that each attempt must wait
// Base, then twice that, doubling up to Cap. There is deliberately no hard
// lockout: an attacker who can lock an account can lock a person out, which is
// the denial of service they wanted. The price of that choice is that while a
// delay is running, the right password is refused as well. The delay is
// bounded by Cap, passkeys and the emailed reset do not pass through here, and
// a refused attempt does not extend the delay, so a victim is never pushed
// beyond Cap.
//
// Unknown identities are throttled exactly like known ones. A throttle that
// only tracked real accounts would be an account-existence oracle.
package throttle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Policy is the shape of the delay curve.
type Policy struct {
	Free   int
	Base   time.Duration
	Cap    time.Duration
	Window time.Duration
}

// DefaultPolicy gives five free attempts, then 30 seconds doubling to 15
// minutes, and forgets an identity a day after its last attempt.
func DefaultPolicy() Policy {
	return Policy{Free: 5, Base: 30 * time.Second, Cap: 15 * time.Minute, Window: 24 * time.Hour}
}

// Service keeps the counters in Postgres, so a restart or a second replica
// does not hand a guesser a fresh budget.
type Service struct {
	db     *gorm.DB
	policy Policy
	now    func() time.Time
}

// New builds the service over db with DefaultPolicy().
func New(db *gorm.DB) *Service {
	return &Service{db: db, policy: DefaultPolicy(), now: func() time.Time { return time.Now().UTC() }}
}

// Key turns a kind and an identity into the stored key. The identity is
// trimmed and lower-cased, the same normalisation the login lookup applies, so
// "Bob@x.org " and "bob@x.org" share one budget.
func Key(kind, identity string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + strings.ToLower(strings.TrimSpace(identity))))
	return hex.EncodeToString(sum[:])
}

type row struct {
	Failures      int
	LastAttemptAt time.Time
	NextAllowedAt time.Time
}

// Attempt claims one attempt for key. It returns zero when the attempt may go
// ahead, or how long the caller must wait when it may not. A refused attempt
// changes nothing.
//
// The row is locked for the length of the transaction, which is what makes
// the count exact under parallel guesses.
func (s *Service) Attempt(ctx context.Context, key string) (time.Duration, error) {
	var wait time.Duration
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) (err error) {
		wait, err = s.claim(tx, key, s.now())
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("throttle: claim an attempt: %w", err)
	}
	return wait, nil
}

func (s *Service) claim(tx *gorm.DB, key string, now time.Time) (time.Duration, error) {
	if err := tx.Exec(`INSERT INTO login_throttles (key, failures, last_attempt_at, next_allowed_at)
		VALUES (?, 0, ?, ?) ON CONFLICT (key) DO NOTHING`, key, now, now).Error; err != nil {
		return 0, err
	}
	var current row
	if err := tx.Raw(`SELECT failures, last_attempt_at, next_allowed_at FROM login_throttles
		WHERE key = ? FOR UPDATE`, key).Scan(&current).Error; err != nil {
		return 0, err
	}
	if current.NextAllowedAt.After(now) {
		return current.NextAllowedAt.Sub(now), nil
	}
	count := 1
	if !current.LastAttemptAt.Before(now.Add(-s.policy.Window)) {
		count = current.Failures + 1
	}
	return 0, tx.Exec(`UPDATE login_throttles SET failures = ?, last_attempt_at = ?, next_allowed_at = ?
		WHERE key = ?`, count, now, now.Add(delay(s.policy, count)), key).Error
}

// Reset forgets key, after a success.
func (s *Service) Reset(ctx context.Context, key string) error {
	if err := s.db.WithContext(ctx).Exec(`DELETE FROM login_throttles WHERE key = ?`, key).Error; err != nil {
		return fmt.Errorf("throttle: reset: %w", err)
	}
	return nil
}

// delay is the wait that follows the nth attempt in a row.
func delay(p Policy, n int) time.Duration {
	if n < p.Free {
		return 0
	}
	wait := p.Base
	for i := p.Free; i < n && wait < p.Cap; i++ {
		wait *= 2
	}
	return min(wait, p.Cap)
}
