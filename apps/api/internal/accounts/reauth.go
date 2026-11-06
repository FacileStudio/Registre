package accounts

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// ReauthWindow is how long a proven factor counts as fresh. Adding or
// removing a passkey and asking for deletion need a factor proven inside it:
// a session alone, on a borrowed laptop, must not be able to plant a passkey
// of its own or erase the account.
const ReauthWindow = 10 * time.Minute

// Fresh reports whether the session proved a factor within ReauthWindow. It
// reads FactorProvenAt only, never the session's creation time: a session
// opened without every factor (one reissued after a password change) is not
// fresh however new it is.
func (s *Service) Fresh(session schemas.Session) bool {
	return session.FactorProvenAt != nil && session.FactorProvenAt.After(s.now().Add(-ReauthWindow))
}

// MarkFactorProven records that the holder of sessionID just proved every
// factor the account owes: at sign-in, or by reauthenticating.
func (s *Service) MarkFactorProven(ctx context.Context, sessionID string) error {
	err := s.db.WithContext(ctx).Model(&schemas.Session{}).
		Where("id = ? AND ended_at IS NULL", sessionID).
		Update("factor_proven_at", s.now()).Error
	if err != nil {
		return fmt.Errorf("accounts: record the proven factor: %w", err)
	}
	return nil
}
