package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func (s *Service) PurgeAuthRequests(ctx context.Context, cutoff time.Time) error {
	err := s.db.WithContext(ctx).
		Where("expires_at < ?", cutoff).
		Delete(&schemas.AuthRequest{}).Error
	if err != nil {
		return fmt.Errorf("identity: purge auth requests: %w", err)
	}
	return nil
}

func (s *Service) AuthRequestByID(ctx context.Context, id string) (schemas.AuthRequest, error) {
	var row schemas.AuthRequest
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		return schemas.AuthRequest{}, fmt.Errorf("identity: read the auth request: %w", err)
	}
	return row, nil
}

// Complete binds an authenticated user to a pending auth request, after
// checking that the user may use the app the request is for. It answers
// access.ErrNoAccess for a restricted app the user holds no grant to, so every
// sign-in path, password, second factor or live session, is gated in one place.
func (s *Service) Complete(ctx context.Context, id, userID, sessionID string, at time.Time) error {
	var clientID string
	if err := s.db.WithContext(ctx).Model(&schemas.AuthRequest{}).Select("client_id").
		Where("id = ?", id).Scan(&clientID).Error; err != nil {
		return fmt.Errorf("identity: read the auth request's app: %w", err)
	}
	if err := access.New(s.db, "").Require(ctx, clientID, userID); err != nil {
		return err
	}
	if err := s.requireConsent(ctx, userID); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Model(&schemas.AuthRequest{}).
		Where("id = ? AND user_id IS NULL AND expires_at > ?", id, at).
		Updates(map[string]any{"user_id": userID, "authenticated_at": at, "session_id": sessionID})
	if result.Error != nil {
		return fmt.Errorf("identity: complete the login: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrInvalid
	}
	return nil
}

func (s *Service) StartSession(ctx context.Context, userID, clientID string, at time.Time) (string, error) {
	sessionID, err := ids.New()
	if err != nil {
		return "", fmt.Errorf("identity: mint a session id: %w", err)
	}
	row := schemas.Session{ID: sessionID, UserID: userID, CreatedAt: at}
	if clientID != "" {
		row.ClientIDs = schemas.StringList{clientID}
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("identity: open the session: %w", err)
	}
	return sessionID, nil
}

func (s *Service) TouchSession(ctx context.Context, sessionID, clientID string) error {
	if clientID == "" {
		return nil
	}
	var session schemas.Session
	if err := s.db.WithContext(ctx).Where("id = ? AND ended_at IS NULL", sessionID).Take(&session).Error; err != nil {
		return fmt.Errorf("identity: read the session: %w", err)
	}
	if session.ClientIDs.Contains(clientID) {
		return nil
	}
	session.ClientIDs = append(session.ClientIDs, clientID)
	if err := s.db.WithContext(ctx).Model(&schemas.Session{}).
		Where("id = ?", sessionID).
		Update("client_ids", session.ClientIDs).Error; err != nil {
		return fmt.Errorf("identity: attach client to session: %w", err)
	}
	return nil
}

func (s *Service) EndSessions(ctx context.Context, userID string, at time.Time) ([]string, error) {
	if err := s.db.WithContext(ctx).Model(&schemas.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error; err != nil {
		return nil, fmt.Errorf("identity: revoke the refresh grants: %w", err)
	}
	result := s.db.WithContext(ctx).Model(&schemas.Session{}).
		Where("user_id = ? AND ended_at IS NULL", userID).
		Update("ended_at", at)
	if result.Error != nil {
		return nil, fmt.Errorf("identity: end the sessions: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	var rows []schemas.Session
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND ended_at = ?", userID, at).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("identity: read the ended sessions: %w", err)
	}
	clients := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		for _, clientID := range row.ClientIDs {
			if !seen[clientID] {
				seen[clientID] = true
				clients = append(clients, clientID)
			}
		}
	}
	return clients, nil
}

// DropSession deletes a session that never reached anyone, such as one opened
// for a login an access policy then refused. Leaving it would be a live
// session row with no cookie and no app, which back-channel logout would
// later announce for nothing. Failure is ignored: the row expires anyway.
func (s *Service) DropSession(ctx context.Context, sessionID string) {
	_ = s.db.WithContext(ctx).Where("id = ? AND token_hash IS NULL", sessionID).Delete(&schemas.Session{}).Error
}
