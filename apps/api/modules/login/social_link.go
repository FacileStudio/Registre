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

// finishLink attaches the profile to the user who started the link, once the
// browser still holds that user's session. A session that changed hands
// between the click and the callback links nothing.
func (s *Service) finishLink(w http.ResponseWriter, r *http.Request, row schemas.SocialState, profile social.Profile) {
	if s.factor.Accounts == nil {
		http.Redirect(w, r, connectionsPage+"?connection=failed", http.StatusFound)
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
	http.Redirect(w, r, connectionsPage+"?connection="+outcome, http.StatusFound)
}
