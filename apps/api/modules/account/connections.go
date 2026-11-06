package account

import (
	"encoding/json"
	"errors"
	"io"
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

// maxLinkBody caps the optional link body, which holds one URL.
const maxLinkBody = 4 << 10

// linkConnection starts linking a provider account to the holder. The optional
// JSON body {"return_to": url} names where the browser goes once the link
// finishes; anything but a registered client's origin is a 400
// invalid_return_to. It is a POST
// that answers the provider URL rather than a redirect, so no page a signed-in
// person visits can start a link for them, and it needs a fresh factor because
// a linked account is a new way into this one.
func (s *Service) linkConnection(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if s.self.Social == nil || !s.self.Social.Configured(provider) {
		notFound(w)
		return
	}
	var body struct {
		ReturnTo string `json:"return_to"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLinkBody)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		badRequest(w)
		return
	}
	if !s.fresh(w, r) {
		return
	}
	user := holder(r.Context())
	started, err := s.self.Social.Begin(r.Context(), provider, "", user.ID, body.ReturnTo)
	if errors.Is(err, social.ErrInvalidReturn) {
		encode(w, map[string]string{"error": "invalid_return_to"}, http.StatusBadRequest)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.SetCookie(w, s.self.Social.Cookie(started.State))
	encode(w, map[string]string{"url": started.URL}, http.StatusOK)
}

// returnConnection answers where to send the browser back to an app when the
// holder already has provider linked: the validated return_to query value with
// connection=linked merged in. A provider that is not linked, or a return_to
// that is not a registered client's origin, is a 400 invalid_return_to, so the
// page never navigates to a URL the server did not vouch for.
func (s *Service) returnConnection(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if s.self.Social == nil || !s.self.Social.Configured(provider) {
		notFound(w)
		return
	}
	links, err := s.self.Social.Links(r.Context(), holder(r.Context()).ID)
	if err != nil {
		serverError(w, err)
		return
	}
	target := ""
	for _, link := range links {
		if link.Provider == provider {
			target = s.self.Social.ReturnURL(r.Context(), provider, r.URL.Query().Get("return_to"), "connection", "linked")
		}
	}
	if target == "" {
		encode(w, map[string]string{"error": "invalid_return_to"}, http.StatusBadRequest)
		return
	}
	encode(w, map[string]string{"url": target}, http.StatusOK)
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
