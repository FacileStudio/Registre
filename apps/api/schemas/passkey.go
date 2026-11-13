package schemas

import "time"

// Passkey is one WebAuthn credential a person registered. The private key
// never leaves their authenticator; registre keeps the public key and the
// signature counter that detects a cloned authenticator.
type Passkey struct {
	ID              string `gorm:"primaryKey"`
	UserID          string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	Transports      StringList `gorm:"type:jsonb"`
	AAGUID          []byte     `gorm:"column:aaguid"`
	SignCount       int64

	// Flags is the authenticator data flags byte from registration (user
	// present, user verified, backup eligible, backup state). go-webauthn
	// rebuilds its CredentialFlags from it and refuses a login whose backup
	// eligibility changed.
	Flags int16

	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// TableName is the table this model reads.
func (Passkey) TableName() string { return "webauthn_credentials" }

// Ceremony is the server half of one WebAuthn exchange, between the options
// handed to the browser and the answer it sends back. The browser holds only
// a bearer whose SHA-256 is TokenHash; the challenge itself never leaves this
// row except inside the options.
type Ceremony struct {
	ID         string `gorm:"primaryKey"`
	TokenHash  string
	Purpose    string
	UserID     *string
	Session    JSONMap `gorm:"type:jsonb"`
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// TableName is the table this model reads.
func (Ceremony) TableName() string { return "webauthn_ceremonies" }
