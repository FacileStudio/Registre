package social

import (
	"net/http"
	"time"
)

const handoffName = "registre-2fa"

// HandoffTTL is how long the second-factor cookie lives. It matches the
// five minutes a login challenge stays answerable.
const HandoffTTL = 5 * time.Minute

func (s *Service) handoffName() string {
	if s.cfg.Secure {
		return "__Host-" + handoffName
	}
	return handoffName
}

// HandoffCookie carries a second-factor challenge from the social callback to
// the login page's code step, so the token never rides in a URL. HttpOnly, so
// the page cannot read it; Strict, because only the page's own same-origin
// POST needs it.
func (s *Service) HandoffCookie(token string) *http.Cookie {
	return s.handoff(token, int(HandoffTTL.Seconds()))
}

// ClearHandoff expires the second-factor cookie.
func (s *Service) ClearHandoff() *http.Cookie { return s.handoff("", -1) }

// Handoff is the challenge this browser's cookie holds, or "".
func (s *Service) Handoff(r *http.Request) string {
	cookie, err := r.Cookie(s.handoffName())
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (s *Service) handoff(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: s.handoffName(), Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.Secure, SameSite: http.SameSiteStrictMode,
	}
}
