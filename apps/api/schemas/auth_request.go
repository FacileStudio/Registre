package schemas

import "time"

// AuthRequest is one pending browser flow: what /authorize was asked, and
// who answered it. The row exists from the moment the browser arrives until
// the code is exchanged or the request expires.
type AuthRequest struct {
	ID string `gorm:"primaryKey"`

	ClientID    string
	RedirectURI string

	Scopes StringList `gorm:"type:jsonb"`
	State  string
	Nonce  string
	// ResponseType is always "code" today; the column exists so a second
	// response type is a migration, not a silent change of meaning.
	ResponseType string
	// CodeChallenge and its method carry PKCE. Empty means the client did
	// not send one, which an application client is refused for elsewhere.
	CodeChallenge       string
	CodeChallengeMethod string
	UserID              *string
	AuthenticatedAt     *time.Time
	ExpiresAt           time.Time

	// Code is the authorization code while it is unspent. It is stored
	// hashed-free deliberately: the code is single-use, short-lived and
	// already an opaque random, so a second hash layer buys nothing.
	Code string
}

// TableName is the table this model reads.
func (AuthRequest) TableName() string { return "auth_requests" }

// Done reports whether the human has authenticated and the flow may proceed
// to the callback. It mirrors op.AuthRequest's contract of the same name.
func (a AuthRequest) Done() bool { return a.UserID != nil && a.AuthenticatedAt != nil }
