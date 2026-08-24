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

	// Roles is the flat list this human holds. What reaches a token is
	// the intersection with the client's VisibleRoles, never the raw row:
	// SPEC §6 requires the claim filtered per client, and porte's role
	// checks deny silently on anything else.
	Roles StringList `gorm:"type:jsonb"`

	DisabledAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName is the table this model reads.
func (User) TableName() string { return "users" }

// Enabled reports whether the user may still sign in.
func (u User) Enabled() bool { return u.DisabledAt == nil }
