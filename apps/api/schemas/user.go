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

	DisabledAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName is the table this model reads.
func (User) TableName() string { return "users" }

// Enabled reports whether the user may still sign in.
func (u User) Enabled() bool { return u.DisabledAt == nil }
