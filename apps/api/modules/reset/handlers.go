package reset

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
)

// MinPasswordLength is the floor a reset may set, shared with every other
// path that writes the same column. See credentials.MinPasswordLength for why
// it is defined once.
const MinPasswordLength = credentials.MinPasswordLength

// request starts a recovery and always answers the same way.
//
// The status, the body and the timing are identical whether or not the
// address is known, which is SPEC §7's rule. That includes the failure case:
// an internal error is logged and answered with the success body, because an
// error visible to the caller is the enumeration oracle the equal timing was
// there to prevent. Nothing a caller could do with the difference is
// legitimate, and the person who typed their own address is told the same
// true thing either way.
func (s *Service) request(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		encode(w, map[string]string{"status": "sent"}, http.StatusOK)
		return
	}
	if s.resets != nil {
		if err := s.resets.Request(r.Context(), body.Email); err != nil {
			s.logger.Error("registre: recovery request failed", slog.Any("error", err))
		}
	}
	encode(w, map[string]string{"status": "sent"}, http.StatusOK)
}

// redeem spends a reset token and sets the new password.
//
// Unlike request this one does answer truthfully, because by the time
// somebody holds a token there is nothing left to enumerate: the token either
// exists or it does not, and it names an address whoever holds it already
// received. One refusal covers unknown, expired and already-spent.
func (s *Service) redeem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	if credentials.CheckLength(body.Password) != nil {
		encode(w, map[string]string{"error": "password_too_short"}, http.StatusBadRequest)
		return
	}
	if s.resets == nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusUnauthorized)
		return
	}
	err := s.resets.Redeem(r.Context(), body.Token, body.Password)
	if errors.Is(err, recovery.ErrInvalid) {
		encode(w, map[string]string{"error": "invalid"}, http.StatusUnauthorized)
		return
	}
	if err != nil {
		s.logger.Error("registre: recovery redemption failed", slog.Any("error", err))
		encode(w, map[string]string{"error": "invalid"}, http.StatusInternalServerError)
		return
	}
	encode(w, map[string]string{"status": "reset"}, http.StatusOK)
}

// encode writes one JSON object with the given status. A write failure after
// the status is sent can only be logged; the stream is already committed.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: recovery response write failed", slog.Any("error", err))
	}
}
