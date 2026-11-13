package signup

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
)

// start answers 202 "sent" once the mail is on its way, whether or not the
// address has an account: both branches send one. Every refusal it can give
// (bad proof, bad address, too many links, a mail that could not be sent)
// is decided without knowing whether the address has an account, so none
// of them is an oracle, and a person is never told "sent" when nothing was.
func (s *Service) start(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Altcha      string `json:"altcha"`
		Locale      string `json:"locale"`
		AuthRequest string `json:"auth_request"`
		Invitation  string `json:"invitation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	locale := body.Locale
	if !i18n.Supported(locale) {
		locale = i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	}
	err := s.Flow.Start(r.Context(), flow.Request{
		Email: body.Email, Proof: body.Altcha, Locale: locale, AuthRequest: s.bound(r, body.AuthRequest),
		Invitation: body.Invitation,
	})
	if err != nil {
		s.refuseStart(w, err)
		return
	}
	encode(w, map[string]string{"status": "sent"}, http.StatusAccepted)
}

// refuseStart maps a failed start to its answer. A throttled address learns
// how long to wait, in seconds, in the body and in Retry-After.
func (s *Service) refuseStart(w http.ResponseWriter, err error) {
	var throttled *flow.Throttled
	switch {
	case errors.Is(err, flow.ErrProof):
		encode(w, map[string]string{"error": "proof"}, http.StatusBadRequest)
	case errors.Is(err, flow.ErrEmail):
		encode(w, map[string]string{"error": "email"}, http.StatusBadRequest)
	case errors.As(err, &throttled):
		wait := int(math.Ceil(throttled.Wait.Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(wait))
		encode(w, map[string]any{"error": "rate_limited", "retry_after": wait}, http.StatusTooManyRequests)
	case errors.Is(err, flow.ErrMail):
		s.logger.Error("registre: sign-up mail could not be sent", slog.String("error", err.Error()))
		encode(w, map[string]string{"error": "mail_failed"}, http.StatusServiceUnavailable)
	default:
		s.logger.Error("registre: sign-up start failed", slog.String("error", flow.Redact(err.Error())))
		encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
	}
}
