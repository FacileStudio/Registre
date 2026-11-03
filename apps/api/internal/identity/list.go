package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// List returns every account, oldest first. The dashboard reads whole rows
// but never ships a PasswordHash over the wire; that discipline lives with
// the handler, which builds its own view of the row.
func (s *Service) List(ctx context.Context) ([]schemas.User, error) {
	var rows []schemas.User
	if err := s.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("identity: list: %w", err)
	}
	return rows, nil
}

// LastSignIns answers when each user last opened a session with registre,
// keyed by user id. A user who never signed in, or whose sessions were all
// purged, is absent.
func (s *Service) LastSignIns(ctx context.Context) (map[string]time.Time, error) {
	var rows []struct {
		UserID string
		Last   time.Time
	}
	err := s.db.WithContext(ctx).Model(&schemas.Session{}).
		Select("user_id, max(created_at) AS last").Group("user_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("identity: read last sign-ins: %w", err)
	}
	out := make(map[string]time.Time, len(rows))
	for _, row := range rows {
		out[row.UserID] = row.Last
	}
	return out, nil
}
