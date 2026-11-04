// Package directory serves the suite's shared spaces to the apps.
//
// Every route authenticates the calling app with HTTP Basic, its own OIDC
// client id and secret, verified under the same argon2 ceiling as the token
// endpoint. Only application clients may call: a service account has no
// users and no spaces to keep in step.
package directory

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	spaces "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/go-chi/chi/v5"
)

// maxImportBytes bounds an import body. A suite's spaces fit in kilobytes.
const maxImportBytes = 1 << 20

// Service serves the snapshot, the one-time import and the entitlements
// push.
type Service struct {
	register *clients.Service
	spaces   *spaces.Service
	logger   *slog.Logger
	writer   string

	deletionsFor DeletionLister
}

// New builds the module over the register and the spaces service.
func New(register *clients.Service, spaces *spaces.Service, logger *slog.Logger) *Service {
	return &Service{register: register, spaces: spaces, logger: logger}
}

// Mount registers the app-facing routes.
func Mount(router chi.Router, s *Service) {
	router.Get("/api/directory/snapshot", s.snapshot)
	router.Post("/api/directory/import", s.importSpaces)
	router.Put("/api/directory/entitlements", s.setEntitlements)
	router.Get("/api/directory/deletions", s.deletions)
}

// snapshot answers every shared space with its members.
func (s *Service) snapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.app(w, r); !ok {
		return
	}
	out, err := s.spaces.Snapshot(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	encode(w, out, http.StatusOK)
}

// importSpaces merges the calling app's pre-existing spaces, once.
func (s *Service) importSpaces(w http.ResponseWriter, r *http.Request) {
	app, ok := s.app(w, r)
	if !ok {
		return
	}
	var body struct {
		Spaces []wire.ImportSpace `json:"spaces"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxImportBytes)).Decode(&body); err != nil {
		encode(w, map[string]string{"error": "malformed import"}, http.StatusBadRequest)
		return
	}
	out, err := s.spaces.Import(r.Context(), app.ClientID, body.Spaces)
	if err != nil {
		s.fail(w, err)
		return
	}
	encode(w, out, http.StatusOK)
}

// app authenticates the caller as an enabled application client, answering
// 401 itself when it is not one.
func (s *Service) app(w http.ResponseWriter, r *http.Request) (schemas.Client, bool) {
	id, secret, ok := r.BasicAuth()
	if ok {
		row, err := s.register.Authenticate(r.Context(), id, secret)
		if err == nil && row.Kind == schemas.KindApplication && row.DisabledAt == nil {
			return row, true
		}
		if err != nil && !errors.Is(err, clients.ErrUnknownClient) {
			s.fail(w, err)
			return schemas.Client{}, false
		}
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="registre directory"`)
	encode(w, map[string]string{"error": "unauthorized"}, http.StatusUnauthorized)
	return schemas.Client{}, false
}

// fail logs the cause and tells the caller nothing about it.
func (s *Service) fail(w http.ResponseWriter, err error) {
	s.logger.Error("registre: directory request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
}

// encode writes body as JSON with status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
