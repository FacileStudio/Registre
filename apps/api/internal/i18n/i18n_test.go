package i18n

import (
	"strings"
	"testing"
)

// TestEveryMailExistsInEveryLanguage fails the build when a kind lacks a
// locale.
func TestEveryMailExistsInEveryLanguage(t *testing.T) {
	for kind, byLocale := range mailCatalog() {
		for _, locale := range Locales() {
			if text, ok := byLocale[locale]; !ok || text.subject == "" || len(text.paragraphs) == 0 {
				t.Errorf("mail %s has no %s text", kind, locale)
			}
		}
	}
}

// TestVariablesAreEscaped keeps a substituted value or a link from becoming
// markup.
func TestVariablesAreEscaped(t *testing.T) {
	_, body := Mail(MailDeletionScheduled, "fr", `https://sso.example.com/"><script>`,
		map[string]string{"date": "<b>2026</b>"})
	if strings.Contains(body, "<b>") || strings.Contains(body, "<script>") {
		t.Fatalf("unescaped markup in %s", body)
	}
}

// TestAcceptLanguagePicksTheBestSupportedLanguage.
func TestAcceptLanguagePicksTheBestSupportedLanguage(t *testing.T) {
	cases := map[string]string{
		"":                           "en",
		"it-IT,it;q=0.9":             "en",
		"fr-FR,fr;q=0.9,en;q=0.8":    "fr",
		"it;q=1, de;q=0.5, es;q=0.7": "es",
		"de-CH":                      "de",
		"en;q=0, fr;q=0":             "en",
	}
	for header, want := range cases {
		if got := FromAcceptLanguage(header); got != want {
			t.Errorf("FromAcceptLanguage(%q) = %q, want %q", header, got, want)
		}
	}
	if Normalize("ES-mx") != "es" || Normalize("pt") != "en" {
		t.Fatal("Normalize does not reduce to the supported set")
	}
}
