package schemas

import "time"

// SocialIdentity links one external account, such as a Discord user, to one
// Registre user. (Provider, Subject) is the key; a user holds at most one
// identity per provider.
type SocialIdentity struct {
	Provider    string `gorm:"primaryKey"`
	Subject     string `gorm:"primaryKey"`
	UserID      string
	DisplayName string
	CreatedAt   time.Time
}

// TableName is the table this model reads.
func (SocialIdentity) TableName() string { return "social_identities" }

// SocialState is a sign-in or link that has left for the provider. Only the
// digest of the state is stored; the browser holds the state itself in a
// cookie. UserID is set when a signed-in user is linking an account.
type SocialState struct {
	StateHash     string `gorm:"primaryKey"`
	Provider      string
	AuthRequestID string
	UserID        string
	Verifier      string
	ReturnTo      string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// TableName is the table this model reads.
func (SocialState) TableName() string { return "social_states" }
