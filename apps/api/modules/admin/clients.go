package admin

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

// timeFormat is RFC 3339 with a UTC offset, which is what every timestamp
// this API ships uses.
const timeFormat = time.RFC3339

// clientView is what the dashboard sees of a client row: everything but the
// secret hash, which is a credential even in its sealed form.
type clientView struct {
	ClientID   string   `json:"client_id"`
	Subject    string   `json:"subject"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	GrantTypes []string `json:"grant_types,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	Audiences  []string `json:"audiences,omitempty"`

	// RedirectURIs, VisibleRoles and BackchannelLogoutURI are shown because
	// they are the three settings whose absence breaks an app silently:
	// without the first it cannot finish a login, without the second its
	// authorization checks all deny, and without the third it never learns
	// a session was terminated. A dashboard that hides them is a dashboard
	// nobody can diagnose from.
	RedirectURIs         []string `json:"redirect_uris,omitempty"`
	VisibleRoles         []string `json:"visible_roles,omitempty"`
	BackchannelLogoutURI string   `json:"backchannel_logout_uri,omitempty"`
	DeletionURI          string   `json:"deletion_uri,omitempty"`

	AccessPolicy string `json:"access_policy"`

	Managed    bool    `json:"managed"`
	CreatedAt  string  `json:"created_at"`
	DisabledAt *string `json:"disabled_at,omitempty"`
}

func viewClient(row schemas.Client) clientView {
	view := clientView{
		ClientID: row.ClientID, Subject: row.Subject, Name: row.Name, Kind: row.Kind,
		GrantTypes: row.GrantTypes, Scopes: row.Scopes, Audiences: row.Audiences,
		RedirectURIs: row.RedirectURIs, VisibleRoles: row.VisibleRoles,
		BackchannelLogoutURI: row.BackchannelLogoutURI, DeletionURI: row.DeletionURI,
		Managed: row.Managed, AccessPolicy: row.AccessPolicy,
		CreatedAt: row.CreatedAt.UTC().Format(timeFormat),
	}
	if row.DisabledAt != nil {
		at := row.DisabledAt.UTC().Format(timeFormat)
		view.DisabledAt = &at
	}
	return view
}

func (s *Service) listClients(w http.ResponseWriter, r *http.Request) {
	rows, err := s.register.List(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	views := make([]clientView, 0, len(rows))
	for _, row := range rows {
		views = append(views, viewClient(row))
	}
	encode(w, views, http.StatusOK)
}

// createClient mints a service account or an application and answers with
// its secret exactly once. The response is the only copy; losing it means
// rotating.
func (s *Service) createClient(w http.ResponseWriter, r *http.Request) {
	var body newClient
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if reason, ok := body.invalid(); !ok {
		encode(w, map[string]string{"error": reason}, http.StatusBadRequest)
		return
	}
	row, secret, err := s.register.Create(r.Context(), body.declaration())
	if err != nil {
		serverError(w, err)
		return
	}
	s.record(r, "client.create", clientTarget(row.ClientID), map[string]any{"name": row.Name, "kind": row.Kind})
	s.logger.Info("registre: admin minted a client",
		slog.String("client_id", row.ClientID), slog.String("kind", row.Kind))
	encode(w, map[string]any{"client": viewClient(row), "secret": secret}, http.StatusCreated)
}

func (s *Service) getClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	row, err := s.register.ByClientID(r.Context(), clientID)
	if errors.Is(err, clients.ErrUnknownClient) {
		encode(w, map[string]string{"error": "unknown client"}, http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, viewClient(row), http.StatusOK)
}

func (s *Service) updateClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	existing, err := s.register.ByClientID(r.Context(), clientID)
	if errors.Is(err, clients.ErrUnknownClient) {
		encode(w, map[string]string{"error": "unknown client"}, http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxClientBody))
	if err != nil {
		badRequest(w)
		return
	}
	edit, err := readEdit(existing, raw)
	if err != nil {
		badRequest(w)
		return
	}
	if reason, ok := edit.valid(existing); !ok {
		encode(w, map[string]string{"error": reason}, http.StatusBadRequest)
		return
	}
	row, err := s.register.UpdateForm(r.Context(), clientID, edit.changes)
	if err != nil {
		serverError(w, err)
		return
	}
	s.record(r, "client.update", clientTarget(clientID), nil)
	s.logger.Info("registre: admin updated client", slog.String("client_id", clientID))
	encode(w, viewClient(row), http.StatusOK)
}

func (s *Service) rotateSecret(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	_, secret, err := s.register.RotateSecret(r.Context(), clientID)
	if errors.Is(err, clients.ErrUnknownClient) {
		encode(w, map[string]string{"error": "unknown client"}, http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	s.record(r, "client.rotate_secret", clientTarget(clientID), nil)
	s.logger.Info("registre: admin rotated a client secret", slog.String("client_id", clientID))
	encode(w, map[string]string{"client_id": clientID, "secret": secret}, http.StatusOK)
}

func (s *Service) disableClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	if err := s.register.Disable(r.Context(), clientID, now()); err != nil {
		if errors.Is(err, clients.ErrUnknownClient) {
			encode(w, map[string]string{"error": "unknown client"}, http.StatusNotFound)
			return
		}
		serverError(w, err)
		return
	}
	s.record(r, "client.disable", clientTarget(clientID), nil)
	s.logger.Info("registre: admin disabled client", slog.String("client_id", clientID))
	encode(w, map[string]any{"disabled": clientID}, http.StatusOK)
}

func (s *Service) enableClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	if err := s.register.Enable(r.Context(), clientID); err != nil {
		if errors.Is(err, clients.ErrUnknownClient) {
			encode(w, map[string]string{"error": "unknown client"}, http.StatusNotFound)
			return
		}
		serverError(w, err)
		return
	}
	s.record(r, "client.enable", clientTarget(clientID), nil)
	s.logger.Info("registre: admin enabled client", slog.String("client_id", clientID))
	encode(w, map[string]any{"enabled": clientID}, http.StatusOK)
}

func (s *Service) deleteClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	if err := s.register.Delete(r.Context(), clientID); err != nil {
		encode(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return
	}
	s.record(r, "client.delete", clientTarget(clientID), nil)
	s.logger.Info("registre: admin deleted client", slog.String("client_id", clientID))
	encode(w, map[string]any{"deleted": clientID}, http.StatusOK)
}

func badRequest(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "bad body"}, http.StatusBadRequest)
}

func serverError(w http.ResponseWriter, err error) {
	slog.Error("registre: admin request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal"}, http.StatusInternalServerError)
}
