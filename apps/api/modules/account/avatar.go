package account

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
)

// putAvatar replaces the holder's picture with the request body, sent as the
// raw image bytes.
//
// The body is capped before it is read, so an oversized upload costs the
// ceiling and not its own size. A refusal from avatars.Normalize is the
// sender's own file described back to them, so its message is passed on.
func (s *Service) putAvatar(w http.ResponseWriter, r *http.Request) {
	upload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, avatars.MaxUploadBytes))
	if err != nil {
		encode(w, map[string]string{"error": "the picture must be at most 4 MiB"}, http.StatusRequestEntityTooLarge)
		return
	}
	user := holder(r.Context())
	picture, err := s.avatars.Put(r.Context(), user.ID, upload)
	if errors.Is(err, avatars.ErrInvalid) {
		encode(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an account changed its picture", slog.String("user_id", user.ID))
	encode(w, map[string]string{"picture": picture}, http.StatusOK)
}

// deleteAvatar removes the holder's picture. Apps fall back to their own
// default the next time the holder signs in to them.
func (s *Service) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	user := holder(r.Context())
	if err := s.avatars.Remove(r.Context(), user.ID); err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an account removed its picture", slog.String("user_id", user.ID))
	encode(w, map[string]string{"picture": ""}, http.StatusOK)
}
