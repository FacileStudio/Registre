package env

import (
	"fmt"

	troncenv "github.com/FacileStudio/tronc/env"
)

// Social is the social sign-in credentials. Both Discord values empty turns
// the provider off everywhere; one without the other fails the boot, because a
// half-configured provider shows a button that can never work.
type Social struct {
	DiscordClientID     string
	DiscordClientSecret string
}

// DiscordEnabled reports whether the Discord provider has credentials.
func (s Social) DiscordEnabled() bool {
	return s.DiscordClientID != "" && s.DiscordClientSecret != ""
}

// LoadSocial reads DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET.
func LoadSocial() (Social, error) {
	out := Social{
		DiscordClientID:     troncenv.String("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: troncenv.String("DISCORD_CLIENT_SECRET", ""),
	}
	if (out.DiscordClientID == "") != (out.DiscordClientSecret == "") {
		return Social{}, fmt.Errorf(
			"env: DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET must be set together: " +
				"an id without a secret cannot finish a sign-in and a secret without an id cannot start one")
	}
	return out, nil
}
