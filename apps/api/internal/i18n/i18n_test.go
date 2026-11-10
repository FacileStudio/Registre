package i18n

import (
	"strings"
	"testing"
	"time"
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

// TestTypedTextCannotBreakOutOfAMail feeds a space name and a display name
// built to inject markup, a header line and reversed text, and expects
// escaped HTML, a one-line subject and a readable plain-text part.
func TestTypedTextCannotBreakOutOfAMail(t *testing.T) {
	vars := map[string]string{
		"inviter": "Eve\r\nBcc: victim@example.com <img src=x onerror=alert(1)>",
		"space":   "\"><script>alert(1)</script>‮gnp.exe",
		"role":    RoleName("admin", "de"),
		"date":    Date(time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC), "de"),
	}
	subject, body := Mail(MailSpaceInvite, "de", "https://sso.example.com/invitations/abc", vars)
	if strings.ContainsAny(subject, "\r\n‮") {
		t.Fatalf("the subject can carry a header line or an override: %q", subject)
	}
	for _, raw := range []string{"<img", "<script>", "‮"} {
		if strings.Contains(body, raw) {
			t.Fatalf("the body carries raw %q: %s", raw, body)
		}
	}
	if !strings.Contains(body, "als Admin") || !strings.Contains(body, "24. November 2026") {
		t.Fatalf("the role or the date is missing: %s", body)
	}
	text := PlainText(body)
	if !strings.Contains(text, "Einladung ansehen: https://sso.example.com/invitations/abc") ||
		!strings.Contains(text, `„"><script>alert(1)</script>gnp.exe“`) || strings.Contains(text, "&") {
		t.Fatalf("the plain-text part = %q", text)
	}
}

// TestCleanCapsLengthAndFoldsSpace.
func TestCleanCapsLengthAndFoldsSpace(t *testing.T) {
	long := strings.Repeat("é", MaxVarLength+10)
	if got := []rune(Clean(long)); len(got) != MaxVarLength || got[len(got)-1] != '…' {
		t.Fatalf("Clean(long) has %d runes", len(got))
	}
	if got := Clean("  a \t\n b  "); got != "a b" {
		t.Fatalf("Clean folded to %q", got)
	}
}

// TestDatesAndRolesInEveryLanguage.
func TestDatesAndRolesInEveryLanguage(t *testing.T) {
	day := time.Date(2026, 3, 5, 23, 0, 0, 0, time.UTC)
	want := map[string]string{"en": "5 March 2026", "fr": "5 mars 2026", "es": "5 de marzo de 2026", "de": "5. März 2026"}
	for _, locale := range Locales() {
		if got := Date(day, locale); got != want[locale] {
			t.Errorf("Date in %s = %q, want %q", locale, got, want[locale])
		}
		for _, role := range []string{"owner", "admin", "member"} {
			if RoleName(role, locale) == "" {
				t.Errorf("role %s has no %s name", role, locale)
			}
		}
	}
}
