package logout

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

// errTransient marks a delivery failure worth retrying: the app was
// unreachable, timed out, or answered 5xx or 429. A 4xx means the app read the
// token and refused it, and sending it again changes nothing.
var errTransient = errors.New("logout: transient")

// retryDelays spaces the retries after a transient first failure: four
// attempts in all, the last starting about a minute after the logout. The
// window stays short on purpose, because porte revokes every session of the
// user and a late retry would also end one the user opened after logging out.
// Each delay is jittered by up to 20% either way, so logouts that failed
// together do not all hit the recovering app at the same instant.
var retryDelays = []time.Duration{2 * time.Second, 10 * time.Second, 45 * time.Second}

// WithLifetime bounds the background retries by ctx, the server's lifetime:
// when it ends, a pending retry stops and logs that it was abandoned.
func (s *Service) WithLifetime(ctx context.Context) *Service {
	s.lifetime = ctx
	return s
}

// refuseRedirect keeps the logout token on the registered host.
func refuseRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// retry re-sends a logout token after each of retryDelays until one delivery
// succeeds, one fails permanently, the delays run out, or the Service's
// lifetime ends.
func (s *Service) retry(userID, clientID, uri string) {
	ctx := s.lifetime
	for attempt, delay := range s.retryDelays {
		timer := time.NewTimer(jitter(delay))
		select {
		case <-ctx.Done():
			timer.Stop()
			s.abandon(clientID, uri, attempt+1)
			return
		case <-timer.C:
		}
		err := s.deliver(ctx, userID, clientID, uri)
		if err == nil {
			return
		}
		if !errors.Is(err, errTransient) {
			s.logger.Warn("registre: back-channel logout retry refused",
				slog.String("client_id", clientID), slog.String("uri", uri),
				slog.Int("attempts", attempt+2), slog.Any("error", err))
			return
		}
	}
	if ctx.Err() != nil {
		s.abandon(clientID, uri, len(s.retryDelays)+1)
		return
	}
	s.logger.Error("registre: back-channel logout gave up",
		slog.String("client_id", clientID), slog.String("uri", uri),
		slog.Int("attempts", len(s.retryDelays)+1))
}

// abandon records a retry the server's shutdown cut short.
func (s *Service) abandon(clientID, uri string, attempts int) {
	s.logger.Warn("registre: back-channel logout retry abandoned at shutdown",
		slog.String("client_id", clientID), slog.String("uri", uri),
		slog.Int("attempts", attempts))
}

// jitter spreads delay uniformly over plus or minus 20%.
func jitter(delay time.Duration) time.Duration {
	spread := delay / 5
	return delay - spread + rand.N(2*spread+1)
}
