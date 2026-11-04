package signup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// finish creates the account, opens its session, sets the account cookie
// and answers where the browser goes: back to the app sign-up started from
// when that login is still pending, else Registre's home.
func (s *Service) finish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token      string          `json:"token"`
		Name       string          `json:"name"`
		Locale     string          `json:"locale"`
		Accept     bool            `json:"accept_terms"`
		Password   string          `json:"password"`
		Ceremony   string          `json:"ceremony"`
		Credential json.RawMessage `json:"credential"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		encode(w, map[string]string{"error": "invalid"}, http.StatusBadRequest)
		return
	}
	created, err := s.Flow.Finish(r.Context(), flow.Account{
		Token: body.Token, Name: body.Name, Locale: body.Locale, Accepted: body.Accept, IP: clientIP(r),
		Password: body.Password, Ceremony: body.Ceremony, Credential: body.Credential,
	})
	if err != nil {
		s.refuse(w, err)
		return
	}
	s.record(r.Context(), created.User, body.Ceremony != "")
	redirect, err := s.signIn(r.Context(), w, created)
	if err != nil {
		s.logger.Warn("registre: a new account could not be signed in", slog.Any("error", err))
		if redirect == "" {
			redirect = "/login"
		}
	}
	encode(w, map[string]string{"redirect": redirect}, http.StatusCreated)
}

// signIn opens the new account's session, as a login would.
func (s *Service) signIn(ctx context.Context, w http.ResponseWriter, created flow.Created) (string, error) {
	now := time.Now().UTC()
	clientID := ""
	authReq, err := s.Users.AuthRequestByID(ctx, created.AuthRequest)
	live := created.AuthRequest != "" && err == nil && authReq.ExpiresAt.After(now) && !authReq.Done()
	if live {
		clientID = authReq.ClientID
	}
	sessionID, err := s.Users.StartSession(ctx, created.User.ID, clientID, now)
	if err != nil {
		return "", err
	}
	token, err := s.Sessions.Issue(ctx, sessionID)
	if err != nil {
		return "", err
	}
	s.Cookies.Write(w, token)
	if !live {
		return "/", nil
	}
	err = s.Users.Complete(ctx, authReq.ID, created.User.ID, sessionID, now)
	if errors.Is(err, identity.ErrInvalid) {
		return "/", nil
	}
	if err != nil {
		return "/", fmt.Errorf("signup: complete the pending login: %w", err)
	}
	return "/authorize/callback?id=" + authReq.ID, nil
}

// record writes the account.email_verified and account.signup audit rows:
// the link proved the address, then the account was created behind it.
func (s *Service) record(ctx context.Context, user schemas.User, passkey bool) {
	if s.Audit == nil {
		return
	}
	method := "password"
	if passkey {
		method = "passkey"
	}
	target := audit.Target{Type: "user", ID: user.ID}
	err := s.Audit.Record(ctx, user.ID, "account.email_verified", target, nil)
	if err == nil {
		err = s.Audit.Record(ctx, user.ID, "account.signup", target,
			map[string]any{"method": method, "locale": user.Locale})
	}
	if err != nil {
		s.logger.Warn("registre: audit write failed", slog.Any("error", err))
	}
}

// clientIP is the caller's address without its port. httpx's RealIP has
// already rewritten RemoteAddr from the trusted proxy headers.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
