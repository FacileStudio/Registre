// Social sign-in: "Continue with Discord" in place of an address and a
// password. Registre sends the browser to the provider and takes it back at
// one callback. The provider proves who the person is; the second factor an
// account owes stays owed, so a TOTP-enrolled account still gets the
// challenge.

package login

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/go-chi/chi/v5"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// The reasons a social sign-in sends back to the login page as ?error=. Each
// matches the page's PLAIN_REASON shape, because the page prints nothing else,
// and each is a key of the page's own translation table.
const (
	reasonExpired  = "the sign-in link expired"
	reasonCanceled = "sign-in was cancelled"
	reasonFailed   = "the sign-in failed, try again"
	reasonClosed   = "sign-up is closed"
	reasonNoEmail  = "the provider shared no verified email"
	reasonTaken    = "this account is linked to another user"
	reasonNoAccess = "no access to this app"
)

// WithSocial turns social sign-in on.
func (s *Service) WithSocial(service *social.Service) *Service {
	s.social = service
	return s
}

// providers answers which social buttons the login for ?auth_request= shows:
// the providers configured here that the app sending the person opted into.
// It answers an empty list for no request, a gone one, or an app with none,
// so the page cannot tell them apart.
func (s *Service) providers(w http.ResponseWriter, r *http.Request) {
	out := []string{}
	if s.social != nil {
		if authReq, err := s.pendingRequest(r.Context(), r.URL.Query().Get("auth_request")); err == nil && authReq != nil {
			if names, err := s.social.ForClient(r.Context(), authReq.ClientID); err == nil {
				out = names
			}
		}
	}
	encode(w, map[string]any{"providers": out}, http.StatusOK)
}

// socialStart sends the browser to the provider. The state it records is bound
// to the auth request and, through the cookie, to this browser.
func (s *Service) socialStart(w http.ResponseWriter, r *http.Request) {
	id, provider := r.URL.Query().Get("auth_request"), chi.URLParam(r, "provider")
	authReq, err := s.pendingRequest(r.Context(), id)
	if s.social == nil || err != nil || authReq == nil {
		s.socialFailure(w, r, "", reasonExpired)
		return
	}
	allowed, err := s.social.ForClient(r.Context(), authReq.ClientID)
	if err != nil || !slices.Contains(allowed, provider) {
		s.socialFailure(w, r, id, reasonFailed)
		return
	}
	started, err := s.social.Begin(r.Context(), provider, id, "")
	if err != nil {
		s.logger.Error("registre: begin a social sign-in", slog.Any("error", err))
		s.socialFailure(w, r, id, reasonFailed)
		return
	}
	http.SetCookie(w, s.social.Cookie(started.State))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, started.URL, http.StatusFound)
}

// socialCallback is where every provider sends the browser back. The state is
// spent first and the cookie cleared whatever happens, so a callback replays
// nothing.
func (s *Service) socialCallback(w http.ResponseWriter, r *http.Request) {
	if s.social == nil {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	row, err := s.social.Take(r.Context(), query.Get("state"), s.social.Held(r))
	http.SetCookie(w, s.social.ClearCookie())
	if err != nil || row.Provider != chi.URLParam(r, "provider") {
		s.socialFailure(w, r, "", reasonExpired)
		return
	}
	if query.Get("error") != "" || query.Get("code") == "" {
		s.socialFailure(w, r, row.AuthRequestID, reasonCanceled)
		return
	}
	profile, err := s.social.Complete(r.Context(), row, query.Get("code"))
	if err != nil {
		s.logger.Warn("registre: a social sign-in did not complete", slog.Any("error", err))
		s.socialFailure(w, r, row.AuthRequestID, reasonFailed)
		return
	}
	if row.UserID != "" {
		s.finishLink(w, r, row, profile)
		return
	}
	s.finishSocialLogin(w, r, row, profile)
}

// finishSocialLogin resolves the profile to a user and finishes the login the
// way a password would: a challenge for an enrolled account, the app callback
// otherwise.
func (s *Service) finishSocialLogin(w http.ResponseWriter, r *http.Request, row schemas.SocialState, profile social.Profile) {
	ctx := r.Context()
	authReq, err := s.pendingRequest(ctx, row.AuthRequestID)
	if err != nil || authReq == nil {
		s.socialFailure(w, r, "", reasonExpired)
		return
	}
	user, err := s.social.Resolve(ctx, row.Provider, profile)
	if err != nil {
		s.socialFailure(w, r, row.AuthRequestID, s.reasonFor(err))
		return
	}
	if user.TOTPEnrolled() {
		s.socialChallenge(w, r, user.ID, row.AuthRequestID)
		return
	}
	answer, err := s.finish(ctx, w, user.ID, authReq)
	if errors.Is(err, access.ErrNoAccess) {
		s.socialFailure(w, r, row.AuthRequestID, reasonNoAccess)
		return
	}
	if err != nil {
		s.logger.Error("registre: finish a social sign-in", slog.Any("error", err))
		s.socialFailure(w, r, row.AuthRequestID, reasonFailed)
		return
	}
	http.Redirect(w, r, answer.Redirect, http.StatusFound)
}

// socialChallenge hands an enrolled account to the second-factor step. It
// fails closed on a build that cannot check a factor.
func (s *Service) socialChallenge(w http.ResponseWriter, r *http.Request, userID, authRequestID string) {
	if s.factor.Challenges == nil || s.factor.Codes == nil {
		s.logger.Error("registre: an enrolled account met a build with no second factor service")
		s.socialFailure(w, r, authRequestID, reasonFailed)
		return
	}
	token, err := s.factor.Challenges.Issue(r.Context(), userID, authRequestID)
	if err != nil {
		s.logger.Error("registre: issue a social sign-in challenge", slog.Any("error", err))
		s.socialFailure(w, r, authRequestID, reasonFailed)
		return
	}
	query := url.Values{"auth_request": {authRequestID}, "challenge": {token}}
	http.Redirect(w, r, "/login?"+query.Encode(), http.StatusFound)
}

// reasonFor maps a resolution failure to the reason the page shows. A
// disabled account and an unexpected error read the same, so the page does not
// say which accounts exist.
func (s *Service) reasonFor(err error) string {
	switch {
	case errors.Is(err, social.ErrSignupClosed):
		return reasonClosed
	case errors.Is(err, social.ErrUnverifiedEmail):
		return reasonNoEmail
	case errors.Is(err, social.ErrAlreadyLinked), errors.Is(err, social.ErrTaken):
		return reasonTaken
	case errors.Is(err, social.ErrDisabled):
		return reasonFailed
	}
	s.logger.Error("registre: resolve a social sign-in", slog.Any("error", err))
	return reasonFailed
}

// socialFailure sends the browser back to the login page with a reason.
func (s *Service) socialFailure(w http.ResponseWriter, r *http.Request, authRequestID, reason string) {
	query := url.Values{"error": {reason}}
	if authRequestID != "" {
		query.Set("auth_request", authRequestID)
	}
	http.Redirect(w, r, "/login?"+query.Encode(), http.StatusFound)
}
