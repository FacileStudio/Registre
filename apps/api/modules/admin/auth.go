package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

type ctxKey int

const operatorKey ctxKey = 0

// requireOperator refuses every request that does not carry an admin
// account's email and password as HTTP Basic credentials, verified through
// the same identity path and argon2 ceiling as the CLI's operator commands.
//
// One refusal for unknown address, wrong password, right password on a
// non-admin and right password on a disabled account: saying which is an
// enumeration oracle.
func (s *Service) requireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, password, ok := r.BasicAuth()
		if !ok {
			refuse(w)
			return
		}
		user, err := s.users.Verify(r.Context(), email, password)
		if err != nil || !user.Admin || !user.Enabled() {
			refuse(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), operatorKey, user)))
	})
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
