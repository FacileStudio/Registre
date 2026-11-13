package login

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// ConsentPath is the page that collects acceptance of the current legal
// documents before a login may finish.
const ConsentPath = "/consent"

// consentURL is the consent page for a pending app login.
func consentURL(authRequestID string) string {
	return ConsentPath + "?auth_request=" + url.QueryEscape(authRequestID)
}

// answerCompletion answers a live-session login from what Complete
// returned: the app callback, the consent page while an acceptance is owed,
// or a refusal.
func answerCompletion(w http.ResponseWriter, completed error, authRequestID string) {
	switch {
	case completed == nil:
		encode(w, decision{Redirect: "/authorize/callback?id=" + authRequestID}, http.StatusOK)
	case errors.Is(completed, identity.ErrConsentRequired):
		encode(w, decision{Redirect: consentURL(authRequestID)}, http.StatusOK)
	case errors.Is(completed, access.ErrNoAccess):
		encode(w, map[string]string{"error": "no_access"}, http.StatusForbidden)
	default:
		encode(w, map[string]string{"error": "failed to complete auth request"}, http.StatusBadRequest)
	}
}

// pendingRequest reads the auth request a login is for, or nil when the
// login is for Registre's own pages.
func (s *Service) pendingRequest(ctx context.Context, id string) (*schemas.AuthRequest, error) {
	if id == "" {
		return nil, nil
	}
	req, err := s.identity.AuthRequestByID(ctx, id)
	if err != nil || !req.ExpiresAt.After(s.now()) || req.Done() {
		return nil, errAuthRequestGone
	}
	return &req, nil
}

// finish opens the session for a person whose every factor is proven and
// answers where the browser goes next.
func (s *Service) finish(
	r *http.Request, w http.ResponseWriter, userID string, authReq *schemas.AuthRequest,
) (decision, error) {
	if authReq != nil {
		redirect, err := s.complete(r, w, userID, *authReq)
		if err != nil {
			return decision{}, err
		}
		return decision{Redirect: redirect}, nil
	}
	redirect, err := s.completeDirect(r, w, userID)
	if err != nil {
		return decision{}, err
	}
	return decision{Redirect: redirect}, nil
}
