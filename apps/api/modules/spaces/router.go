// Package spaces is the API behind registre's spaces page: the one place in
// the suite where a human creates a team, invites mates and sets their rank.
// Every app shows the result; none edits it.
//
// Every route needs the account cookie, and every write the CSRF header, by
// the same guard as /api/account. A space the caller is not in answers 404,
// so membership is not something a stranger can probe.
package spaces

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/directory"

	"github.com/go-chi/chi/v5"
)

// Service serves the spaces page.
type Service struct {
	spaces   *directory.Service
	sessions *accounts.Service
	cookies  accounts.Cookies
	logger   *slog.Logger
}

// New builds the module.
func New(spaces *directory.Service, sessions *accounts.Service, cookies accounts.Cookies, logger *slog.Logger) *Service {
	return &Service{spaces: spaces, sessions: sessions, cookies: cookies, logger: logger}
}

// Mount registers the spaces routes behind the account guard.
func Mount(router chi.Router, s *Service) {
	guard := accounts.NewGuard(s.sessions, s.cookies)
	router.Route("/api/spaces", func(r chi.Router) {
		r.Use(guard.CSRF, guard.Session)
		r.Get("/", s.list)
		r.Post("/", s.create)
		r.Get("/{spaceID}", s.get)
		r.Post("/{spaceID}/rename", s.rename)
		r.Post("/{spaceID}/members", s.addMember)
		r.Post("/{spaceID}/members/{userID}/role", s.setRole)
		r.Delete("/{spaceID}/members/{userID}", s.removeMember)
		r.Post("/{spaceID}/leave", s.leave)
		r.Delete("/{spaceID}", s.deleteSpace)
	})
}

// holderID is the signed-in human's subject.
func holderID(r *http.Request) string {
	user, _ := accounts.Holder(r.Context())
	return user.ID
}

// decode reads a small JSON body, answering 400 itself when it cannot.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(into); err != nil {
		encode(w, map[string]string{"error": "malformed request"}, http.StatusBadRequest)
		return false
	}
	return true
}

// answer maps a directory error to its status, or writes body on success.
func (s *Service) answer(w http.ResponseWriter, err error, body any) {
	status := http.StatusOK
	switch {
	case err == nil:
	case errors.Is(err, directory.ErrNotFound), errors.Is(err, directory.ErrNoSuchUser):
		status = http.StatusNotFound
	case errors.Is(err, directory.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, directory.ErrNameTaken), errors.Is(err, directory.ErrLastOwner),
		errors.Is(err, directory.ErrAlreadyMember):
		status = http.StatusConflict
	case errors.Is(err, directory.ErrInvalidRole), errors.Is(err, directory.ErrInvalidName):
		status = http.StatusBadRequest
	default:
		s.logger.Error("registre: spaces request failed", slog.Any("error", err))
		encode(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
		return
	}
	if err != nil {
		body = map[string]string{"error": err.Error()}
	}
	encode(w, body, status)
}

// encode writes body as JSON with status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
