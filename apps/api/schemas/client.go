package schemas

import "time"

// Client kinds. A service account is a machine principal that authenticates
// with client_credentials and has a subject of its own; an application is a
// suite app authenticating a human, which arrives with phase 1.
const (
	KindServiceAccount = "service_account"
	KindApplication    = "application"
)

// Access policies. An app is open to everyone until an operator restricts it.
const (
	AccessEveryone = "everyone"
	AccessAssigned = "assigned"
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

	// Public marks a client that holds no secret: a CLI on somebody's
	// laptop, where anything shipped in the binary is readable by whoever
	// has the binary. It is a column rather than "SecretHash is empty"
	// precisely because a half-seeded row is also empty, and one of those
	// two states must never be allowed to authenticate.
	Public bool

	GrantTypes StringList `gorm:"type:jsonb"`
	Scopes     StringList `gorm:"type:jsonb"`

	// Audiences is what lands in the token's aud claim. It names the apps
	// this client may call, and an app rejecting a token it is not the
	// audience of is the only thing stopping one agent's credential from
	// working everywhere.
	Audiences StringList `gorm:"type:jsonb"`

	// RedirectURIs is the exact-match list an application client may send
	// the browser back to. Service accounts have none: they never start a
	// browser flow.
	RedirectURIs StringList `gorm:"type:jsonb"`

	// BackchannelLogoutURI is where registre POSTs a logout token when the
	// client's session at the provider ends. Empty means the app does not
	// consume back-channel logout yet and hears nothing — which is why the
	// field exists per client rather than as one global switch.
	BackchannelLogoutURI string

	// DirectorySyncURI is where registre nudges the app after any change to
	// shared spaces, so it pulls the snapshot within seconds instead of at
	// its next interval. Empty means the app only pulls on its own clock.
	DirectorySyncURI string

	// VisibleRoles is the whitelist of roles this app may see in its
	// tokens' roles claim. The default is deliberately none: a new client
	// must ask for role visibility, rather than every app silently
	// learning every role anyone holds.
	VisibleRoles StringList `gorm:"type:jsonb"`

	// TokenExchange is what this client may trade a human's token for at
	// the token endpoint: one entry per audience, each with the scopes it
	// may ask for there. Empty means the client may not exchange.
	TokenExchange ExchangeGrants `gorm:"type:jsonb"`

	AccessTokenTTL int64

	// AccessPolicy decides who may sign in to an application: AccessEveryone
	// admits every enabled user, AccessAssigned only users holding an
	// AppGrant for it. The seed never writes it, so an operator's choice
	// survives every boot.
	AccessPolicy string `gorm:"default:everyone"`

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
