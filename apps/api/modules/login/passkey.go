// Passkey sign-in: a discoverable WebAuthn assertion in place of an address
// and a password. A passkey is a phishing-resistant factor on its own, so a
// passkey sign-in never owes the TOTP step a password sign-in does.

package login

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
)

// WithPasskeys turns passkey sign-in on.
func (s *Service) WithPasskeys(service *passkeys.Service) *Service {
	s.passkeys = service
	return s
}

// beginPasskey answers the options for a discoverable assertion.
func (s *Service) beginPasskey(w http.ResponseWriter, r *http.Request) {
	if s.passkeys == nil {
		encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return
	}
	options, err := s.passkeys.BeginLogin(r.Context())
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, options, http.StatusOK)
}

// finishPasskey verifies the assertion and finishes the login exactly as a
// password whose account has no second factor would.
func (s *Service) finishPasskey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AuthRequest string          `json:"auth_request"`
		Ceremony    string          `json:"ceremony"`
		Credential  json.RawMessage `json:"credential"`
	}
	if s.passkeys == nil || json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPasskeyBody)).Decode(&body) != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	authReq, err := s.pendingRequest(r.Context(), body.AuthRequest)
	if err != nil {
		s.refuse(w, err)
		return
	}
	userID, err := s.passkeys.FinishLogin(r.Context(), body.Ceremony, body.Credential)
	if errors.Is(err, passkeys.ErrInvalid) {
		err = identity.ErrInvalid
	}
	if err != nil {
		s.refuse(w, err)
		return
	}
	answer, err := s.finish(r.Context(), w, userID, authReq)
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, answer, http.StatusOK)
}

// maxPasskeyBody caps an assertion. A real one is a few kilobytes.
const maxPasskeyBody = 64 << 10

// WithSignup tells the client build, through GET /api/config, that public
// sign-up is open, so the login page offers "Create your Facile account".
func (s *Service) WithSignup(open bool) *Service {
	s.signup = open
	return s
}
