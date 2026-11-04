package logout

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// UserDeletedEvent is the event key of the notice an app receives when an
// account is erased. SPEC §6 documents the payload for apps.
const UserDeletedEvent = "https://schemas.facile.studio/event/user-deleted"

// deletedTTL is how long a deletion notice verifies. Longer than a logout
// token because an app may queue it before acting.
const deletedTTL = 10 * time.Minute

// NotifyDeleted POSTs a signed Security Event Token (RFC 8417, delivered as
// RFC 8935 push: the body is the JWT, Content-Type application/secevent+jwt)
// to every enabled application's directory sync URI, telling it userID no
// longer exists and that it must erase what it holds about them within 30
// days.
//
// It goes to the sync URI because that is the one endpoint every syncing app
// already exposes; an app that ignores the body still pulls the snapshot,
// which no longer lists the person. Delivery is best-effort, like logout,
// and every failure is logged.
func (s *Service) NotifyDeleted(ctx context.Context, userID string) error {
	clients, err := s.register.List(ctx)
	if err != nil {
		return fmt.Errorf("logout: list the apps: %w", err)
	}
	for _, client := range clients {
		if client.DirectorySyncURI == "" || client.DisabledAt != nil {
			continue
		}
		if err := s.deliverDeleted(ctx, userID, client.ClientID, client.DirectorySyncURI); err != nil {
			s.logger.Warn("registre: user.deleted notice failed",
				slog.String("client_id", client.ClientID), slog.Any("error", err))
		}
	}
	return nil
}

// deliverDeleted signs and posts one notice.
func (s *Service) deliverDeleted(ctx context.Context, userID, clientID, uri string) error {
	claims, err := s.claims(userID, clientID, deletedTTL)
	if err != nil {
		return err
	}
	claims["events"] = map[string]any{UserDeletedEvent: map[string]any{"erase_within_days": 30}}
	token, err := s.sign(claims, "secevent+jwt")
	if err != nil {
		return err
	}
	post, err := http.NewRequestWithContext(ctx, http.MethodPost, uri, strings.NewReader(token))
	if err != nil {
		return fmt.Errorf("logout: build the notice: %w", err)
	}
	post.Header.Set("Content-Type", "application/secevent+jwt")
	response, err := s.deliveries.Do(post)
	if err != nil {
		return fmt.Errorf("logout: deliver the notice: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 300 {
		return fmt.Errorf("logout: %s answered %d", uri, response.StatusCode)
	}
	return nil
}
