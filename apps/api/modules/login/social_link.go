package login

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// connectionsPage is where a finished link sends the signed-in person back.
const connectionsPage = "/account/security"

// linkRedirect sends the browser back from a link: to the app that asked,
// with connection=outcome merged into its query, when the state carries a
// return_to that is still valid, and to the account page otherwise.
func (s *Service) linkRedirect(w http.ResponseWriter, r *http.Request, row schemas.SocialState, outcome string) {
	if to := s.social.ReturnURL(r.Context(), row.Provider, row.ReturnTo, "connection", outcome); to != "" {
		http.Redirect(w, r, to, http.StatusFound)
		return
	}
	http.Redirect(w, r, connectionsPage+"?connection="+outcome+"&provider="+row.Provider, http.StatusFound)
}

// finishLink attaches the profile to the user who started the link, once the
// browser still holds that user's session. A session that changed hands
// between the click and the callback links nothing.
func (s *Service) finishLink(w http.ResponseWriter, r *http.Request, row schemas.SocialState, profile social.Profile) {
	if s.factor.Accounts == nil {
		s.linkRedirect(w, r, row, "failed")
		return
	}
	user, _, err := s.factor.Accounts.Resolve(r.Context(), s.factor.Cookies.Read(r))
	if err != nil || user.ID != row.UserID {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	outcome := "linked"
	switch err := s.social.LinkTo(r.Context(), user.ID, row.Provider, profile); {
	case errors.Is(err, social.ErrTaken), errors.Is(err, social.ErrAlreadyLinked):
		outcome = "taken"
	case err != nil:
		s.logger.Error("registre: link a social account", slog.Any("error", err))
		outcome = "failed"
	}
	s.linkRedirect(w, r, row, outcome)
}
