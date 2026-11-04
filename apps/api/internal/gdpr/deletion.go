// Package gdpr is a person's rights over their Registre data: a JSON export
// of everything Registre holds about them, and erasure after a thirty-day
// grace period they can cancel.
//
// Erasure ends every session, deletes the account and every row keyed on it,
// then tells each app: a back-channel logout to the apps the sessions reached
// and a signed user.deleted notice to every app's sync URI, which asks the
// app to erase what it holds within thirty days. The audit row it leaves
// names only the subject, which is a random id.
//
// It owns no routes; modules/account does.
package gdpr

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Grace is how long a deletion request waits before it runs.
const Grace = 30 * 24 * time.Hour

// Notifier tells the apps a person is gone. internal/logout implements it.
type Notifier interface {
	Fire(ctx context.Context, userID string, clientIDs []string) error
	NotifyDeleted(ctx context.Context, userID string) error
}

// Directory bumps the shared-spaces version after memberships vanish.
type Directory interface {
	Touch(ctx context.Context) error
}

// Sessions ends a person's sessions and answers the apps they reached.
// internal/identity implements it.
type Sessions interface {
	EndSessions(ctx context.Context, userID string, at time.Time) ([]string, error)
}

// Deps is what the service is built over. Mailer, Notifier and Directory
// may be nil; the matching side effect is then skipped.
type Deps struct {
	DB        *gorm.DB
	Sessions  Sessions
	Audit     *audit.Service
	Mailer    recovery.Mailer
	Notifier  Notifier
	Directory Directory
	Logger    *slog.Logger
}

// Service runs exports and deletions.
type Service struct {
	Deps
	issuer string
	now    func() time.Time
}

// New builds the service; issuer is where mailed links point.
func New(deps Deps, issuer string) *Service {
	return &Service{Deps: deps, issuer: strings.TrimRight(issuer, "/"), now: func() time.Time { return time.Now().UTC() }}
}

// Request schedules user's account for deletion after Grace and mails them
// the date. Asking twice keeps the first date.
func (s *Service) Request(ctx context.Context, user schemas.User) (time.Time, error) {
	row := schemas.DeletionRequest{UserID: user.ID, RequestedAt: s.now(), ExecuteAfter: s.now().Add(Grace)}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return time.Time{}, fmt.Errorf("gdpr: record the request: %w", err)
	}
	if err := s.DB.WithContext(ctx).Where("user_id = ?", user.ID).Take(&row).Error; err != nil {
		return time.Time{}, fmt.Errorf("gdpr: read the request: %w", err)
	}
	s.record(ctx, user.ID, "account.deletion_requested", map[string]any{"execute_after": row.ExecuteAfter})
	if s.Mailer != nil {
		subject, body := i18n.Mail(i18n.MailDeletionScheduled, user.Locale, s.issuer+"/account",
			map[string]string{"date": row.ExecuteAfter.Format("2006-01-02")})
		if err := s.Mailer.Send(ctx, user.Email, subject, body); err != nil {
			s.Logger.Warn("registre: deletion mail failed", slog.Any("error", err))
		}
	}
	return row.ExecuteAfter, nil
}

// Cancel withdraws userID's pending deletion. Cancelling nothing is fine.
func (s *Service) Cancel(ctx context.Context, userID string) error {
	result := s.DB.WithContext(ctx).Where("user_id = ?", userID).Delete(&schemas.DeletionRequest{})
	if result.Error != nil {
		return fmt.Errorf("gdpr: cancel: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		s.record(ctx, userID, "account.deletion_cancelled", nil)
	}
	return nil
}

// Scheduled answers when userID's account will be deleted, or nil.
func (s *Service) Scheduled(ctx context.Context, userID string) (*time.Time, error) {
	var rows []schemas.DeletionRequest
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("gdpr: read the request: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0].ExecuteAfter, nil
}

// record writes an audit row whose actor is the person themself. A failed
// audit write is logged rather than failing the request it describes.
func (s *Service) record(ctx context.Context, userID, action string, detail map[string]any) {
	if s.Audit == nil {
		return
	}
	target := audit.Target{Type: "user", ID: userID}
	if err := s.Audit.Record(ctx, userID, action, target, detail); err != nil {
		s.Logger.Warn("registre: audit write failed", slog.String("action", action), slog.Any("error", err))
	}
}
