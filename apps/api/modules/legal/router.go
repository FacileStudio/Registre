// Package legal serves Registre's legal documents: the CGU (terms of use),
// the CGV (terms of sale), the privacy policy and the legal notice, each in
// fr, en, es and de, at stable URLs every Facile app links to.
//
// The texts are markdown files embedded in the binary, one per document and
// language, and the version of each lives in internal/consent. The client
// renders /legal/{document}?lang=xx from GET /api/legal/{document}?lang=xx.
// The texts are drafts pending legal review and say so at the top; the
// placeholders a lawyer must fill are listed in LEGAL_TODO.md.
package legal

import (
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"

	"github.com/go-chi/chi/v5"
)

//go:embed content/*.md
var content embed.FS

// Draft marks every text as pending legal review. Flip it in the commit that
// lands the reviewed texts.
const Draft = true

// Document is one legal text in one language.
type Document struct {
	Document string `json:"document"`
	Version  string `json:"version"`
	Locale   string `json:"locale"`
	Title    string `json:"title"`
	HTML     string `json:"html"`
	Draft    bool   `json:"draft"`
	Required bool   `json:"required"`
}

// Mount registers the legal routes.
func Mount(router chi.Router) {
	router.Get("/api/legal", index)
	router.Get("/api/legal/{document}", serve)
}

// index lists every document with its current version and whether an
// account must accept it.
func index(w http.ResponseWriter, _ *http.Request) {
	required := map[string]bool{}
	for _, name := range consent.Required() {
		required[name] = true
	}
	list := []Document{}
	for _, name := range []string{consent.CGU, consent.CGV, consent.Privacy, consent.LegalNotice} {
		list = append(list, Document{
			Document: name, Version: consent.Versions()[name], Draft: Draft, Required: required[name],
		})
	}
	encode(w, map[string]any{"documents": list, "locales": i18n.Locales()}, http.StatusOK)
}

// serve answers one document in the asked language, English when the
// language is unknown, 404 for an unknown document.
func serve(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "document")
	version, known := consent.Versions()[name]
	if !known {
		encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return
	}
	locale := r.URL.Query().Get("lang")
	if !i18n.Supported(locale) {
		locale = i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	}
	raw, err := content.ReadFile("content/" + name + "." + locale + ".md")
	if err != nil {
		encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return
	}
	title, body := Render(string(raw))
	required := false
	for _, owed := range consent.Required() {
		required = required || owed == name
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	encode(w, Document{
		Document: name, Version: version, Locale: locale, Title: title, HTML: body,
		Draft: Draft, Required: required,
	}, http.StatusOK)
}

// encode writes one JSON object with the given status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: legal response write failed", slog.Any("error", err))
	}
}

// trimTitle strips the heading marker from the first line.
func trimTitle(line string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, "# "))
}
