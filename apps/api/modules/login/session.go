// The session half of the login module: finishing a flow with the account
// session the browser already holds, which is how a human signed in to
// registre reaches a second app without typing a password again.

package login

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
)

// sessionLogin finishes a pending flow with the session the browser already
// holds, or, with no flow named, says where a signed-in human goes.
//
// Finishing a flow is a POST carrying the CSRF header, with the flow id in the
// body. A GET only answers the probe: a GET that finished flows would let any
// link a signed-in human follows finish a flow somebody else started, because
// the account cookie is Lax and rides along on a top-level navigation.
func (s *Service) sessionLogin(w http.ResponseWriter, r *http.Request) {
	authRequestID, ok := requestedFlow(r)
	if !ok {
		encode(w, map[string]string{"error": "forbidden"}, http.StatusForbidden)
		return
	}
	if s.factor.Accounts == nil {
		encode(w, map[string]any{"logged_in": false}, http.StatusUnauthorized)
		return
	}
	user, session, err := s.factor.Accounts.Resolve(r.Context(), s.factor.Cookies.Read(r))
	if err != nil {
		s.factor.Cookies.Clear(w)
		encode(w, map[string]any{"logged_in": false}, http.StatusUnauthorized)
		return
	}
	if authRequestID == "" {
		encode(w, decision{Redirect: "/"}, http.StatusOK)
		return
	}
	s.resume(r.Context(), w, authRequestID, user.ID, session.ID)
}

// requestedFlow reads which flow a session login asks to finish. It refuses a
// GET that names one and a POST without the CSRF header.
func requestedFlow(r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		return "", !r.URL.Query().Has("auth_request")
	}
	if r.Header.Get(accounts.CSRFHeader) == "" {
		return "", false
	}
	var body struct {
		AuthRequest string `json:"auth_request"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.AuthRequest, true
}

// resume completes one pending flow with an existing session.
func (s *Service) resume(ctx context.Context, w http.ResponseWriter, authRequestID, userID, sessionID string) {
	authReq, err := s.identity.AuthRequestByID(ctx, authRequestID)
	if err != nil || !authReq.ExpiresAt.After(s.now()) || authReq.Done() {
		encode(w, map[string]string{"error": "auth request is gone"}, http.StatusBadRequest)
		return
	}
	if err := s.identity.TouchSession(ctx, sessionID, authReq.ClientID); err != nil {
		s.logger.Warn("registre: touch session client", slog.Any("error", err))
	}
	if err := s.identity.Complete(ctx, authReq.ID, userID, sessionID, s.now()); err != nil {
		if errors.Is(err, access.ErrNoAccess) {
			encode(w, map[string]string{"error": "no_access"}, http.StatusForbidden)
			return
		}
		encode(w, map[string]string{"error": "failed to complete auth request"}, http.StatusBadRequest)
		return
	}
	encode(w, decision{Redirect: "/authorize/callback?id=" + authReq.ID}, http.StatusOK)
}
