package logout

import (
	"context"
	"fmt"
	"io"
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

// DeliverDeleted signs one Security Event Token (RFC 8417) telling clientID
// that userID no longer exists and must be erased within 30 days, and
// pushes it to uri as RFC 8935 describes: the body is the JWT, the type
// application/secevent+jwt. internal/gdpr persists the notice and retries a
// failed push with backoff.
func (s *Service) DeliverDeleted(ctx context.Context, userID, clientID, uri string) error {
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
