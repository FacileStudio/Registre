package gdpr

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// Notice delivery. A push that fails is retried after five minutes, then
// twice as long each time up to a day, at most MaxAttempts times; the row
// stays listable at GET /api/directory/deletions for NoticeRetention either
// way, which is how an app with no deletion URI, or one that was down for a
// week, still learns.
const (
	MaxAttempts     = 12
	firstRetry      = 5 * time.Minute
	maxRetry        = 24 * time.Hour
	NoticeRetention = 60 * 24 * time.Hour
)

// Deleted is one erased account as an app lists it.
type Deleted struct {
	Subject   string    `json:"sub"`
	DeletedAt time.Time `json:"deleted_at"`
}

// pendingNotice is one undelivered push and where it goes.
type pendingNotice struct {
	ID       int64
	ClientID string
	Subject  string
	Attempts int
	URI      string
}

// queueNotices writes a notice for every enabled application, inside the
// erasure's transaction, so a crash between the delete and the push cannot
// lose the event.
func queueNotices(tx *gorm.DB, subject string) error {
	err := tx.Exec(`INSERT INTO deletion_notices (client_id, subject)
		SELECT client_id, ? FROM clients WHERE kind = 'application' AND disabled_at IS NULL`, subject).Error
	if err != nil {
		return fmt.Errorf("gdpr: queue the deletion notices: %w", err)
	}
	return nil
}

// DeliverNotices pushes every due notice to its app's deletion URI. Never
// the sync URI: a sync handler answers 202 to any POST, so a notice sent
// there would be counted delivered and erase nothing.
func (s *Service) DeliverNotices(ctx context.Context) {
	if s.Notifier == nil {
		return
	}
	var due []pendingNotice
	err := s.DB.WithContext(ctx).Raw(`SELECT n.id, n.client_id, n.subject, n.attempts, c.deletion_uri AS uri
		FROM deletion_notices n JOIN clients c ON c.client_id = n.client_id
		WHERE n.delivered_at IS NULL AND n.next_attempt_at <= ? AND n.attempts < ?
		  AND c.deletion_uri <> '' AND c.disabled_at IS NULL
		ORDER BY n.id LIMIT 200`, s.now(), MaxAttempts).Scan(&due).Error
	if err != nil {
		s.Logger.Error("registre: read due deletion notices", slog.Any("error", err))
		return
	}
	for _, notice := range due {
		s.settle(ctx, notice, s.Notifier.DeliverDeleted(ctx, notice.Subject, notice.ClientID, notice.URI))
	}
}

// settle records one push: delivered, or the next attempt after a backoff.
func (s *Service) settle(ctx context.Context, notice pendingNotice, delivered error) {
	updates := map[string]any{"attempts": notice.Attempts + 1}
	if delivered == nil {
		updates["delivered_at"] = s.now()
	} else {
		updates["next_attempt_at"] = s.now().Add(backoff(notice.Attempts))
		s.Logger.Warn("registre: user.deleted push failed, will retry",
			slog.String("client_id", notice.ClientID), slog.Int("attempt", notice.Attempts+1), slog.Any("error", delivered))
	}
	err := s.DB.WithContext(ctx).Table("deletion_notices").Where("id = ?", notice.ID).Updates(updates).Error
	if err != nil {
		s.Logger.Error("registre: record a deletion notice", slog.Any("error", err))
	}
}

// backoff is the wait after the given number of failed attempts.
func backoff(attempts int) time.Duration {
	wait := firstRetry << min(attempts, 10)
	return min(wait, maxRetry)
}

// DeletedFor lists the accounts erased since since, for one app.
func DeletedFor(ctx context.Context, db *gorm.DB, clientID string, since time.Time) ([]Deleted, error) {
	out := []Deleted{}
	err := db.WithContext(ctx).Raw(`SELECT subject, created_at AS deleted_at FROM deletion_notices
		WHERE client_id = ? AND created_at > ? ORDER BY created_at, id LIMIT 1000`, clientID, since).Scan(&out).Error
	return out, err
}

// PurgeNotices deletes notices older than NoticeRetention.
func (s *Service) PurgeNotices(ctx context.Context, now time.Time) error {
	return s.DB.WithContext(ctx).Exec("DELETE FROM deletion_notices WHERE created_at < ?", now.Add(-NoticeRetention)).Error
}
