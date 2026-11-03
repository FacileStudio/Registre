package schemas

import "time"

// User is one human principal. A user's `sub` is its id and never changes:
// apps key an account on (issuer, subject), so re-keying here orphans the
// human in every app that has seen them.
type User struct {
	ID           string `gorm:"primaryKey"`
	Email        string
	Name         string
	PasswordHash string
	Admin        bool

	// EmailVerified records whether registre has reason to believe the
	// address is the user's. Every account today is minted by an operator,
	// who attests the address by creating it; self-service signup, when it
	// arrives, sets this false until an address challenge answers.
	EmailVerified bool

	// TOTPSecret is the RFC 6238 seed, sealed under the process root key
	// beside the nonce that sealed it. It is sealed rather than hashed
	// because verifying a code needs the seed back, which is exactly what
	// a signing key needs and why both are stored the same way.
	TOTPSecret []byte
	TOTPNonce  []byte

	// TOTPConfirmedAt separates an enrolment that was started from one that
	// was proven with a live code. Only a confirmed factor gates a login:
	// an abandoned enrolment must never be able to lock its owner out.
	TOTPConfirmedAt *time.Time

	// TOTPLastStep is the last time step this account spent, so the same
	// code cannot be presented twice. RFC 6238 §5.2 requires the check:
	// without it a captured code stays good for the rest of its window.
	TOTPLastStep *int64

	DisabledAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName is the table this model reads.
func (User) TableName() string { return "users" }

// Enabled reports whether the user may still sign in.
func (u User) Enabled() bool { return u.DisabledAt == nil }

// TOTPEnrolled reports whether a proven second factor gates this account's
// logins. An unconfirmed seed is not enrolment.
func (u User) TOTPEnrolled() bool {
	return u.TOTPConfirmedAt != nil && len(u.TOTPSecret) > 0
}
