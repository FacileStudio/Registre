package schemas

import "time"

// Consent is one acceptance of one version of one legal document. Rows are
// appended, never updated, so the history of what a person agreed to and
// when survives every version bump.
type Consent struct {
	ID         int64 `gorm:"primaryKey"`
	UserID     string
	Document   string
	Version    string
	Locale     string
	AcceptedAt time.Time

	// IPHash is a keyed digest of the address the acceptance came from:
	// enough to match two acceptances to one network, not enough to name it.
	IPHash string

	// Draft is set when the accepted version was a draft pending legal
	// review. Such an acceptance never satisfies a reviewed version.
	Draft bool
}

// TableName is the table this model reads.
func (Consent) TableName() string { return "consents" }

// SignupChallenge is one emailed sign-up link. Only the SHA-256 of the token
// is stored. A row written for an address that already has an account is
// born spent, so both branches of a sign-up do the same database work.
type SignupChallenge struct {
	ID        string `gorm:"primaryKey"`
	Email     string
	TokenHash string
	Locale    string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// TableName is the table this model reads.
func (SignupChallenge) TableName() string { return "signup_challenges" }

// Redeemable reports whether the link may still be spent at the given moment.
func (c SignupChallenge) Redeemable(at time.Time) bool {
	return c.UsedAt == nil && c.ExpiresAt.After(at)
}

// AltchaSpent is one proof-of-work challenge already answered. The primary
// key is the challenge itself, so a replay fails on the insert.
type AltchaSpent struct {
	Challenge string `gorm:"primaryKey"`
	ExpiresAt time.Time
}

// TableName is the table this model reads.
func (AltchaSpent) TableName() string { return "altcha_spent" }

// DeletionRequest is an account scheduled for erasure. Cancelling deletes the
// row; executing deletes the user, which cascades to it.
type DeletionRequest struct {
	UserID       string `gorm:"primaryKey"`
	RequestedAt  time.Time
	ExecuteAfter time.Time
}

// TableName is the table this model reads.
func (DeletionRequest) TableName() string { return "deletion_requests" }
