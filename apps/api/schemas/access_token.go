package schemas

import "time"

// AccessToken records one issued token. The token itself is a signed JWT that
// apps verify offline against the JWKS, so this row is not the credential — it
// is what makes revocation and introspection possible, which a self-contained
// JWT is not on its own.
type AccessToken struct {
	// ID is the jti claim.
	ID        string     `gorm:"primaryKey"`
	ClientID  string     `gorm:"index"`
	Subject   string     `gorm:"index"`
	Scopes    StringList `gorm:"type:jsonb"`
	Audiences StringList `gorm:"type:jsonb"`
	IssuedAt  time.Time
	ExpiresAt time.Time `gorm:"index"`
	RevokedAt *time.Time
}

// TableName is the table this model reads.
func (AccessToken) TableName() string { return "access_tokens" }

// Live reports whether the token is neither revoked nor expired at now.
func (t AccessToken) Live(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
