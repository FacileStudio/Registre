package throttle

import (
	"context"
	"log/slog"
	"time"
)

// PurgeExpired deletes the rows whose last attempt is older than the window.
func (s *Service) PurgeExpired(ctx context.Context) error {
	cutoff := s.now().Add(-s.policy.Window)
	return s.db.WithContext(ctx).Exec(`DELETE FROM login_throttles WHERE last_attempt_at < ?`, cutoff).Error
}

// StartPurge sweeps old rows hourly until ctx ends, and returns the service.
// Without it every guessed address, real or invented, would stay for ever.
func (s *Service) StartPurge(ctx context.Context, logger *slog.Logger) *Service {
	go s.purgeLoop(ctx, logger)
	return s
}

func (s *Service) purgeLoop(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.PurgeExpired(ctx); err != nil && ctx.Err() == nil {
				logger.Error("registre: login throttle purge failed", slog.Any("error", err))
			}
		}
	}
}
