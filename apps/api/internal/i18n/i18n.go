// Package i18n holds the four languages Registre speaks and the mails it
// sends in each of them.
//
// The locale set is closed on purpose: fr, en, es, de. A person's locale is
// stored on their row, chosen at sign-up from Accept-Language, editable on
// /account, and emitted to apps as the OIDC locale claim. Anything outside
// the set falls back to English rather than failing.
package i18n

import (
	"sort"
	"strconv"
	"strings"
)

// Fallback is the locale used when nothing better is known.
const Fallback = "en"

// Locales is the closed set, in the order pages list them.
func Locales() []string { return []string{"fr", "en", "es", "de"} }

// Supported reports whether locale is one of the four.
func Supported(locale string) bool {
	switch locale {
	case "fr", "en", "es", "de":
		return true
	}
	return false
}

// Normalize lowercases a tag such as "fr-CA" to its language and answers it
// when supported, else Fallback.
func Normalize(tag string) string {
	language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	if Supported(language) {
		return language
	}
	return Fallback
}

// preference is one Accept-Language entry.
type preference struct {
	language string
	weight   float64
}

// FromAcceptLanguage picks the supported language a browser ranks highest,
// or Fallback when it names none of them.
func FromAcceptLanguage(header string) string {
	var ranked []preference
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		language, _, _ := strings.Cut(strings.ToLower(tag), "-")
		if !Supported(language) {
			continue
		}
		ranked = append(ranked, preference{language: language, weight: weightOf(params)})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].weight > ranked[j].weight })
	if len(ranked) == 0 || ranked[0].weight <= 0 {
		return Fallback
	}
	return ranked[0].language
}

// weightOf reads the q parameter, defaulting to 1.
func weightOf(params string) float64 {
	value, ok := strings.CutPrefix(strings.TrimSpace(params), "q=")
	if !ok {
		return 1
	}
	weight, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return weight
}
