package accounts

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Ended is the session End closed: who held it and every app it reached. A
// zero Ended means the cookie named nothing live, and there is nobody to tell.
type Ended struct {
	UserID    string
	ClientIDs []string
}

// End terminates the session behind a cookie value and revokes the refresh
// grants its apps hold. It is idempotent: a cookie that names nothing live is
// already in the state the caller wanted, and answering an unknown cookie with
// an error is how logging out fails for exactly the person whose session has
// gone wrong.
func (s *Service) End(ctx context.Context, token string) (Ended, error) {
	return s.end(ctx, token, "")
}

// end ends the live session behind token, unless keep names its holder, in
// one transaction with revokeGrants.
func (s *Service) end(ctx context.Context, token, keep string) (Ended, error) {
	if token == "" {
		return Ended{}, nil
	}
	var ended Ended
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		ended, err = s.endRow(tx, token, keep)
		if err != nil {
			return err
		}
		return s.revokeGrants(tx, ended)
	})
	if err != nil {
		return Ended{}, err
	}
	return ended, nil
}

// endRow stamps ended_at on the live session behind token and returns who
// held it and the apps it reached, or a zero Ended when nothing was live.
func (s *Service) endRow(tx *gorm.DB, token, keep string) (Ended, error) {
	var rows []schemas.Session
	query := tx.Model(&rows).Clauses(clause.Returning{}).
		Where("token_hash = ? AND ended_at IS NULL", hash(token))
	if keep != "" {
		query = query.Where("user_id <> ?", keep)
	}
	if err := query.Update("ended_at", s.now()).Error; err != nil {
		return Ended{}, fmt.Errorf("accounts: end the session: %w", err)
	}
	if len(rows) == 0 {
		return Ended{}, nil
	}
	return Ended{UserID: rows[0].UserID, ClientIDs: []string(rows[0].ClientIDs)}, nil
}

// revokeGrants revokes the live refresh grants the ended session's holder
// has with the apps it reached.
//
// A refresh grant carries no session id, only the user and the app, so the
// grants revoked are every live one the user holds with those apps. That is
// the same reach as the logout token Fire sends: it names only the subject,
// so each app already ends every session that user holds there. Apps the
// session never reached keep their grants, which is what spares a CLI that
// signed in by device flow from a browser logout. Every grant in the suite
// carries offline_access, because porte always asks for it, so the exemption
// Back-Channel Logout 1.0 §2.7 allows for those grants would revoke nothing.
func (s *Service) revokeGrants(tx *gorm.DB, ended Ended) error {
	if len(ended.ClientIDs) == 0 {
		return nil
	}
	err := tx.Model(&schemas.RefreshToken{}).
		Where("user_id = ? AND client_id IN ? AND revoked_at IS NULL", ended.UserID, ended.ClientIDs).
		Update("revoked_at", s.now()).Error
	if err != nil {
		return fmt.Errorf("accounts: revoke the session's refresh grants: %w", err)
	}
	return nil
}

// SignOut ends the session behind a cookie value and tells every app it
// reached, through back-channel logout. It is what a logout does: ending only
// the row closes registre's own pages and leaves every app signed in, which
// is the bug this exists to prevent.
func (s *Service) SignOut(ctx context.Context, token string) error {
	ended, err := s.End(ctx, token)
	if err != nil {
		return err
	}
	s.announce(ended)
	return nil
}

// Replace signs out the session behind previous when it belongs to someone
// other than userID. Every path that hands a browser a new account cookie
// calls it first, so a browser switching from A to B also signs A out of the
// apps A reached: without it, A's session lived on in every app and a reload
// showed A while registre showed B. Signing in again as the same person ends
// nothing.
func (s *Service) Replace(ctx context.Context, previous, userID string) error {
	if userID == "" {
		return nil
	}
	ended, err := s.end(ctx, previous, userID)
	if err != nil {
		return err
	}
	s.announce(ended)
	return nil
}

// announce fires the back-channel logout for an ended session on a
// background context, exactly as end_session does: the request that ended the
// session must not wait on, or be cancelled with, a slow app.
func (s *Service) announce(ended Ended) {
	if s.notifier == nil || ended.UserID == "" || len(ended.ClientIDs) == 0 {
		return
	}
	go func() {
		if err := s.notifier.Fire(context.Background(), ended.UserID, ended.ClientIDs); err != nil {
			s.logger.Warn("registre: back-channel logout incomplete", slog.Any("error", err))
		}
	}()
}
