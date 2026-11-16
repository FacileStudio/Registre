package admin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// refreshing wraps a user handler so that, after it answers with a success,
// the directory version is bumped and apps are nudged. Operators hold every
// entitlement in the snapshot, so creating, promoting, demoting, disabling or
// enabling an account changes what apps must read, and an app skips a
// snapshot whose version it has already applied.
//
// A failed bump is only logged: the account change already committed, and
// apps converge at the next version bump.
func (s *Service) refreshing(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next(wrapped, r)
		if s.directory == nil || wrapped.Status() >= 300 {
			return
		}
		if err := s.directory.Touch(r.Context()); err != nil {
			s.logger.Warn("registre: directory bump after an account change", slog.String("error", err.Error()))
		}
	}
}
