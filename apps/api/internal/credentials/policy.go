package credentials

import (
	"context"
	"errors"
	"unicode/utf8"
)

// MaxPasswordLength is the longest password any path stores. NIST SP
// 800-63B-4 asks for at least sixty-four; the ceiling exists because argon2
// hashes whatever it is handed, and a megabyte password is a megabyte of work
// per guess on a public endpoint.
const MaxPasswordLength = 128

// ErrPasswordTooShort and ErrPasswordTooLong are the only two refusals the
// length policy makes. There are no composition rules: NIST SP 800-63B-4
// forbids them, because they buy predictable substitutions, not entropy.
var (
	ErrPasswordTooShort = errors.New("credentials: password too short")
	ErrPasswordTooLong  = errors.New("credentials: password too long")
)

// ErrPasswordBreached is a new password found in a known breach.
var ErrPasswordBreached = errors.New("credentials: password appears in a known breach")

// Screen answers whether a password appears in a known breach.
// internal/pwned implements it; a nil Screen screens nothing.
type Screen interface {
	Breached(ctx context.Context, password string) bool
}

// CheckNew applies the length policy, then the breach screen, to a password
// about to be stored.
func CheckNew(ctx context.Context, screen Screen, password string) error {
	if err := CheckLength(password); err != nil {
		return err
	}
	if screen != nil && screen.Breached(ctx, password) {
		return ErrPasswordBreached
	}
	return nil
}

// CheckLength applies the length floor and ceiling, counted in characters
// rather than bytes so a password in any script gets the same budget.
func CheckLength(password string) error {
	count := utf8.RuneCountInString(password)
	if count < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if count > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}
