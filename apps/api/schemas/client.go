package schemas

import "time"

// Client kinds. A service account is a machine principal that authenticates
// with client_credentials and has a subject of its own; an application is a
// suite app authenticating a human, which arrives with phase 1.
const (
	KindServiceAccount = "service_account"
	KindApplication    = "application"
)

// Client is one OAuth client: an entry in the register, with the credential it
// authenticates by and the ceiling on what it may ask for.
//
// Subject is the `sub` its tokens carry and it never changes. Apps match an
// account on (issuer, subject), so re-keying a client here orphans it in every
// app that has seen it.
type Client struct {
	ID       int64  `gorm:"primaryKey"`
	ClientID string `gorm:"uniqueIndex"`
	Subject  string `gorm:"uniqueIndex"`
	Name     string
	Kind     string

	// SecretHash is argon2id, at porte's parameters. A client with no
	// secret cannot authenticate at all, which is what a disabled row and
	// a half-seeded row both look like.
	SecretHash string

	GrantTypes StringList `gorm:"type:jsonb"`
	Scopes     StringList `gorm:"type:jsonb"`

	// Audiences is what lands in the token's aud claim. It names the apps
	// this client may call, and an app rejecting a token it is not the
	// audience of is the only thing stopping one agent's credential from
	// working everywhere.
	Audiences StringList `gorm:"type:jsonb"`

	AccessTokenTTL int64

	// Managed marks a row the YAML seed owns. The seed rewrites it at every
	// boot, so an edit made anywhere else is lost on the next restart.
	Managed bool

	DisabledAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName is the table this model reads.
func (Client) TableName() string { return "clients" }

// Enabled reports whether the client may still be issued tokens.
func (c Client) Enabled() bool { return c.DisabledAt == nil }

// TokenLifetime returns the access token lifetime for this client, falling
// back to fallback when the row does not set one.
func (c Client) TokenLifetime(fallback time.Duration) time.Duration {
	if c.AccessTokenTTL <= 0 {
		return fallback
	}
	return time.Duration(c.AccessTokenTTL) * time.Second
}
