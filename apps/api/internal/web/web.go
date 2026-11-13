package web

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// indexPath is the single document every client route is served from.
const indexPath = "index.html"

// immutablePrefix is the build-hashed asset directory. Every name under it
// carries a content hash, so it can be cached for a year; nothing else here
// can be cached at all.
const immutablePrefix = "_app/immutable/"

// immutableCache is what a content-hashed asset gets.
const immutableCache = "public, max-age=31536000, immutable"

// Handler serves the built client app at the domain root, as the catch-all
// mounted after the api's own routes. Unknown paths are answered with the
// index document so client-side routing owns them.
//
// It returns an error rather than a handler that answers 500, because a build
// this process cannot read is a condition that should refuse the boot. env.go
// states the principle for the configuration it validates: a provider that
// starts and then cannot serve is worse than one that refuses to start, and
// an unreadable index is the same failure one layer down.
//
// The index is written directly rather than handed to http.FileServerFS with
// a rewritten path, which is what this used to do and why no deep link ever
// worked. FileServer canonicalises any request for index.html by redirecting
// to "./", so rewriting /login to /index.html made the server answer 301 to
// the origin root and the route the human asked for was discarded on the way.
// Every page except "/" was unreachable by URL: reload, bookmark, or the
// redirect back from an OIDC flow all landed on the landing page.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("web: the embedded client build is unreadable: %w", err)
	}
	index, err := fs.ReadFile(sub, indexPath)
	if err != nil {
		return nil, fmt.Errorf("web: the embedded client build has no %s: %w", indexPath, err)
	}
	fileServer := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, sub, fileServer, index)
	}), nil
}

// serve answers one request: a real file when the build has one, a JSON 404
// under /api, and the index document for everything else.
func serve(w http.ResponseWriter, r *http.Request, sub fs.FS, files http.Handler, index []byte) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "" && exists(sub, name) {
		w.Header().Set("Cache-Control", cacheFor(name))
		files.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		missingAPI(w)
		return
	}
	writeIndex(w, index)
}

// cacheFor is how long a build file may be held.
//
// Only the hashed directory may be cached, and everything else must not be.
// Without a header the browser applies heuristic caching, which is harmless
// for a favicon and not harmless for _app/version.json: that file is what
// SvelteKit polls to notice a new deploy, so a heuristically cached copy
// leaves a tab convinced it is current forever. Same failure as a cached
// index, one file along.
func cacheFor(name string) string {
	if strings.HasPrefix(name, immutablePrefix) {
		return immutableCache
	}
	return "no-cache"
}

// exists reports whether name is a regular file in the build. A directory is
// not one: serving it would only produce the redirect this handler exists to
// avoid, and the build has no directory a human is meant to reach.
func exists(sub fs.FS, name string) bool {
	info, err := fs.Stat(sub, name)
	return err == nil && !info.IsDir()
}

// writeIndex sends the single-page document.
//
// Framing is refused by the security headers tronc's router puts on every
// response (frame-ancestors 'none' and X-Frame-Options: DENY, enforced): the
// login, consent and device pages each end in one click that grants
// something, and a page that can be framed can be clicked through an
// invisible overlay.
//
// It must not be cached. The document names build-hashed chunk files, so a
// cached copy from a previous deploy asks for chunks that no longer exist and
// the app fails to boot with nothing in the log to explain it.
func writeIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(index)
}

// missingAPI answers an unrouted /api path with JSON rather than the index.
//
// It matches the bare "/api" as well as everything under it, because a caller
// hitting the prefix by mistake is exactly the caller this protects.
//
// The catch-all would otherwise hand every mistyped or removed endpoint a 200
// and a page of HTML, which a caller parsing JSON reads as a malformed success
// rather than a missing route. Journal shipped exactly this and lost reports
// in silence for it; env.go already guards the client side of that story.
func missingAPI(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"not_found"}` + "\n"))
}

// InlineScriptHashes returns a CSP source, 'sha256-...', for every inline
// script in the embedded index document: the bootstrap adapter-static writes
// and the scripts app.html carries. The index is the only document this
// package serves, so these are all the inline scripts a page here runs, and
// script-src can allow them without 'unsafe-inline'.
func InlineScriptHashes() ([]string, error) {
	index, err := fs.ReadFile(dist, "dist/"+indexPath)
	if err != nil {
		return nil, fmt.Errorf("web: the embedded client build has no %s: %w", indexPath, err)
	}
	return hashInlineScripts(string(index)), nil
}

// hashInlineScripts hashes the body of every attribute-less script element,
// the only inline form adapter-static and app.html write. A script with a src
// attribute is skipped: script-src allows it by origin, not by hash.
func hashInlineScripts(page string) []string {
	const open, closing = "<script>", "</script>"
	var hashes []string
	for {
		start := strings.Index(page, open)
		if start < 0 {
			return hashes
		}
		page = page[start+len(open):]
		end := strings.Index(page, closing)
		if end < 0 {
			return hashes
		}
		sum := sha256.Sum256([]byte(page[:end]))
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		page = page[end+len(closing):]
	}
}
