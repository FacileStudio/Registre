package schemas

import "time"

// DeviceCode is one RFC 8628 flow: a CLI that cannot open a browser asked for
// a token, a human was shown a short code to type somewhere else, and the row
// carries whatever answer they gave until the CLI's next poll collects it.
//
// The table's key is the SHA-256 of the device code, never the value. The CLI
// presents that value at the token endpoint the way it would present a refresh
// token, so it is a bearer credential and gets the same treatment every other
// bearer credential in this schema gets.
type DeviceCode struct {
	DeviceCodeHash string `gorm:"primaryKey"`

	// UserCode is the human-typed half, normalised: upper case with the
	// grouping dashes stripped. Normalising on the way in is what makes
	// "bcdf-ghjk" and "BCDFGHJK" the same flow rather than a near miss the
	// person retypes three times.
	UserCode string `gorm:"uniqueIndex"`

	ClientID string
	Scopes   StringList `gorm:"type:jsonb"`

	// UserID, AMR and AuthTime are filled by the approval and are what the
	// issued token says about who is behind it. Null until then: a flow
	// nobody has answered has no subject, and the poll reads exactly that.
	UserID   *string
	AMR      StringList `gorm:"type:jsonb"`
	AuthTime *time.Time

	ApprovedAt *time.Time
	DeniedAt   *time.Time

	// RedeemedAt is stamped when the token endpoint hands the grant over,
	// which is what makes the device code single use.
	RedeemedAt *time.Time

	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

// TableName is the table this model reads.
func (DeviceCode) TableName() string { return "device_codes" }

// Approved reports whether a human said yes and the flow may mint a token.
func (d DeviceCode) Approved() bool { return d.ApprovedAt != nil && d.UserID != nil }

// Denied reports whether a human said no, or has already spent this flow.
//
// A redeemed flow answers denied rather than approved because zitadel/oidc
// offers no third outcome: the state it polls is done, denied or pending, and
// a second redemption of a spent code is none of the three. Denied is the safe
// reading — the caller is refused — and it is what a replayed authorization
// code gets here too.
func (d DeviceCode) Denied() bool { return d.DeniedAt != nil || d.RedeemedAt != nil }

// Subject is the human the approval named, or empty.
func (d DeviceCode) Subject() string {
	if d.UserID == nil {
		return ""
	}
	return *d.UserID
}

// AuthenticatedAt is when that human proved who they were, or the zero time.
func (d DeviceCode) AuthenticatedAt() time.Time {
	if d.AuthTime == nil {
		return time.Time{}
	}
	return *d.AuthTime
}
