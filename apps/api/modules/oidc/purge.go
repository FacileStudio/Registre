package oidc

import (
	"context"
	"log/slog"
	"time"
)

// purgeEvery is how often expired token records are swept. Nothing depends on
// the sweep being prompt — an expired token is already refused — so it runs
// rarely enough to be invisible.
const purgeEvery = time.Hour

// StartPurge sweeps expired token records until ctx is done. A failed sweep is
// logged and the next one retries: the register is still correct with the rows
// in it, only larger.
//
// A sweep that fails because the process is shutting down is not logged. The
// database handle closes while a tick may be in flight, and an identity
// provider whose shutdown always prints an error teaches everyone to ignore
// the errors it prints.
func (s *Storage) StartPurge(ctx context.Context, retention time.Duration) {
	ticker := time.NewTicker(purgeEvery)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.PurgeTokens(ctx, retention); err != nil && ctx.Err() == nil {
					s.logger.Error("registre: token purge failed", slog.Any("error", err))
				}
			}
		}
	}()
}
