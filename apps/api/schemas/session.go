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

	CreatedAt time.Time
	EndedAt   *time.Time
}

// TableName is the table this model reads.
func (Session) TableName() string { return "sessions" }

// Live reports whether the session has not been ended.
func (s Session) Live() bool { return s.EndedAt == nil }
