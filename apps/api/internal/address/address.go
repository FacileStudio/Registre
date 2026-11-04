// Package address shape-checks an email address typed into a public form.
// It decides from the string alone, so refusing an address never says
// anything about who has an account.
package address

import "strings"

// maxLength is RFC 5321's path limit.
const maxLength = 254

// Clean lowercases and trims raw and reports whether it is shaped like an
// address: one @, a dotted domain, no spaces or characters that need
// quoting.
func Clean(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || !strings.Contains(domain, ".") || len(email) > maxLength {
		return "", false
	}
	if strings.ContainsAny(email, " \t\r\n<>\"(),;:\\[]") || strings.Contains(domain, "@") {
		return "", false
	}
	return email, true
}
