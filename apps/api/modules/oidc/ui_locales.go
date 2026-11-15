package oidc

import (
	"net/http"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
)

// uiLocalesParam is the OpenID Connect parameter an app sends to say which
// languages the person reads, best first.
const uiLocalesParam = "ui_locales"

// uiLocale answers the first language in a ui_locales value that Registre
// speaks, or the empty string. Only one of i18n.Locales ever comes back, so
// the result is safe to put in a URL.
func uiLocale(value string) string {
	for _, tag := range strings.Fields(value) {
		language, _, _ := strings.Cut(strings.ToLower(tag), "-")
		if i18n.Supported(language) {
			return language
		}
	}
	return ""
}

// localeHint adds ui_locales to the redirect that sends a browser to the
// login page, so the page opens in the language the app asked for. It is a
// display hint only: nothing is stored on the auth request or the account.
type localeHint struct {
	http.ResponseWriter
	locale string
}

// WriteHeader appends the hint when the response is the login redirect.
func (h localeHint) WriteHeader(status int) {
	if location := h.Header().Get("Location"); strings.HasPrefix(location, loginPath) {
		h.Header().Set("Location", location+"&"+uiLocalesParam+"="+h.locale)
	}
	h.ResponseWriter.WriteHeader(status)
}

// hinted wraps w so the login redirect carries the ui_locales r asked for.
// A request that names no supported language gets w back untouched.
func hinted(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	locale := uiLocale(r.Form.Get(uiLocalesParam))
	if locale == "" {
		return w
	}
	return localeHint{ResponseWriter: w, locale: locale}
}
