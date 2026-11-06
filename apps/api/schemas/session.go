package schemas

import "time"

// Session records one human login at the provider. It is the thing
// end_session terminates and back-channel logout announces: an app holding
// its own long-lived cookie learns a session ended only if registre knows
// which sessions existed and which apps they reached.
type Session struct {
	ID string `gorm:"primaryKey"`

	UserID string `gorm:"index"`

	// ClientIDs is every app the session authenticated to, appended at the
	// token exchange. Back-channel logout notifies exactly these.
	ClientIDs StringList `gorm:"type:jsonb"`

	// TokenHash is the SHA-256 of the cookie that lets this human reach
	// registre's own account pages. It hangs off the session row rather
	// than a table of its own so that ending a session closes the account
	// pages in the same statement that closes everything else — a second
	// table would be a second thing to remember to revoke, which is how a
	// revocation gets missed.
	//
	// The tag says uniqueIndex and the migration builds a partial one,
	// WHERE token_hash IS NOT NULL. They are not the same object. Nothing
	// creates the tag's version, because migrations own the schema and
	// AutoMigrate is refused by scripts/check.sh, so this is a reader's
	// hazard rather than a data one. Postgres already tolerates unlimited
	// NULLs in a plain unique index, so the predicate buys index size on a
	// table where most rows never carry a cookie, not correctness.
	TokenHash *string `gorm:"uniqueIndex"`

	// FactorProvenAt is the last time the holder proved a factor in this
	// session: at sign-in (every factor the account owes) or by
	// reauthenticating. A session reissued after a password change does not
	// set it, so sensitive changes cannot inherit freshness from a new row.
	FactorProvenAt *time.Time

	CreatedAt time.Time
	EndedAt   *time.Time
}

// TableName is the table this model reads.
func (Session) TableName() string { return "sessions" }

// Live reports whether the session has not been ended.
func (s Session) Live() bool { return s.EndedAt == nil }
