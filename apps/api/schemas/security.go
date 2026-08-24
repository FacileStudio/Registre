package schemas

import "time"

// RecoveryCode is one unused way back into an account whose second-factor
// device is gone. The row holds only the SHA-256 of the code; the plaintext
// is shown once at generation and lives nowhere else, exactly like an
// invitation or a client secret.
type RecoveryCode struct {
	ID string `gorm:"primaryKey"`

	UserID   string `gorm:"index"`
	CodeHash string `gorm:"uniqueIndex"`

	UsedAt    *time.Time
	CreatedAt time.Time
}

// TableName is the table this model reads.
func (RecoveryCode) TableName() string { return "totp_recovery_codes" }

// Spent reports whether the code has already been redeemed.
func (c RecoveryCode) Spent() bool { return c.UsedAt != nil }

// LoginChallenge is a password that checked out with a second factor still
// owed. It exists so the "the password was right" state lives in registre's
// database rather than in the browser, which is where handing it to the
// client would put the whole factor.
type LoginChallenge struct {
	ID string `gorm:"primaryKey"`

	// AuthRequestID is the browser flow this challenge unlocks, and nothing
	// else. Binding the two means a challenge minted for one login cannot
	// complete another.
	AuthRequestID string
	UserID        string

	TokenHash string `gorm:"uniqueIndex"`

	// Attempts counts codes tried against this challenge. A six-digit code
	// is 10^6 wide, so an unbounded challenge is a guessable one.
	Attempts int

	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// TableName is the table this model reads.
func (LoginChallenge) TableName() string { return "login_challenges" }

// Live reports whether the challenge may still be answered at the given
// moment: unconsumed, unexpired and inside its attempt budget.
func (c LoginChallenge) Live(at time.Time, maxAttempts int) bool {
	return c.ConsumedAt == nil && c.ExpiresAt.After(at) && c.Attempts < maxAttempts
}

// PasswordReset is one emailed way back in for a human who has forgotten
// their password. It is deliberately not an invitation: an invitation creates
// an account and attests its address, and a reset must do neither.
type PasswordReset struct {
	ID string `gorm:"primaryKey"`

	UserID    string `gorm:"index"`
	TokenHash string `gorm:"uniqueIndex"`

	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// TableName is the table this model reads.
func (PasswordReset) TableName() string { return "password_resets" }

// Redeemable reports whether the reset may still be spent at the given moment.
func (r PasswordReset) Redeemable(at time.Time) bool {
	return r.UsedAt == nil && r.ExpiresAt.After(at)
}
