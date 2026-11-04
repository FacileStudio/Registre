package identity

import (
	"context"
	"errors"
)

// ErrConsentRequired is a person who proved who they are but has not
// accepted the current version of a required legal document. The login
// finishes only after they accept it.
var ErrConsentRequired = errors.New("identity: consent to the current terms is required")

// ConsentChecker answers the required documents a person still owes an
// acceptance of. internal/consent implements it.
type ConsentChecker interface {
	Outdated(ctx context.Context, userID string) ([]string, error)
}

// WithConsent makes Complete refuse, with ErrConsentRequired, a person whose
// acceptance of the current legal documents is missing. Without it every
// login completes as it did before legal documents existed.
func (s *Service) WithConsent(checker ConsentChecker) *Service {
	s.consents = checker
	return s
}

// ConsentOwed answers ErrConsentRequired while userID owes an acceptance of
// a required legal document, and nil otherwise or when consent is not wired.
func (s *Service) ConsentOwed(ctx context.Context, userID string) error {
	return s.requireConsent(ctx, userID)
}

// requireConsent answers ErrConsentRequired while userID owes an acceptance.
func (s *Service) requireConsent(ctx context.Context, userID string) error {
	if s.consents == nil {
		return nil
	}
	owed, err := s.consents.Outdated(ctx, userID)
	if err != nil {
		return err
	}
	if len(owed) > 0 {
		return ErrConsentRequired
	}
	return nil
}
