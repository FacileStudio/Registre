package admin

import (
	"encoding/json"
	"errors"
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
	CreatedAt  string   `json:"created_at"`
	DisabledAt *string  `json:"disabled_at,omitempty"`
}

func viewClient(row schemas.Client) clientView {
	view := clientView{
		ClientID: row.ClientID, Subject: row.Subject, Name: row.Name, Kind: row.Kind,
		GrantTypes: row.GrantTypes, Scopes: row.Scopes, Audiences: row.Audiences,
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
	var body struct {
		ClientID   string   `json:"client_id"`
		Name       string   `json:"name"`
		Kind       string   `json:"kind"`
		GrantTypes []string `json:"grant_types"`
		Scopes     []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if body.ClientID == "" || body.Name == "" ||
		(body.Kind != schemas.KindServiceAccount && body.Kind != schemas.KindApplication) {
		encode(w, map[string]string{
			"error": "client_id, name and kind (service_account|application) are required",
		}, http.StatusBadRequest)
		return
	}
	declared := schemas.Client{
		ClientID: body.ClientID, Name: body.Name, Kind: body.Kind,
		GrantTypes: schemas.StringList(body.GrantTypes),
		Scopes:     schemas.StringList(body.Scopes),
	}
	row, secret, err := s.register.Create(r.Context(), declared)
	if err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: admin minted a client",
		slog.String("client_id", row.ClientID), slog.String("kind", row.Kind))
	encode(w, map[string]any{"client": viewClient(row), "secret": secret}, http.StatusCreated)
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
	s.logger.Info("registre: admin rotated a client secret", slog.String("client_id", clientID))
	encode(w, map[string]string{"client_id": clientID, "secret": secret}, http.StatusOK)
}

func badRequest(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "bad body"}, http.StatusBadRequest)
}

func serverError(w http.ResponseWriter, err error) {
	slog.Error("registre: admin request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal"}, http.StatusInternalServerError)
}
