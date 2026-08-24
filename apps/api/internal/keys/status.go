package keys

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// KeyState is one signing key's lifecycle position, for the dashboard: which
// key exists, whether it is published, active or retired, and when each
// transition happened.
type KeyState struct {
	ID          string     `json:"id"`
	Algorithm   string     `json:"algorithm"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	RetiredAt   *time.Time `json:"retired_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// Status reads every key row's lifecycle position without touching private
// material. Rotation itself stays in the background loop; the dashboard
// watches, it does not drive.
func (s *Service) Status(ctx context.Context) ([]KeyState, error) {
	var rows []schemas.SigningKey
	if err := s.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("keys: read the key set: %w", err)
	}
	states := make([]KeyState, 0, len(rows))
	for _, row := range rows {
		states = append(states, KeyState{
			ID: row.ID, Algorithm: row.Algorithm, State: row.State,
			CreatedAt: row.CreatedAt, ActivatedAt: row.ActivatedAt,
			RetiredAt: row.RetiredAt, ExpiresAt: row.ExpiresAt,
		})
	}
	return states, nil
}
