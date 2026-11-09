package env

import (
	"fmt"

	troncenv "github.com/FacileStudio/tronc/env"
)

// Social holds the credentials for external identity providers. For each
// provider, leaving both fields empty disables it; setting only one fails the
// boot because a half-configured provider shows a button that cannot work.
type Social struct {
	DiscordClientID     string
	DiscordClientSecret string
	GitHubClientID      string
	GitHubClientSecret  string
}

// DiscordEnabled reports whether the Discord provider has credentials.
func (s Social) DiscordEnabled() bool {
	return s.DiscordClientID != "" && s.DiscordClientSecret != ""
}

// GitHubEnabled reports whether the GitHub provider has credentials.
func (s Social) GitHubEnabled() bool {
	return s.GitHubClientID != "" && s.GitHubClientSecret != ""
}

// LoadSocial reads credentials for every supported social provider.
func LoadSocial() (Social, error) {
	out := Social{
		DiscordClientID:     troncenv.String("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: troncenv.String("DISCORD_CLIENT_SECRET", ""),
		GitHubClientID:      troncenv.String("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  troncenv.String("GITHUB_CLIENT_SECRET", ""),
	}
	if (out.DiscordClientID == "") != (out.DiscordClientSecret == "") {
		return Social{}, fmt.Errorf(
			"env: DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET must be set together: " +
				"an id without a secret cannot finish a sign-in and a secret without an id cannot start one")
	}
	if (out.GitHubClientID == "") != (out.GitHubClientSecret == "") {
		return Social{}, fmt.Errorf(
			"env: GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET must be set together: " +
				"an id without a secret cannot finish a sign-in and a secret without an id cannot start one")
	}
	return out, nil
}
