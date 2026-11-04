package passkeys

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// Options is what the browser hands to navigator.credentials, beside the
// ceremony bearer it must send back with the answer.
type Options struct {
	Ceremony  string `json:"ceremony"`
	PublicKey any    `json:"publicKey"`
}

// Registered is a credential whose attestation verified and which is not
// stored yet. Sign-up stores it in the same transaction as the account.
type Registered struct {
	UserID     string
	credential *webauthn.Credential
}

// BeginRegistration starts adding a passkey for userID, labelled email on
// the authenticator. The account's existing passkeys are excluded so the same
// authenticator is not registered twice. For sign-up, userID is the subject
// the account will be created with.
func (s *Service) BeginRegistration(ctx context.Context, purpose, userID, email string) (Options, error) {
	holder, err := s.personFor(ctx, userID, email)
	if err != nil {
		return Options{}, err
	}
	creation, session, err := s.web.BeginRegistration(holder,
		webauthn.WithExclusions(webauthn.Credentials(holder.credentials).CredentialDescriptors()),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExtensions(map[string]any{"credProps": true}),
	)
	if err != nil {
		return Options{}, fmt.Errorf("passkeys: begin registration: %w", err)
	}
	token, err := s.open(ctx, purpose, userID, session)
	if err != nil {
		return Options{}, err
	}
	return Options{Ceremony: token, PublicKey: creation.Response}, nil
}

// FinishRegistration verifies the browser's answer to a registration begun
// for purpose and answers the credential, unstored.
func (s *Service) FinishRegistration(ctx context.Context, token, purpose string, response []byte) (Registered, error) {
	row, session, err := s.claim(ctx, token, purpose)
	if err != nil {
		return Registered{}, err
	}
	userID := string(session.UserID)
	if row.UserID == nil || *row.UserID != userID {
		return Registered{}, ErrInvalid
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return Registered{}, ErrInvalid
	}
	credential, err := s.web.CreateCredential(person{id: userID}, session, parsed)
	if err != nil {
		s.logger.Info("registre: passkey registration refused", "error", err)
		return Registered{}, ErrInvalid
	}
	return Registered{UserID: userID, credential: credential}, nil
}

// Store writes a verified credential for its account, inside tx when one is
// given. A credential id already registered anywhere is refused.
func (s *Service) Store(ctx context.Context, tx *gorm.DB, reg Registered, name string) (schemas.Passkey, error) {
	if reg.credential == nil {
		return schemas.Passkey{}, ErrInvalid
	}
	if tx == nil {
		tx = s.db
	}
	id, err := ids.New()
	if err != nil {
		return schemas.Passkey{}, fmt.Errorf("passkeys: mint an id: %w", err)
	}
	transports := make(schemas.StringList, 0, len(reg.credential.Transport))
	for _, transport := range reg.credential.Transport {
		transports = append(transports, string(transport))
	}
	row := schemas.Passkey{
		ID: id, UserID: reg.UserID, CredentialID: reg.credential.ID, PublicKey: reg.credential.PublicKey,
		AttestationType: reg.credential.AttestationType, Transports: transports,
		AAGUID: reg.credential.Authenticator.AAGUID, SignCount: int64(reg.credential.Authenticator.SignCount),
		Flags: int16(reg.credential.Flags.ProtocolValue()), Name: CleanName(name), CreatedAt: s.now(),
	}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "webauthn_credentials_one_id") {
			return schemas.Passkey{}, ErrInvalid
		}
		return schemas.Passkey{}, fmt.Errorf("passkeys: store the credential: %w", err)
	}
	return row, nil
}

// CleanName trims a label to MaxNameLength characters, defaulting to
// "Passkey".
func CleanName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) > MaxNameLength {
		runes = runes[:MaxNameLength]
	}
	if len(runes) == 0 {
		return "Passkey"
	}
	return string(runes)
}

// personFor loads userID's passkeys into a go-webauthn user named email. An
// unknown userID, such as a sign-up's future subject, has none.
func (s *Service) personFor(ctx context.Context, userID, email string) (person, error) {
	var rows []schemas.Passkey
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return person{}, fmt.Errorf("passkeys: read the credentials: %w", err)
	}
	holder := person{id: userID, name: email}
	for _, row := range rows {
		holder.credentials = append(holder.credentials, toCredential(row))
	}
	return holder, nil
}
