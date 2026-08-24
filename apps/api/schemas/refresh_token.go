package schemas

import "time"

// RefreshToken is one renewable grant: the row a client's refresh token
// redeems against. The table's id is the SHA-256 of the token value, never
// the value — the token crosses redirects and access logs on every host
// between registre and the app, and a captured database must not be a list
// of working credentials.
type RefreshToken struct {
	ID string `gorm:"primaryKey"`

	UserID   string `gorm:"index"`
	ClientID string

	Scopes    StringList `gorm:"type:jsonb"`
	Audiences StringList `gorm:"type:jsonb"`
	AMR       StringList `gorm:"type:jsonb"`
	AuthTime  time.Time

	// CreatedAt is when the grant was minted, which is also when it was
	// last rotated: a row is written fresh on every redemption.
	CreatedAt time.Time

	ExpiresAt time.Time `gorm:"index"`
	RevokedAt *time.Time
}

// TableName is the table this model reads.
func (RefreshToken) TableName() string { return "refresh_tokens" }

// Live reports whether the grant is neither revoked nor expired at now.
func (r RefreshToken) Live(now time.Time) bool {
	return r.RevokedAt == nil && now.Before(r.ExpiresAt)
}
