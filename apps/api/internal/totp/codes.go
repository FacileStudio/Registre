package totp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// RecoveryCodeCount is how many codes an enrolment mints. Ten is enough that
// losing a phone is an inconvenience rather than an incident, and few enough
// that a human will actually store the list somewhere.
const RecoveryCodeCount = 10

// recoveryCodeBytes is the entropy behind one code: 80 bits, which is far
// past guessable and still short enough to read off paper.
const recoveryCodeBytes = 10

// RedeemRecoveryCode spends one of user's recovery codes and reports whether
// it was good. A spent code is marked used in the same statement that finds
// it, so two requests racing the same code cannot both win.
//
// The lookup is by digest and scoped to the user, so a code minted for one
// account can never open another. A global uniqueness constraint alone would
// not stop that, it would only make the collision unlikely.
func (s *Service) RedeemRecoveryCode(ctx context.Context, userID, code string) error {
	digest := hashCode(code)
	result := s.db.WithContext(ctx).Model(&schemas.RecoveryCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, digest).
		Update("used_at", s.now())
	if result.Error != nil {
		return fmt.Errorf("totp: spend the recovery code: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrInvalid
	}
	return nil
}

// RemainingRecoveryCodes counts the codes user has left to spend, which is
// what the account page shows so nobody discovers the number is zero at the
// moment they need one.
func (s *Service) RemainingRecoveryCodes(ctx context.Context, userID string) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&schemas.RecoveryCode{}).
		Where("user_id = ? AND used_at IS NULL", userID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("totp: count the recovery codes: %w", err)
	}
	return int(count), nil
}

// RegenerateRecoveryCodes replaces every code user holds with a fresh set and
// returns the new ones, shown exactly once.
//
// Replacing rather than topping up is deliberate: the reason to regenerate is
// that the old list may have been seen, and a top-up leaves the seen ones
// live. It refuses an account with no confirmed factor, which would otherwise
// accumulate codes that open nothing.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, user schemas.User) ([]string, error) {
	if !user.TOTPEnrolled() {
		return nil, fmt.Errorf("totp: %s has no second factor", user.Email)
	}
	plain, hashes, err := mintRecoveryCodes()
	if err != nil {
		return nil, err
	}
	at := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&schemas.RecoveryCode{}).Error; err != nil {
			return fmt.Errorf("totp: clear the old recovery codes: %w", err)
		}
		return storeRecoveryCodes(tx, user.ID, hashes, at)
	})
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// mintRecoveryCodes returns a fresh set of codes and their digests, in the
// same order. The plaintexts are the caller's to show once and forget.
func mintRecoveryCodes() (plain, hashes []string, err error) {
	plain = make([]string, 0, RecoveryCodeCount)
	hashes = make([]string, 0, RecoveryCodeCount)
	for range RecoveryCodeCount {
		code, err := newRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		plain = append(plain, code)
		hashes = append(hashes, hashCode(code))
	}
	return plain, hashes, nil
}

// newRecoveryCode returns one code, hyphenated in the middle so a human
// reading it aloud or typing it off paper has somewhere to breathe.
func newRecoveryCode() (string, error) {
	raw := make([]byte, recoveryCodeBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("totp: mint a recovery code: %w", err)
	}
	encoded := hex.EncodeToString(raw)
	return encoded[:10] + "-" + encoded[10:], nil
}

// storeRecoveryCodes writes a set of digests for user inside the caller's
// transaction.
func storeRecoveryCodes(tx *gorm.DB, userID string, hashes []string, at time.Time) error {
	rows := make([]schemas.RecoveryCode, 0, len(hashes))
	for _, digest := range hashes {
		id, err := ids.New()
		if err != nil {
			return fmt.Errorf("totp: mint a recovery code id: %w", err)
		}
		rows = append(rows, schemas.RecoveryCode{
			ID: id, UserID: userID, CodeHash: digest, CreatedAt: at,
		})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("totp: store the recovery codes: %w", err)
	}
	return nil
}

// hashCode digests a recovery code for storage and lookup, ignoring the
// hyphen and the case a human may have typed it in.
func hashCode(code string) string {
	normalised := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	sum := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(sum[:])
}
