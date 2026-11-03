// Package avatar serves profile pictures to anyone holding their URL.
//
// The route is public on purpose: porte fetches a `picture` claim without
// credentials, and an app showing a colleague's face has no token for
// registre. What keeps it from being an enumeration surface is the id, which
// is random, unrelated to the subject and replaced on every upload.
//
// The bytes were encoded by registre, so the response can promise exactly
// what it is. nosniff and a CSP of nothing stop a browser treating it as
// anything but an image, and immutable is honest because a changed picture is
// a different URL.
package avatar

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/avatars"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// Mount registers the picture route on router.
func Mount(router chi.Router, service *avatars.Service) {
	router.Get(avatars.Path+"{avatarID}", serve(service))
}

// serve answers one stored picture, or 404 for an id that names nothing.
func serve(service *avatars.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		row, err := service.ByID(r.Context(), chi.URLParam(r, "avatarID"))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			slog.Error("registre: reading an avatar failed", slog.Any("error", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		header := w.Header()
		header.Set("Content-Type", "image/png")
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Content-Security-Policy", "default-src 'none'")
		header.Set("Cross-Origin-Resource-Policy", "cross-origin")
		_, _ = w.Write(row.Data)
	}
}
