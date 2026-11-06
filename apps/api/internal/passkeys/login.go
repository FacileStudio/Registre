package passkeys

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// BeginLogin starts a discoverable sign-in: no address is asked for, the
// authenticator offers the passkeys it holds for this relying party, and the
// answer names its own account through the user handle. User verification is
// required, because a passkey sign-in owes no second factor: presence alone
// would let whoever holds an unlocked security key in.
func (s *Service) BeginLogin(ctx context.Context) (Options, error) {
	assertion, session, err := s.web.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return Options{}, fmt.Errorf("passkeys: begin login: %w", err)
	}
	token, err := s.open(ctx, PurposeLogin, "", "", session)
	if err != nil {
		return Options{}, err
	}
	return Options{Ceremony: token, PublicKey: assertion.Response}, nil
}

// FinishLogin verifies a discoverable sign-in and answers the account it
// opens. A disabled account answers ErrInvalid like a bad signature.
func (s *Service) FinishLogin(ctx context.Context, token string, response []byte) (string, error) {
	_, session, err := s.claim(ctx, token, PurposeLogin, "")
	if err != nil {
		return "", err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return "", ErrInvalid
	}
	user, credential, err := s.web.ValidatePasskeyLogin(s.discover(ctx), session, parsed)
	if err != nil {
		s.logger.Info("registre: passkey sign-in refused", slog.Any("error", err))
		return "", ErrInvalid
	}
	userID := string(user.WebAuthnID())
	if err := s.touch(ctx, userID, credential); err != nil {
		return "", err
	}
	return userID, nil
}

// BeginReauth starts proving a passkey again inside a session that already
// knows who it is, for the sensitive account changes that ask for it.
func (s *Service) BeginReauth(ctx context.Context, userID, email string) (Options, error) {
	holder, err := s.personFor(ctx, userID, email)
	if err != nil {
		return Options{}, err
	}
	if len(holder.credentials) == 0 {
		return Options{}, ErrInvalid
	}
	assertion, session, err := s.web.BeginLogin(holder,
		webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return Options{}, fmt.Errorf("passkeys: begin reauthentication: %w", err)
	}
	token, err := s.open(ctx, PurposeReauth, userID, "", session)
	if err != nil {
		return Options{}, err
	}
	return Options{Ceremony: token, PublicKey: assertion.Response}, nil
}

// FinishReauth verifies that userID, and nobody else, answered a ceremony
// begun for them.
func (s *Service) FinishReauth(ctx context.Context, token, userID string, response []byte) error {
	row, session, err := s.claim(ctx, token, PurposeReauth, "")
	if err != nil {
		return err
	}
	if row.UserID == nil || *row.UserID != userID {
		return ErrInvalid
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return ErrInvalid
	}
	holder, err := s.personFor(ctx, userID, "")
	if err != nil {
		return err
	}
	credential, err := s.web.ValidateLogin(holder, session, parsed)
	if err != nil {
		s.logger.Info("registre: passkey reauthentication refused", slog.Any("error", err))
		return ErrInvalid
	}
	return s.touch(ctx, userID, credential)
}

// discover resolves the user handle of a discoverable answer to an enabled
// account and its passkeys.
func (s *Service) discover(ctx context.Context) webauthn.DiscoverableUserHandler {
	return func(_, userHandle []byte) (webauthn.User, error) {
		var user schemas.User
		err := s.db.WithContext(ctx).Where("id = ? AND disabled_at IS NULL", string(userHandle)).Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalid
		}
		if err != nil {
			return nil, err
		}
		return s.personFor(ctx, user.ID, user.Email)
	}
}

// touch records a use: the new signature counter and the time. A counter
// that went backwards is go-webauthn's clone warning, and is refused.
func (s *Service) touch(ctx context.Context, userID string, credential *webauthn.Credential) error {
	if credential.Authenticator.CloneWarning {
		s.logger.Warn("registre: passkey counter went backwards, refusing", slog.String("user_id", userID))
		return ErrInvalid
	}
	err := s.db.WithContext(ctx).Model(&schemas.Passkey{}).
		Where("user_id = ? AND credential_id = ?", userID, credential.ID).
		Updates(map[string]any{"sign_count": int64(credential.Authenticator.SignCount), "last_used_at": s.now()}).Error
	if err != nil {
		return fmt.Errorf("passkeys: record the use: %w", err)
	}
	return nil
}
