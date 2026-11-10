package i18n

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// fill replaces every {key} in text with its value.
func fill(text string, vars map[string]string) string {
	for key, value := range vars {
		text = strings.ReplaceAll(text, "{"+key+"}", value)
	}
	return text
}

// MaxVarLength caps a substituted value, in characters. A space name or a
// display name longer than this is cut and ends with an ellipsis.
const MaxVarLength = 80

// Clean makes text someone typed safe to put in a mail line: it drops
// control characters (CR and LF included, so nothing can start a header)
// and the bidirectional overrides that make text read backwards, folds
// runs of space into one, and caps the length at MaxVarLength.
func Clean(text string) string {
	var out []rune
	for _, r := range text {
		switch {
		case unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r):
			continue
		case unicode.IsSpace(r):
			r = ' '
		}
		out = append(out, r)
	}
	folded := strings.Join(strings.Fields(string(out)), " ")
	runes := []rune(folded)
	if len(runes) > MaxVarLength {
		return strings.TrimSpace(string(runes[:MaxVarLength-1])) + "…"
	}
	return folded
}

var (
	mailLink = regexp.MustCompile(`<a href="([^"]*)"[^>]*>([^<]*)</a>`)
	mailTag  = regexp.MustCompile(`<[^>]*>`)
)

// PlainText is the text/plain part of a body Mail rendered: one paragraph
// per line pair, the link spelled out after its label, entities decoded.
// The transport's own fallback strips tags only, which loses the link and
// leaves entities such as &#39; in the text.
func PlainText(body string) string {
	text := mailLink.ReplaceAllString(body, "$2: $1")
	text = strings.ReplaceAll(text, "</p>", "\n\n")
	text = mailTag.ReplaceAllString(text, "")
	return strings.TrimSpace(html.UnescapeString(text))
}
