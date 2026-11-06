package account

import (
	"errors"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/social"

	"github.com/go-chi/chi/v5"
)

// connectionView is one provider on the account page: whether the holder has
// linked it and, if so, as whom.
type connectionView struct {
	Provider    string     `json:"provider"`
	Linked      bool       `json:"linked"`
	DisplayName string     `json:"display_name,omitempty"`
	LinkedAt    *time.Time `json:"linked_at,omitempty"`
}

// listConnections answers the providers worth showing the holder: those at
// least one app opted into, and any the holder has already linked. A
// deployment where no app opted in answers an empty list, so nothing about a
// provider appears until it is in use.
func (s *Service) listConnections(w http.ResponseWriter, r *http.Request) {
	if s.self.Social == nil {
		encode(w, map[string]any{"connections": []connectionView{}}, http.StatusOK)
		return
	}
	offered, err := s.self.Social.Offered(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	links, err := s.self.Social.Links(r.Context(), holder(r.Context()).ID)
	if err != nil {
		serverError(w, err)
		return
	}
	views := []connectionView{}
	linked := map[string]bool{}
	for _, link := range links {
		at := link.LinkedAt
		views = append(views, connectionView{Provider: link.Provider, Linked: true, DisplayName: link.DisplayName, LinkedAt: &at})
		linked[link.Provider] = true
	}
	for _, name := range offered {
		if !linked[name] {
			views = append(views, connectionView{Provider: name})
		}
	}
	encode(w, map[string]any{"connections": views}, http.StatusOK)
}

// linkConnection starts linking a provider account to the holder. It is a POST
// that answers the provider URL rather than a redirect, so no page a signed-in
// person visits can start a link for them, and it needs a fresh factor because
// a linked account is a new way into this one.
func (s *Service) linkConnection(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if s.self.Social == nil || !s.self.Social.Configured(provider) {
		notFound(w)
		return
	}
	if !s.fresh(w, r) {
		return
	}
	user := holder(r.Context())
	started, err := s.self.Social.Begin(r.Context(), provider, "", user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	http.SetCookie(w, s.self.Social.Cookie(started.State))
	encode(w, map[string]string{"url": started.URL}, http.StatusOK)
}

// unlinkConnection removes a linked account, behind a fresh factor, and never
// the last way into the account.
func (s *Service) unlinkConnection(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if s.self.Social == nil || !s.self.Social.Configured(provider) {
		notFound(w)
		return
	}
	if !s.fresh(w, r) {
		return
	}
	user := holder(r.Context())
	err := s.self.Social.Unlink(r.Context(), user.ID, provider)
	switch {
	case errors.Is(err, social.ErrLastFactor):
		conflict(w, "last_factor")
	case errors.Is(err, social.ErrNotLinked):
		notFound(w)
	case err != nil:
		serverError(w, err)
	default:
		s.audit(r.Context(), user.ID, "account.connection_removed", map[string]any{"provider": provider})
		encode(w, map[string]string{"ok": "true"}, http.StatusOK)
	}
}
