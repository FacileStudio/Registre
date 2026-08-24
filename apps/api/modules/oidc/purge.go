package oidc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// PurgeGrants drops refresh grants and sessions that expired or ended more
// than retention ago. The rows outlive their usefulness on purpose — an audit
// question asked tomorrow morning is why — but not forever.
//
// Sessions live on the grant clock, not the token clock: the passed-in
// retention is derived from the access-token lifetime, but a refresh grant may
// be configured far outlive it (REGISTRE_REFRESH_TOKEN_TTL), and a back-channel
// logout needs the session row to name the apps to notify. So the effective
// retention is whichever is longer between the passed value and the configured
// refresh TTL plus 24h of breathing room.
func (s *Storage) PurgeGrants(ctx context.Context, retention time.Duration) error {
	cutoff := s.now().Add(-s.grantRetention(retention))
	err := s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.RefreshToken{}).Error
	if err != nil {
		return fmt.Errorf("oidc: purge refresh tokens: %w", err)
	}
	idleCutoff := cutoff.Add(-24 * time.Hour)
	sessions := s.db.WithContext(ctx).
		Where(
			"(ended_at IS NOT NULL AND ended_at < ?) OR (ended_at IS NULL AND created_at < ?)",
			cutoff, idleCutoff,
		).
		Delete(&schemas.Session{})
	if sessions.Error != nil {
		return fmt.Errorf("oidc: purge sessions: %w", sessions.Error)
	}
	return nil
}

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
				if err := s.PurgeGrants(ctx, retention); err != nil && ctx.Err() == nil {
					s.logger.Error("registre: grant purge failed", slog.Any("error", err))
				}
			}
		}
	}()
}

// grantRetention is the retention actually applied to grants and sessions:
// never shorter than the passed-in token-record retention, and never shorter
// than a still-redeemable refresh grant plus a day, so no live grant loses the
// session row its eventual logout depends on.
func (s *Storage) grantRetention(retention time.Duration) time.Duration {
	floor := s.refreshTTL() + 24*time.Hour
	if retention > floor {
		return retention
	}
	return floor
}

// PurgeTokens drops token records that expired more than retention ago. The
// row outlives the token deliberately: revoking something already expired is a
// no-op, but an audit question asked the next morning is not.
func (s *Storage) PurgeTokens(ctx context.Context, retention time.Duration) error {
	cutoff := s.now().Add(-retention)
	err := s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.AccessToken{}).Error
	if err != nil {
		return fmt.Errorf("oidc: purge tokens: %w", err)
	}
	return nil
}
