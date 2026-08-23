package keys

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// lockID is the advisory lock the rotation takes. Two replicas ticking at the
// same second would otherwise each mint a key and each try to promote it; the
// partial unique index would refuse the second, but the first would already
// have retired a key nobody meant to retire.
const lockID int64 = 0x5265_6769_7374_7200

// tickBounds keep the rotation loop's interval sane whatever the publish delay
// is configured to. The loop's tightest deadline is the delay, so it ticks at a
// quarter of it, never faster than a minute and never slower than an hour.
const (
	minTick = time.Minute
	maxTick = time.Hour
)

// Start runs the rotation until ctx is done. It is fire and forget: a failed
// tick is logged and the next one retries, because the alternative is a
// process that stops rotating without saying so.
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.tick())
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Rotate(ctx); err != nil && ctx.Err() == nil {
					s.logger.Error("registre: key rotation failed", slog.Any("error", err))
				}
			}
		}
	}()
}

// tick is how often the rotation loop wakes up.
func (s *Service) tick() time.Duration {
	interval := s.cfg.PublishDelay / 4
	if interval < minTick {
		return minTick
	}
	if interval > maxTick {
		return maxTick
	}
	return interval
}

// Rotate runs one step of the lifecycle: drop what is past retention, publish
// the next key when the current one is near the end of its term, and promote a
// published key once it has had the delay to propagate.
//
// Every step is idempotent, so running it on a timer and running it by hand
// have the same effect and a missed tick costs nothing but time.
func (s *Service) Rotate(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		held, err := s.lock(tx)
		if err != nil || !held {
			return err
		}
		if err := s.prune(tx); err != nil {
			return err
		}
		if err := s.publish(tx); err != nil {
			return err
		}
		return s.activate(tx)
	})
}

// lock takes the transaction-scoped advisory lock, reporting whether it got it.
// Not getting it means another replica is already rotating, which is a
// complete answer rather than an error.
func (s *Service) lock(tx *gorm.DB) (bool, error) {
	var held bool
	if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", lockID).Scan(&held).Error; err != nil {
		return false, fmt.Errorf("keys: take the rotation lock: %w", err)
	}
	return held, nil
}

// prune drops retired keys whose retention has run out. Their public halves
// have outlived every token they signed, so nothing can still need them.
func (s *Service) prune(tx *gorm.DB) error {
	err := tx.Where("state = ? AND expires_at IS NOT NULL AND expires_at < ?", schemas.KeyRetired, s.now()).
		Delete(&schemas.SigningKey{}).Error
	if err != nil {
		return fmt.Errorf("keys: prune: %w", err)
	}
	return nil
}

// publish mints the next key when the active one is within a publish delay of
// the end of its term. The new key goes straight into the JWKS and signs
// nothing, which is the whole point.
func (s *Service) publish(tx *gorm.DB) error {
	var pending int64
	if err := tx.Model(&schemas.SigningKey{}).Where("state = ?", schemas.KeyPublished).Count(&pending).Error; err != nil {
		return fmt.Errorf("keys: count published: %w", err)
	}
	if pending > 0 {
		return nil
	}

	var active schemas.SigningKey
	err := tx.Where("state = ?", schemas.KeyActive).Take(&active).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keys: read the active key: %w", err)
	}
	if s.now().Before(since(active).Add(s.cfg.RotateEvery - s.cfg.PublishDelay)) {
		return nil
	}

	row, err := s.mint(schemas.KeyPublished)
	if err != nil {
		return err
	}
	if err := tx.Create(row).Error; err != nil {
		return fmt.Errorf("keys: publish: %w", err)
	}
	s.logger.Info("registre: published the next signing key",
		slog.String("kid", row.ID), slog.Duration("signs_in", s.cfg.PublishDelay))
	return nil
}

// activate promotes a published key once the delay has passed, retiring the
// key it replaces rather than deleting it.
func (s *Service) activate(tx *gorm.DB) error {
	var pending schemas.SigningKey
	err := tx.Where("state = ?", schemas.KeyPublished).Order("created_at").Take(&pending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keys: read the published key: %w", err)
	}
	now := s.now()
	if now.Before(pending.CreatedAt.Add(s.cfg.PublishDelay)) {
		return nil
	}

	retire := map[string]any{"state": schemas.KeyRetired, "retired_at": now, "expires_at": now.Add(s.cfg.Retention)}
	if err := tx.Model(&schemas.SigningKey{}).Where("state = ?", schemas.KeyActive).Updates(retire).Error; err != nil {
		return fmt.Errorf("keys: retire the active key: %w", err)
	}
	promote := map[string]any{"state": schemas.KeyActive, "activated_at": now}
	if err := tx.Model(&schemas.SigningKey{}).Where("id = ?", pending.ID).Updates(promote).Error; err != nil {
		return fmt.Errorf("keys: activate %s: %w", pending.ID, err)
	}
	s.logger.Info("registre: signing key rotated",
		slog.String("kid", pending.ID), slog.Duration("retained_for", s.cfg.Retention))
	return nil
}

// since reports when a key started signing, falling back to when it was minted
// for a row written before activation was recorded.
func since(row schemas.SigningKey) time.Time {
	if row.ActivatedAt != nil {
		return *row.ActivatedAt
	}
	return row.CreatedAt
}
