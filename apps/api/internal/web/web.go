package web

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the built client app under /login. Every asset URL the
// build emits already carries that prefix, so one subtree mount covers it.
// Unknown paths fall back to index.html because the app is a single page.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "embedded client build is unreadable", http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/login")
		if name == "" || name == "/" {
			name = "/index.html"
		}
		if _, err := fs.Stat(sub, strings.TrimPrefix(name, "/")); err != nil {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}
		http.StripPrefix("/login", fileServer).ServeHTTP(w, r)
	})
}
