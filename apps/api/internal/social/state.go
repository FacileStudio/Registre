package social

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// StateTTL is how long a person has to authorise at the provider.
const StateTTL = 10 * time.Minute

const cookieName = "registre-social"

// Started is a sign-in or link on its way to the provider: where to send the
// browser, and the state to hold in a cookie until it comes back.
type Started struct {
	URL   string
	State string
}

// Begin records a sign-in for authRequestID, or a link for userID, and
// answers the provider URL. Exactly one of the two is set. returnTo is where
// a finished link sends the browser; it must pass ValidReturn or Begin
// refuses with ErrInvalidReturn before recording anything.
func (s *Service) Begin(ctx context.Context, provider, authRequestID, userID, returnTo string) (Started, error) {
	impl, ok := s.providers[provider]
	if !ok {
		return Started{}, ErrUnknownProvider
	}
	if returnTo != "" {
		if err := s.ValidReturn(ctx, provider, returnTo); err != nil {
			return Started{}, err
		}
	}
	state, err := ids.Secret()
	if err != nil {
		return Started{}, fmt.Errorf("social: mint a state: %w", err)
	}
	verifier, err := newVerifier()
	if err != nil {
		return Started{}, err
	}
	now := s.now()
	row := schemas.SocialState{
		StateHash: digest(state), Provider: provider, AuthRequestID: authRequestID, UserID: userID,
		Verifier: verifier, ReturnTo: returnTo, ExpiresAt: now.Add(StateTTL), CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return Started{}, fmt.Errorf("social: record the state: %w", err)
	}
	return Started{URL: impl.AuthURL(state, challenge(verifier), s.RedirectURI(provider)), State: state}, nil
}

// Take spends the state a callback arrived with. held is the value of the
// browser's state cookie: a state the browser does not hold was not started
// here, and is refused without touching the stored one.
func (s *Service) Take(ctx context.Context, state, held string) (schemas.SocialState, error) {
	if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(held)) != 1 {
		return schemas.SocialState{}, ErrInvalidState
	}
	var rows []schemas.SocialState
	err := s.db.WithContext(ctx).Raw(
		`DELETE FROM social_states WHERE state_hash = ? AND expires_at > ?
		 RETURNING state_hash, provider, auth_request_id, user_id, verifier, return_to, expires_at, created_at`,
		digest(state), s.now(),
	).Scan(&rows).Error
	if err != nil {
		return schemas.SocialState{}, fmt.Errorf("social: spend the state: %w", err)
	}
	if len(rows) != 1 {
		return schemas.SocialState{}, ErrInvalidState
	}
	return rows[0], nil
}

// Complete finishes the provider half of a callback: the profile for a code.
func (s *Service) Complete(ctx context.Context, row schemas.SocialState, code string) (Profile, error) {
	impl, ok := s.providers[row.Provider]
	if !ok {
		return Profile{}, ErrUnknownProvider
	}
	profile, err := impl.Exchange(ctx, code, row.Verifier, s.RedirectURI(row.Provider))
	if err != nil {
		return Profile{}, err
	}
	if profile.Subject == "" {
		return Profile{}, errors.New("social: the provider returned no subject")
	}
	return profile, nil
}

// PurgeExpired drops states that expired before cutoff.
func (s *Service) PurgeExpired(ctx context.Context, cutoff time.Time) error {
	err := s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.SocialState{}).Error
	if err != nil {
		return fmt.Errorf("social: purge the states: %w", err)
	}
	return nil
}

// Cookie is the cookie that binds a started sign-in to its browser. Lax so
// it rides the provider's redirect back, which is a top-level GET.
func (s *Service) Cookie(state string) *http.Cookie {
	return s.cookie(state, int(StateTTL.Seconds()))
}

// ClearCookie expires the state cookie.
func (s *Service) ClearCookie() *http.Cookie { return s.cookie("", -1) }

// Held is the state this browser's cookie holds, or "".
func (s *Service) Held(r *http.Request) string {
	cookie, err := r.Cookie(s.cookieName())
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (s *Service) cookieName() string {
	if s.cfg.Secure {
		return "__Host-" + cookieName
	}
	return cookieName
}

func (s *Service) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: s.cookieName(), Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.Secure, SameSite: http.SameSiteLaxMode,
	}
}

// digest is the stored form of a state.
func digest(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

// newVerifier mints a PKCE code verifier: 32 random bytes, base64url.
func newVerifier() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("social: mint a verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// challenge is the S256 PKCE challenge for a verifier.
func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
