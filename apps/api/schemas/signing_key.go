package schemas

import "time"

// Signing key states, in the order a key passes through them.
//
// The order is the whole point of SPEC §7: a key is published in the JWKS
// before anything is signed with it, and its public half is retained after it
// stops signing. A key that went straight to active would 401 every relying
// party whose key set was cached, all at once.
const (
	KeyPublished = "published"
	KeyActive    = "active"
	KeyRetired   = "retired"
)

// SigningKey is one token signing key. Exactly one row is active at a time;
// published and retired rows are served in the JWKS alongside it.
type SigningKey struct {
	// ID is the kid a relying party matches a token against.
	ID        string `gorm:"primaryKey"`
	Algorithm string

	// PublicKey is PKIX DER, stored in the clear so the JWKS can be served
	// without touching the encryption key.
	PublicKey []byte

	// PrivateKey is PKCS#8 DER sealed with AES-256-GCM under the root key,
	// and PrivateKeyNonce is the nonce that sealed it.
	PrivateKey      []byte
	PrivateKeyNonce []byte

	State       string
	CreatedAt   time.Time
	ActivatedAt *time.Time
	RetiredAt   *time.Time

	// ExpiresAt is when a retired key leaves the JWKS. It is set one full
	// token lifetime past retirement, so the last token this key signed has
	// expired before its public half stops being published.
	ExpiresAt *time.Time
}

// TableName is the table this model reads.
func (SigningKey) TableName() string { return "signing_keys" }
