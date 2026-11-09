package main

import (
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/social"

	"gorm.io/gorm"
)

// buildSocial builds the social sign-in service over the providers that have
// credentials. With none configured it is still returned, so the callers hold
// one value and every list it answers is empty.
func buildSocial(db *gorm.DB, cfg env.Config, signupOpen bool, log *slog.Logger) (*social.Service, error) {
	settings, err := env.LoadSocial()
	if err != nil {
		return nil, err
	}
	var providers []social.Provider
	if settings.DiscordEnabled() {
		providers = append(providers, social.NewDiscord(settings.DiscordClientID, settings.DiscordClientSecret))
		log.Info("registre: Discord sign-in is configured")
	}
	if settings.GitHubEnabled() {
		providers = append(providers, social.NewGitHub(settings.GitHubClientID, settings.GitHubClientSecret))
		log.Info("registre: GitHub sign-in is configured")
	}
	return social.New(db, providers, social.Config{
		Issuer: cfg.Issuer, Secure: !cfg.InsecureIssuer(), SignupOpen: signupOpen,
	}), nil
}
