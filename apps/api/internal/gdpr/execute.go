package gdpr

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// RunDue erases every account whose grace period is over and answers how
// many it erased. One failure is logged and does not stop the others.
func (s *Service) RunDue(ctx context.Context) int {
	var due []schemas.DeletionRequest
	if err := s.DB.WithContext(ctx).Where("execute_after <= ?", s.now()).Find(&due).Error; err != nil {
		s.Logger.Error("registre: read due deletions", slog.Any("error", err))
		return 0
	}
	erased := 0
	for _, request := range due {
		if err := s.Erase(ctx, request.UserID); err != nil {
			s.Logger.Error("registre: account deletion failed",
				slog.String("user_id", request.UserID), slog.Any("error", err))
			continue
		}
		erased++
	}
	return erased
}

// Erase deletes userID now, then tells the apps and the person.
//
// The notices go out after the transaction commits, never inside it: an app
// told about a deletion that then rolled back would erase a live account.
func (s *Service) Erase(ctx context.Context, userID string) error {
	var user schemas.User
	if err := s.DB.WithContext(ctx).Where("id = ?", userID).Take(&user).Error; err != nil {
		return fmt.Errorf("gdpr: read the account: %w", err)
	}
	reached, err := s.Sessions.EndSessions(ctx, userID, s.now())
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return erase(tx, user) }); err != nil {
		return err
	}
	s.announce(ctx, user, reached)
	return nil
}

// erase hands over the spaces the account alone owns, then deletes the
// account and every row keyed on it that has no cascade.
func erase(tx *gorm.DB, user schemas.User) error {
	if err := handOver(tx, user.ID); err != nil {
		return err
	}
	if err := queueNotices(tx, user.ID); err != nil {
		return err
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{"DELETE FROM refresh_tokens WHERE user_id = ?", []any{user.ID}},
		{"DELETE FROM sessions WHERE user_id = ?", []any{user.ID}},
		{"DELETE FROM auth_requests WHERE user_id = ?", []any{user.ID}},
		{"DELETE FROM access_tokens WHERE subject = ?", []any{user.ID}},
		{"DELETE FROM signup_challenges WHERE lower(email) = lower(?)", []any{user.Email}},
		{"DELETE FROM invite_tokens WHERE lower(email) = lower(?)", []any{user.Email}},
		{"DELETE FROM space_invitations WHERE lower(email) = lower(?)", []any{user.Email}},
		{"DELETE FROM users WHERE id = ?", []any{user.ID}},
	}
	for _, statement := range statements {
		if err := tx.Exec(statement.sql, statement.args...).Error; err != nil {
			return fmt.Errorf("gdpr: erase: %w", err)
		}
	}
	return nil
}

// announce runs the side effects of an erasure: logout and deletion notices
// to the apps, a new directory version, the audit row and a last mail.
func (s *Service) announce(ctx context.Context, user schemas.User, reached []string) {
	if s.Notifier != nil {
		s.warn("registre: back-channel logout after deletion", s.Notifier.Fire(ctx, user.ID, reached))
	}
	if s.Directory != nil {
		s.warn("registre: directory bump after deletion", s.Directory.Touch(ctx))
	}
	s.DeliverNotices(ctx)
	s.record(ctx, user.ID, "account.deleted", nil)
	if s.Mailer != nil {
		subject, body := i18n.Mail(i18n.MailDeletionDone, user.Locale, "", nil)
		if err := s.Mailer.Send(ctx, user.Email, subject, body); err != nil {
			s.Logger.Warn("registre: deletion confirmation mail failed", slog.Any("error", err))
		}
	}
}

// warn logs err under message when it is not nil.
func (s *Service) warn(message string, err error) {
	if err != nil {
		s.Logger.Warn(message, slog.Any("error", err))
	}
}

// Start runs RunDue and every housekeeping task every interval until ctx
// ends.
func (s *Service) Start(ctx context.Context, interval time.Duration, chores ...func(context.Context)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx, chores)
			}
		}
	}()
}

// tick runs one housekeeping round.
func (s *Service) tick(ctx context.Context, chores []func(context.Context)) {
	s.RunDue(ctx)
	s.DeliverNotices(ctx)
	s.warn("registre: purge deletion notices", s.PurgeNotices(ctx, s.now()))
	for _, chore := range chores {
		chore(ctx)
	}
}
