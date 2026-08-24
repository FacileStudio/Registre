package schemas

import "time"

// InviteToken is one single-use, short-lived secret that lets a named email
// set its password through the normal login path (SPEC §10 q1). The row
// holds only the SHA-256 of the token; the plaintext is returned once at
// issue and lives nowhere else, least of all in this table.
type InviteToken struct {
	ID string `gorm:"primaryKey"`

	Email     string
	TokenHash string `gorm:"uniqueIndex"`
	CreatedBy string

	ExpiresAt  time.Time
	RedeemedAt *time.Time
	CreatedAt  time.Time
}

func (InviteToken) TableName() string { return "invite_tokens" }
