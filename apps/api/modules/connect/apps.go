package connect

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/grants"
	"github.com/go-chi/chi/v5"
)

// appView is one connected app as the account page lists it.
type appView struct {
	ID           string     `json:"id"`
	ClientID     string     `json:"client_id"`
	ClientName   string     `json:"client_name"`
	ClientURI    string     `json:"client_uri,omitempty"`
	RedirectHost string     `json:"redirect_host"`
	Resource     string     `json:"resource"`
	AppName      string     `json:"app_name"`
	Scopes       []string   `json:"scopes"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// apps answers GET /api/account/apps: the person's live grants.
func (s *Service) apps(w http.ResponseWriter, r *http.Request) {
	user, _ := accounts.Holder(r.Context())
	rows, err := s.grants.ForUser(r.Context(), user.ID)
	if err != nil {
		s.refuse(w, err)
		return
	}
	views := make([]appView, 0, len(rows))
	for _, row := range rows {
		view := appView{
			ID: row.ID, ClientID: row.ClientID, ClientName: row.ClientID, Resource: row.Resource,
			Scopes: row.Scopes, CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt,
		}
		if client, found, err := s.register.Stored(r.Context(), row.ClientID); err == nil && found {
			view.ClientName, view.ClientURI = client.Name, client.ClientURI
			view.RedirectHost = firstHost(client.RedirectURIs)
		}
		if owner, err := s.register.ByResource(r.Context(), row.Resource); err == nil {
			view.AppName = owner.Name
		}
		views = append(views, view)
	}
	encode(w, views, http.StatusOK)
}

// revoke answers DELETE /api/account/apps/{grantID}.
func (s *Service) revoke(w http.ResponseWriter, r *http.Request) {
	user, _ := accounts.Holder(r.Context())
	if err := s.grants.Revoke(r.Context(), user.ID, chi.URLParam(r, "grantID"), s.now()); err != nil {
		s.refuse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// firstHost is the host of the first redirect URI, for display.
func firstHost(uris []string) string {
	for _, raw := range uris {
		if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	return ""
}

// refuse answers a failure: 404 for anything the caller may not see, 500
// otherwise.
func (s *Service) refuse(w http.ResponseWriter, err error) {
	if errors.Is(err, errGone) || errors.Is(err, grants.ErrUnknown) {
		encode(w, map[string]string{"error": "not_found"}, http.StatusNotFound)
		return
	}
	s.logger.Error("registre: connect failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "server_error"}, http.StatusInternalServerError)
}

// decode reads a small JSON body.
func decode(r *http.Request, into any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4096)).Decode(into)
}

// encode writes one JSON value with the status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: connect response write failed", slog.Any("error", err))
	}
}
