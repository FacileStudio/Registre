package legal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"

	"github.com/go-chi/chi/v5"
)

// TestEveryDocumentExistsInEveryLanguage serves all sixteen texts, each
// marked as a draft with the current version.
func TestEveryDocumentExistsInEveryLanguage(t *testing.T) {
	router := chi.NewRouter()
	Mount(router)
	for document, version := range consent.Versions() {
		for _, locale := range i18n.Locales() {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/legal/"+document+"?lang="+locale, nil))
			var got Document
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
				t.Fatalf("%s/%s = %d", document, locale, rec.Code)
			}
			if got.Version != version || got.Locale != locale || got.Title == "" || !got.Draft || !strings.Contains(got.HTML, "<h2>") {
				t.Errorf("%s/%s = %+v", document, locale, got)
			}
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/legal/unknown", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("an unknown document = %d, want 404", rec.Code)
	}
}

// TestRenderEscapesEverything allows only its own tags and safe links.
func TestRenderEscapesEverything(t *testing.T) {
	title, body := Render("# T\n\n<script>x</script> **b** [ok](/legal/cgu) [bad](javascript:alert(1))\n\n- item")
	if title != "T" || strings.Contains(body, "<script>") || strings.Contains(body, `href="javascript`) {
		t.Fatalf("Render = %q, %q", title, body)
	}
	if !strings.Contains(body, `<a href="/legal/cgu">ok</a>`) || !strings.Contains(body, "<strong>b</strong>") ||
		!strings.Contains(body, "<li>item</li>") {
		t.Fatalf("Render dropped markup: %q", body)
	}
}
