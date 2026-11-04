package device

import (
	"encoding/json"
	"errors"
	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
)

// pendingView is what the page shows before anybody approves anything: which
// program is asking, what it would be allowed to do, and how long is left.
//
// Nothing here is a credential and nothing identifies the machine that started
// the flow. The human is being asked to vouch for a program, not to audit a
// host, and a hostname the CLI supplied would be a string an attacker chooses.
type pendingView struct {
	ClientID   string    `json:"client_id"`
	ClientName string    `json:"client_name"`
	Scopes     []string  `json:"scopes"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// pending answers GET /api/device?user_code=BCDF-GHJK.
//
// The code is read from the query rather than a body so the verification page
// can be opened straight from the link the CLI prints — RFC 8628's
// verification_uri_complete is exactly that URL — and the page can render what
// is being asked before the human does anything.
func (s *Service) pending(w http.ResponseWriter, r *http.Request) {
	row, err := s.devices.Pending(r.Context(), r.URL.Query().Get("user_code"))
	if err != nil {
		s.refuse(w, err)
		return
	}
	view := pendingView{ClientID: row.ClientID, ClientName: row.ClientID, Scopes: row.Scopes, ExpiresAt: row.ExpiresAt}
	if client, err := s.register.ByClientID(r.Context(), row.ClientID); err == nil {
		view.ClientName = client.Name
	}
	encode(w, view, http.StatusOK)
}

// approve answers POST /api/device/approve.
//
// The subject and the authentication time come from the session the human is
// already holding with registre, never from the request: a page that could
// name the account it is approving for would be an authorization bypass with a
// JSON body.
func (s *Service) approve(w http.ResponseWriter, r *http.Request) {
	code, ok := s.userCode(w, r)
	if !ok {
		return
	}
	user, session := accounts.Holder(r.Context())
	if s.consent != nil {
		if err := s.consent(r.Context(), user.ID); err != nil {
			s.refuse(w, err)
			return
		}
	}
	err := s.devices.Approve(r.Context(), code, devices.Approval{
		UserID: user.ID, AMR: []string{"pwd"}, AuthTime: session.CreatedAt,
	})
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, map[string]string{"status": "approved"}, http.StatusOK)
}

// deny answers POST /api/device/deny. It is the answer for a code somebody did
// not expect to see, so it must be as easy to give as the other one: a flow
// left unanswered still works for whoever started it until it expires.
func (s *Service) deny(w http.ResponseWriter, r *http.Request) {
	code, ok := s.userCode(w, r)
	if !ok {
		return
	}
	if err := s.devices.Deny(r.Context(), code); err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, map[string]string{"status": "denied"}, http.StatusOK)
}

// userCode reads the code out of a JSON body, answering the caller itself when
// the body is unreadable.
func (s *Service) userCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		UserCode string `json:"user_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserCode == "" {
		encode(w, map[string]string{"error": "malformed request"}, http.StatusBadRequest)
		return "", false
	}
	return body.UserCode, true
}

// refuse answers the one 404 an unknown, expired, already-answered or
// already-spent code gets, 403 no_access when the signed-in human may not use
// the requesting app, and 500 for anything registre broke.
//
// One status for all four states is the point. Telling a caller that a code
// exists but was already used confirms they found a real one, which is the
// only thing worth learning from guessing at this endpoint.
func (s *Service) refuse(w http.ResponseWriter, err error) {
	if errors.Is(err, devices.ErrUnknownCode) {
		encode(w, map[string]string{"error": "unknown_code"}, http.StatusNotFound)
		return
	}
	if errors.Is(err, access.ErrNoAccess) {
		encode(w, map[string]string{"error": "no_access"}, http.StatusForbidden)
		return
	}
	if errors.Is(err, identity.ErrConsentRequired) {
		encode(w, map[string]string{"error": "consent_required"}, http.StatusForbidden)
		return
	}
	s.logger.Error("registre: device verification failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
}

// encode writes one JSON object with the given status. A write failure after
// the status is sent can only be logged; the stream is already committed.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: device response write failed", slog.Any("error", err))
	}
}
