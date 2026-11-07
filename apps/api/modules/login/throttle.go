package login

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/throttle"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const (
	passwordKind = "password"
	totpKind     = "totp"
)

// busyRetryAfter is what a client is told to wait when every argon2 slot was
// taken for the whole queue wait.
const busyRetryAfter = 2 * time.Second

// tooManyAttempts is the refusal a throttled attempt gets. It carries the same
// status and body whether the address exists or not, because the throttle
// counts invented addresses exactly as it counts real ones.
type tooManyAttempts struct{ retryAfter time.Duration }

func (e tooManyAttempts) Error() string {
	return "login: too many attempts, retry in " + e.retryAfter.String()
}

// WithThrottle adds the per-account failed-login throttle. A service built
// without it throttles nothing but the per-address budget, which is what tests
// of the other paths want; production always wires it.
func (s *Service) WithThrottle(guard *throttle.Service) *Service {
	s.throttle = guard
	return s
}

// claim takes one attempt on the account behind identity, or returns
// tooManyAttempts without having touched argon2.
func (s *Service) claim(ctx context.Context, kind, identity string) error {
	if s.throttle == nil {
		return nil
	}
	wait, err := s.throttle.Attempt(ctx, throttle.Key(kind, identity))
	if err != nil {
		return err
	}
	if wait > 0 {
		return tooManyAttempts{retryAfter: wait}
	}
	return nil
}

// forgive clears the account's counter after a success. A failure to clear is
// logged and not surfaced: the person proved who they are, and the counter
// expires on its own.
func (s *Service) forgive(ctx context.Context, kind, identity string) {
	if s.throttle == nil {
		return
	}
	if err := s.throttle.Reset(ctx, throttle.Key(kind, identity)); err != nil {
		s.logger.Error("registre: login throttle reset failed", slog.Any("error", err))
	}
}

// verifyThrottled is identity.Verify behind the per-account throttle: the
// attempt is claimed before argon2 runs and cleared when the password is right.
func (s *Service) verifyThrottled(ctx context.Context, email, password string) (schemas.User, error) {
	if err := s.claim(ctx, passwordKind, email); err != nil {
		return schemas.User{}, err
	}
	user, err := s.identity.Verify(ctx, email, password)
	if err != nil {
		return schemas.User{}, err
	}
	s.forgive(ctx, passwordKind, email)
	return user, nil
}

// checkCodeThrottled is checkCode behind its own per-account throttle, keyed
// on the subject and kept apart from the password key: a password login that
// resets the password counter must not reset the code guesses with it.
func (s *Service) checkCodeThrottled(ctx context.Context, user schemas.User, code string) error {
	if err := s.claim(ctx, totpKind, user.ID); err != nil {
		return err
	}
	if err := s.checkCode(ctx, user, code); err != nil {
		return err
	}
	s.forgive(ctx, totpKind, user.ID)
	return nil
}

// refuseOverload answers the two refusals that are about load rather than
// credentials: a throttled account, 429, and a saturated verifier, 503. Both
// carry Retry-After, and neither says anything about whether the account
// exists.
func (s *Service) refuseOverload(w http.ResponseWriter, err error) bool {
	var throttled tooManyAttempts
	switch {
	case errors.As(err, &throttled):
		w.Header().Set("Retry-After", seconds(throttled.retryAfter))
		encode(w, map[string]string{"error": "too_many_attempts"}, http.StatusTooManyRequests)
	case errors.Is(err, credentials.ErrBusy):
		w.Header().Set("Retry-After", seconds(busyRetryAfter))
		encode(w, map[string]string{"error": "busy"}, http.StatusServiceUnavailable)
	default:
		return false
	}
	return true
}

func seconds(d time.Duration) string {
	return strconv.Itoa(int(math.Ceil(d.Seconds())))
}
