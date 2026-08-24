package account

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

type ctxKey int

const holderKey ctxKey = 0

// requireCSRF refuses a mutating request that does not carry the header no
// cross-origin page can set without a preflight this API never answers.
//
// It runs ahead of requireSession so a forged write costs a header comparison
// rather than a database read and an argon2 hash. GET is exempt: it changes
// nothing, and requiring the header there would break the page's first load.
func (s *Service) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Header.Get(accounts.CSRFHeader) == "" {
			forbid(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireSession resolves the account cookie and refuses every request that
// does not carry a live one.
//
// Absent, unknown, expired and belonging-to-a-disabled-account all answer with
// the same 401. Saying which would tell whoever holds a stolen cookie what
// they are holding.
func (s *Service) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, err := s.sessions.Resolve(r.Context(), s.cookies.Read(r))
		if err != nil {
			refuse(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), holderKey, user)))
	})
}

// holder returns the human behind the request. Every route that calls it is
// behind requireSession, so the absent case is a routing mistake rather than a
// runtime one and answers with the zero user.
func holder(ctx context.Context) schemas.User {
	user, _ := ctx.Value(holderKey).(schemas.User)
	return user
}

// meView is what the account page shows of its own holder. Nothing here is a
// credential: no seed, no hash, no session token.
type meView struct {
	Email                  string `json:"email"`
	Name                   string `json:"name"`
	TOTPEnrolled           bool   `json:"totp_enrolled"`
	RecoveryCodesRemaining int    `json:"recovery_codes_remaining"`
}

// me answers who the cookie belongs to and where their second factor stands.
// The remaining count is here so nobody discovers it is zero at the moment
// they need a code.
func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	user := holder(r.Context())
	remaining, err := s.totp.RemainingRecoveryCodes(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, meView{
		Email: user.Email, Name: user.Name,
		TOTPEnrolled: user.TOTPEnrolled(), RecoveryCodesRemaining: remaining,
	}, http.StatusOK)
}

// logout ends the session and clears the cookie, whatever state the cookie was
// in. It answers 200 for a cookie that names nothing live, because the human
// who most needs to log out is the one whose session has already gone wrong.
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.End(r.Context(), s.cookies.Read(r)); err != nil {
		slog.Warn("registre: ending the account session failed", slog.Any("error", err))
	}
	s.cookies.Clear(w)
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// refuse answers the one 401 every failed authentication gets: a missing
// session, a wrong password, a wrong code, a spent recovery code. The client
// learns no, never which.
func refuse(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "unauthorized"}, http.StatusUnauthorized)
}

// forbid answers a mutating request that arrived without the CSRF header.
func forbid(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "forbidden"}, http.StatusForbidden)
}

// badRequest answers a body this API cannot read.
func badRequest(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "malformed request"}, http.StatusBadRequest)
}

// conflict answers a request that is well formed and authenticated but asks
// for a state change the account is not in, such as enrolling a factor twice.
func conflict(w http.ResponseWriter, message string) {
	encode(w, map[string]string{"error": message}, http.StatusConflict)
}

// serverError logs the cause and tells the client nothing about it.
func serverError(w http.ResponseWriter, err error) {
	slog.Error("registre: account request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
}

// encode writes one JSON object with the given status. A write failure after
// the status is sent can only be logged; the stream is already committed.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: account response write failed", slog.Any("error", err))
	}
}
