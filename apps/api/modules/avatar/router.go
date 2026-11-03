// Package avatar serves profile pictures to anyone holding their URL.
//
// The route is public on purpose: porte fetches a `picture` claim without
// credentials, and an app showing a colleague's face has no token for
// registre. The key is the human's subject, which every app that shows the
// picture already holds, and which is random rather than enumerable.
//
// The URL is stable while the picture changes, so the response must never be
// served from a cache without asking: no-cache with the upload id as ETag
// makes every load a revalidation, answered 304 while the picture is the same
// and with the new bytes the moment it is not. That is the whole of how an
// upload reaches every app instantly.
//
// The bytes were encoded by registre, so the response can promise exactly
// what it is: nosniff and a CSP of nothing stop a browser treating it as
// anything but an image.
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
	router.Get(avatars.Path+"{key}", serve(service))
}

// serve answers the picture a key names, 304 when the caller already holds
// it, or 404 when there is none. The 404 is cacheable for thirty seconds:
// every human's URL is in the claim, so a page listing fifty people with no
// picture would otherwise ask fifty times on every render, and a first upload
// still shows within half a minute.
func serve(service *avatars.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		row, err := service.Lookup(r.Context(), chi.URLParam(r, "key"))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.Header().Set("Cache-Control", "public, max-age=30")
			http.NotFound(w, r)
			return
		}
		if err != nil {
			slog.Error("registre: reading an avatar failed", slog.Any("error", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		header := w.Header()
		header.Set("ETag", `"`+row.ID+`"`)
		header.Set("Cache-Control", "no-cache")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Content-Security-Policy", "default-src 'none'")
		header.Set("Cross-Origin-Resource-Policy", "cross-origin")
		if r.Header.Get("If-None-Match") == `"`+row.ID+`"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		header.Set("Content-Type", "image/png")
		_, _ = w.Write(row.Data)
	}
}
