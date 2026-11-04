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

// Fresh reports whether the session proved a factor within ReauthWindow,
// either when it was opened or by reauthenticating since.
func (s *Service) Fresh(session schemas.Session) bool {
	cutoff := s.now().Add(-ReauthWindow)
	if session.CreatedAt.After(cutoff) {
		return true
	}
	return session.ReauthenticatedAt != nil && session.ReauthenticatedAt.After(cutoff)
}

// MarkReauthenticated records that the holder of sessionID just proved a
// factor again.
func (s *Service) MarkReauthenticated(ctx context.Context, sessionID string) error {
	err := s.db.WithContext(ctx).Model(&schemas.Session{}).
		Where("id = ? AND ended_at IS NULL", sessionID).
		Update("reauthenticated_at", s.now()).Error
	if err != nil {
		return fmt.Errorf("accounts: record the reauthentication: %w", err)
	}
	return nil
}
