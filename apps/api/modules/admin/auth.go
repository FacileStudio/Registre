package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

type ctxKey int

const operatorKey ctxKey = 0

func (s *Service) resolveOperator(r *http.Request) (schemas.User, bool) {
	email, password, ok := r.BasicAuth()
	if ok {
		user, err := s.users.Verify(r.Context(), email, password)
		return user, err == nil && user.Admin && user.Enabled()
	}
	if s.sessions == nil {
		return schemas.User{}, false
	}
	user, _, err := s.sessions.Resolve(r.Context(), s.cookies.Read(r))
	return user, err == nil && user.Admin && user.Enabled()
}

// requireOperator refuses every request that does not carry an admin
// account's credentials (either as HTTP Basic or via a live admin session
// cookie), verified through the same identity path and argon2 ceiling as the
// CLI's operator commands.
func (s *Service) requireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.resolveOperator(r)
		if !ok {
			refuse(w)
			return
		}
		if _, _, basic := r.BasicAuth(); !basic && r.Method != http.MethodGet && r.Header.Get(accounts.CSRFHeader) == "" {
			forbid(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), operatorKey, user)))
	})
}

func forbid(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "forbidden"}, http.StatusForbidden)
}

// refuse answers the one 401 every failed authentication gets.
func refuse(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="registre admin"`)
	encode(w, map[string]string{"error": "unauthorized"}, http.StatusUnauthorized)
}

// encode writes one JSON object with the given status. A write failure after
// the status is sent can only be logged; the stream is already committed.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: admin response write failed", slog.Any("error", err))
	}
}
