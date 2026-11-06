package social

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidReturn marks a return_to that is not the origin of a redirect URI
// of an enabled client that opted into the provider.
var ErrInvalidReturn = errors.New("social: invalid return_to")

// ValidReturn accepts raw only when it is an absolute https URL (http when the
// issuer is insecure, which is development) without userinfo whose scheme,
// host and port equal the origin of a redirect URI of an enabled client that
// lists provider in its login_providers. It is checked when a link starts and
// again before the browser is sent anywhere.
func (s *Service) ValidReturn(ctx context.Context, provider, raw string) error {
	target, err := url.Parse(raw)
	if err != nil || target.Hostname() == "" || target.User != nil || !s.allowedScheme(target.Scheme) {
		return ErrInvalidReturn
	}
	var uris []string
	err = s.db.WithContext(ctx).Raw(
		`SELECT jsonb_array_elements_text(redirect_uris) FROM clients
		 WHERE disabled_at IS NULL AND login_providers @> to_jsonb(?::text)`, provider,
	).Scan(&uris).Error
	if err != nil {
		return fmt.Errorf("social: read the redirect URIs: %w", err)
	}
	for _, uri := range uris {
		registered, err := url.Parse(uri)
		if err == nil && registered.Hostname() != "" && origin(registered) == origin(target) {
			return nil
		}
	}
	return ErrInvalidReturn
}

// allowedScheme is https, plus http when the issuer is insecure.
func (s *Service) allowedScheme(scheme string) bool {
	return scheme == "https" || (scheme == "http" && !s.cfg.Secure)
}

// origin is the scheme, host and port of u, lowercased.
func origin(u *url.URL) string {
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

// ReturnURL is the validated return_to with key=value merged into its query,
// keeping what was already there. It answers "" when returnTo is empty or no
// longer valid, so the caller falls back to the account page.
func (s *Service) ReturnURL(ctx context.Context, provider, returnTo, key, value string) string {
	if returnTo == "" || s.ValidReturn(ctx, provider, returnTo) != nil {
		return ""
	}
	target, err := url.Parse(returnTo)
	if err != nil {
		return ""
	}
	query := target.Query()
	query.Set(key, value)
	target.RawQuery = query.Encode()
	return target.String()
}
