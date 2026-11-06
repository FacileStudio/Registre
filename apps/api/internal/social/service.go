package social

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// Config is what the service needs beyond its providers.
type Config struct {
	// Issuer is Registre's public URL; the redirect URI registered with each
	// provider is derived from it.
	Issuer string

	// Secure marks the state cookie Secure and gives it the __Host- prefix.
	Secure bool

	// SignupOpen lets a sign-in with an unknown email create an account.
	SignupOpen bool
}

// Service resolves provider sign-ins into Registre users.
type Service struct {
	db        *gorm.DB
	providers map[string]Provider
	cfg       Config
	now       func() time.Time
}

// New builds the service over the configured providers. With none it is
// still safe to hold: every list is empty and every start is refused.
func New(db *gorm.DB, providers []Provider, cfg Config) *Service {
	byName := make(map[string]Provider, len(providers))
	for _, provider := range providers {
		byName[provider.Name()] = provider
	}
	cfg.Issuer = strings.TrimSuffix(cfg.Issuer, "/")
	return &Service{db: db, providers: byName, cfg: cfg, now: func() time.Time { return time.Now().UTC() }}
}

// RedirectURI is the callback registered with the provider called name.
func (s *Service) RedirectURI(name string) string {
	return s.cfg.Issuer + "/api/login/social/" + name + "/callback"
}

// Configured reports whether name is a provider with credentials here.
func (s *Service) Configured(name string) bool {
	_, ok := s.providers[name]
	return ok
}

// ForClient lists the configured providers the client opted into, sorted.
// An unknown or disabled client has none.
func (s *Service) ForClient(ctx context.Context, clientID string) ([]string, error) {
	if len(s.providers) == 0 || clientID == "" {
		return []string{}, nil
	}
	var row schemas.Client
	err := s.db.WithContext(ctx).Select("login_providers").
		Where("client_id = ? AND disabled_at IS NULL", clientID).Limit(1).Find(&row).Error
	if err != nil {
		return nil, fmt.Errorf("social: read the client's providers: %w", err)
	}
	return s.configuredIn(row.LoginProviders), nil
}

// Offered lists the configured providers at least one enabled client opted
// into. The account page lists only these, so nothing about a provider shows
// until an app uses it.
func (s *Service) Offered(ctx context.Context) ([]string, error) {
	if len(s.providers) == 0 {
		return []string{}, nil
	}
	var names []string
	err := s.db.WithContext(ctx).Raw(
		`SELECT DISTINCT jsonb_array_elements_text(login_providers) FROM clients WHERE disabled_at IS NULL`,
	).Scan(&names).Error
	if err != nil {
		return nil, fmt.Errorf("social: read the offered providers: %w", err)
	}
	return s.configuredIn(names), nil
}

// configuredIn keeps the names that are configured here, sorted.
func (s *Service) configuredIn(names []string) []string {
	out := []string{}
	for _, name := range names {
		if s.Configured(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
