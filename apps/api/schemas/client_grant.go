package schemas

import "time"

// ClientGrant is what a person allowed a third-party client: the resource it
// may reach on their behalf and the scopes it may hold there.
type ClientGrant struct {
	ID         string `gorm:"primaryKey"`
	UserID     string
	ClientID   string
	Resource   string
	Scopes     StringList `gorm:"type:jsonb"`
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// TableName is the table the grants live in.
func (ClientGrant) TableName() string { return "client_grants" }

// Covers reports whether the grant already allows every scope in scopes.
func (g ClientGrant) Covers(scopes []string) bool {
	for _, scope := range scopes {
		if !g.Scopes.Contains(scope) {
			return false
		}
	}
	return true
}
