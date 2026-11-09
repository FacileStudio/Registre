package social

import (
	"strings"
)

// chooseEmail selects the primary verified email or the first verified email.
func chooseEmail(list []gitHubEmail) (string, error) {
	var fallback string
	for _, item := range list {
		clean := strings.ToLower(strings.TrimSpace(item.Email))
		if !item.Verified || clean == "" {
			continue
		}
		if item.Primary {
			return clean, nil
		}
		if fallback == "" {
			fallback = clean
		}
	}
	if fallback == "" {
		return "", ErrUnverifiedEmail
	}
	return fallback, nil
}
