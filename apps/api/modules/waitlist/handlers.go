package waitlist

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/internal/waitlist"
)

// maxBody caps every waitlist body. The largest is an ALTCHA solution, under
// two kilobytes.
const maxBody = 4 << 10

// joinBody is what a landing page posts.
type joinBody struct {
	Email   string `json:"email"`
	Product string `json:"product"`
	Locale  string `json:"locale"`
	Consent string `json:"consent"`
	Altcha  string `json:"altcha"`
}

// challenge answers a fresh ALTCHA challenge.
func (s *Service) challenge(w http.ResponseWriter, _ *http.Request) {
	challenge, err := s.Proofs.Challenge()
	if err != nil {
		s.serverError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	encode(w, challenge, http.StatusOK)
}

// join validates the entry, spends the proof of work, then records the
// address. The answer is the same for a new and a known address.
func (s *Service) join(w http.ResponseWriter, r *http.Request) {
	var body joinBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	entry, err := waitlist.Validate(waitlist.Entry{
		Email: body.Email, Product: body.Product, Locale: body.Locale, Consent: body.Consent,
	})
	if err != nil {
		s.refuse(w, err)
		return
	}
	if err := s.Proofs.Spend(r.Context(), body.Altcha); err != nil {
		s.refuse(w, err)
		return
	}
	if err := s.List.Join(r.Context(), entry); err != nil {
		s.serverError(w, err)
		return
	}
	encode(w, map[string]bool{"ok": true}, http.StatusAccepted)
}

// unsubscribe deletes the entry a token names. The token comes in the query
// string, as in a List-Unsubscribe one-click POST, or in a JSON body.
func (s *Service) unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		var body struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			token = body.Token
		}
	}
	if err := s.List.Unsubscribe(r.Context(), token); err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, map[string]bool{"ok": true}, http.StatusOK)
}

// refusals maps each caller mistake to its error code.
var refusals = []struct {
	err  error
	code string
}{
	{waitlist.ErrEmail, "invalid_email"},
	{waitlist.ErrProduct, "invalid_product"},
	{waitlist.ErrLocale, "invalid_locale"},
	{waitlist.ErrConsent, "consent_required"},
	{waitlist.ErrToken, "invalid_token"},
	{flow.ErrProof, "invalid_proof"},
}

// refuse answers 400 with the caller mistake's code, or 500 for anything else.
func (s *Service) refuse(w http.ResponseWriter, err error) {
	for _, refusal := range refusals {
		if errors.Is(err, refusal.err) {
			encode(w, map[string]string{"error": refusal.code}, http.StatusBadRequest)
			return
		}
	}
	s.serverError(w, err)
}

// serverError logs the cause and tells the caller nothing about it.
func (s *Service) serverError(w http.ResponseWriter, err error) {
	s.logger.Error("registre: waitlist request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
}

// encode writes one JSON object with the given status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: waitlist response write failed", slog.Any("error", err))
	}
}
