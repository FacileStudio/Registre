package signup

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
)

// maxBody caps every sign-up body. A passkey attestation is the largest, at
// a few kilobytes.
const maxBody = 64 << 10

// open answers 404 for every route while sign-up is off, so a closed
// instance looks like one that never had the feature.
func (s *Service) open(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Flow == nil {
			encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		next.ServeHTTP(w, r)
	})
}

// challenge answers a fresh ALTCHA challenge.
func (s *Service) challenge(w http.ResponseWriter, _ *http.Request) {
	challenge, err := s.Flow.Challenge()
	if err != nil {
		s.serverError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	encode(w, challenge, http.StatusOK)
}

// start answers 202 "sent" for every address once the proof of work holds,
// whether or not the address has an account, including when something
// failed after the proof: an error a caller can see would be the oracle the
// identical answer exists to remove.
func (s *Service) start(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email  string `json:"email"`
		Altcha string `json:"altcha"`
		Locale string `json:"locale"`
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
		Email: body.Email, Proof: body.Altcha, Locale: locale,
	})
	switch {
	case errors.Is(err, flow.ErrProof):
		encode(w, map[string]string{"error": "proof"}, http.StatusBadRequest)
		return
	case errors.Is(err, flow.ErrEmail):
		encode(w, map[string]string{"error": "email"}, http.StatusBadRequest)
		return
	case errors.Is(err, flow.ErrThrottled):
		encode(w, map[string]string{"error": "rate_limited"}, http.StatusTooManyRequests)
		return
	case err != nil:
		s.logger.Error("registre: sign-up start failed", slog.Any("error", err))
	}
	encode(w, map[string]string{"status": "sent"}, http.StatusAccepted)
}

// verify answers what a live link is for, without spending it.
func (s *Service) verify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	pending, err := s.Flow.Verify(r.Context(), body.Token)
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, pending, http.StatusOK)
}

// passkey begins registering the passkey the account will be created with.
func (s *Service) passkey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	options, err := s.Flow.BeginPasskey(r.Context(), body.Token)
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, options, http.StatusOK)
}

// refuse maps the flow's errors to one answer each.
func (s *Service) refuse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, flow.ErrInvalid):
		encode(w, map[string]string{"error": "invalid_link"}, http.StatusGone)
	case errors.Is(err, flow.ErrForm):
		encode(w, map[string]string{"error": "incomplete"}, http.StatusBadRequest)
	case errors.Is(err, flow.ErrBreached):
		encode(w, map[string]string{"error": "password_breached"}, http.StatusBadRequest)
	case errors.Is(err, credentials.ErrPasswordTooShort):
		encode(w, map[string]any{"error": "password_too_short", "minimum": credentials.MinPasswordLength}, http.StatusBadRequest)
	case errors.Is(err, credentials.ErrPasswordTooLong):
		encode(w, map[string]any{"error": "password_too_long", "maximum": credentials.MaxPasswordLength}, http.StatusBadRequest)
	case errors.Is(err, passkeys.ErrInvalid):
		encode(w, map[string]string{"error": "passkey"}, http.StatusBadRequest)
	default:
		s.serverError(w, err)
	}
}

// serverError logs the cause and tells the caller nothing about it.
func (s *Service) serverError(w http.ResponseWriter, err error) {
	s.logger.Error("registre: sign-up request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
}

// encode writes one JSON object with the given status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: sign-up response write failed", slog.Any("error", err))
	}
}
