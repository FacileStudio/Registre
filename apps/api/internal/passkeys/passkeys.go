// Package passkeys is Registre's WebAuthn relying party, over go-webauthn.
//
// The relying party ID and the allowed origins are configuration (env
// WEBAUTHN_RP_ID and WEBAUTHN_ORIGINS, defaulting to the issuer's host and
// origin) and the origin check is exact: go-webauthn compares the client
// data's origin against the list and nothing else passes.
//
// Every ceremony is two requests. Begin stores the go-webauthn session data
// in webauthn_ceremonies under the SHA-256 of a bearer it hands the browser
// beside the options; Finish claims that row once, so a replayed answer or a
// ceremony begun for one purpose and finished for another is refused. The
// user handle is the person's subject, which is random and carries no
// personal data.
//
// It owns no routes. modules/login, modules/account and modules/signup own
// the HTTP.
package passkeys

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// ErrInvalid is every refusal a ceremony makes: an unknown or spent
// ceremony, a wrong purpose, a bad signature, an unknown credential, a
// disabled account. One error for all of them.
var ErrInvalid = errors.New("passkeys: invalid passkey response")

// Ceremony purposes, as stored in webauthn_ceremonies.purpose.
const (
	PurposeRegister = "register"
	PurposeSignup   = "signup"
	PurposeLogin    = "login"
	PurposeReauth   = "reauth"
)

// CeremonyTTL is how long a begun ceremony may be finished.
const CeremonyTTL = 5 * time.Minute

// MaxNameLength caps a passkey's label.
const MaxNameLength = 64

// Config is the relying party.
type Config struct {
	RPID    string
	Origins []string
}

// Service registers, verifies and manages passkeys.
type Service struct {
	db     *gorm.DB
	web    *webauthn.WebAuthn
	logger *slog.Logger
	now    func() time.Time
}

// New builds the relying party. User verification is required for every
// ceremony, registration included: a passkey stands in for a password and a
// second factor at once, so a credential made without verifying the person
// must never exist.
func New(db *gorm.DB, cfg Config, logger *slog.Logger) (*Service, error) {
	web, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: "Facile",
		RPOrigins:     cfg.Origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
	if err != nil {
		return nil, fmt.Errorf("passkeys: configure the relying party: %w", err)
	}
	return &Service{db: db, web: web, logger: logger, now: func() time.Time { return time.Now().UTC() }}, nil
}

// person adapts one account to go-webauthn's User.
type person struct {
	id          string
	name        string
	credentials []webauthn.Credential
}

// WebAuthnID is the user handle: the subject.
func (p person) WebAuthnID() []byte { return []byte(p.id) }

// WebAuthnName is the account's address, which authenticators show.
func (p person) WebAuthnName() string { return p.name }

// WebAuthnDisplayName is the same address.
func (p person) WebAuthnDisplayName() string { return p.name }

// WebAuthnCredentials is every passkey the account holds.
func (p person) WebAuthnCredentials() []webauthn.Credential { return p.credentials }

// toCredential rebuilds go-webauthn's view of a stored row.
func toCredential(row schemas.Passkey) webauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, 0, len(row.Transports))
	for _, transport := range row.Transports {
		transports = append(transports, protocol.AuthenticatorTransport(transport))
	}
	return webauthn.Credential{
		ID: row.CredentialID, PublicKey: row.PublicKey, AttestationType: row.AttestationType,
		Transport: transports,
		Flags:     webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(row.Flags)),
		Authenticator: webauthn.Authenticator{
			AAGUID: row.AAGUID, SignCount: uint32(row.SignCount),
		},
	}
}
