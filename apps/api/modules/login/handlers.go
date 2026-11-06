package login

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/challenges"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/limits"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// DefaultLoginLimit and DefaultLoginBurst are the budget one source address
// gets on the credential endpoint. Tight enough that a password guesser runs
// out of attempts in minutes, loose enough that a family or small studio
// behind one NAT never notices it exists. Lockout-by-account is deliberately
// absent: an attacker who can lock an account can lock a person out, which is
// the denial of service they wanted.
const (
	DefaultLoginLimit = 5
	DefaultLoginBurst = 10
)

// Service is registre's login backend: it checks credentials for the client
// app, which owns every page a human sees between the app that sent them here
// and the code that sends them back.
type Service struct {
	identity *identity.Service
	logger   *slog.Logger
	now      func() time.Time
	browser  map[string]any
	factor   SecondFactor
	passkeys *passkeys.Service
	signup   bool
	social   *social.Service
}

// New builds the login service over identity.
func New(identityService *identity.Service, logger *slog.Logger) *Service {
	return &Service{identity: identityService, logger: logger, now: func() time.Time { return time.Now().UTC() }}
}

// WithSecondFactor wires the TOTP step and the account cookie.
//
// A service built without it logs a human in on the password alone and sets
// no cookie, which is what registre did before the factor existed, so nothing
// on that path may dereference these. What it must not do is fail open: an
// account that has enrolled a factor is refused outright by a deployment that
// cannot check one, because letting it through would make the enrolment
// itself the way around the enrolment.
func (s *Service) WithSecondFactor(deps SecondFactor) *Service {
	s.factor = deps
	return s
}

// WithBrowserConfig adds what GET /api/config hands to the client build. Both
// values come from one env pair, so a nil map means reporting is off and the
// endpoint answers without the journal key at all.
func (s *Service) WithBrowserConfig(url, key string) *Service {
	if url != "" && key != "" {
		s.browser = map[string]any{"url": url, "key": key}
	}
	return s
}

// MountWithLimits registers the login routes with an explicit per-address
// budget. Tests use it to make the limiter reachable without hammering the
// real default; production goes through Mount.
//
// Both credential endpoints share one limiter instance, so the two steps draw
// on a single per-address budget. A second-factor endpoint with a budget of
// its own is where a guesser moves once the password endpoint is throttled,
// and six digits is a far smaller space than a password.
//
// Every route that can finish a sign-in also wants X-Facile-CSRF, which no
// cross-site page can set without a preflight registre never answers. A
// login a third-party page can submit is a login CSRF: it signs the victim
// in to the attacker's account and finishes the attacker's flow with it.
func MountWithLimits(router chi.Router, service *Service, limit float64, burst int) {
	budget := limits.PerAddress(limit, burst)
	csrf := accounts.NewGuard(nil, accounts.Cookies{}).CSRF
	router.With(budget, csrf).Post("/api/login", service.jsonSubmit)
	router.With(budget, csrf).Post("/api/login/totp", service.jsonSecondFactor)
	router.With(budget).Post("/api/login/passkey/begin", service.beginPasskey)
	router.With(budget, csrf).Post("/api/login/passkey/finish", service.finishPasskey)
	router.With(budget).Get("/api/login/providers", service.providers)
	router.With(budget).Get("/api/login/social/{provider}/start", service.socialStart)
	router.With(budget).Get("/api/login/social/{provider}/callback", service.socialCallback)
	router.Get("/api/login/session", service.sessionLogin)
	router.Post("/api/login/session", service.sessionLogin)
	router.Get("/api/config", service.browserConfig)
}

// decision is what a credential check that got past the password hands the
// client: either the callback to navigate to, or the challenge that still
// owes a second factor. Never both.
type decision struct {
	Redirect  string `json:"redirect,omitempty"`
	Challenge string `json:"challenge,omitempty"`
}

// jsonSubmit answers the client app's fetch with the decision that matters:
// a redirect target on success, a challenge when the account has a second
// factor, one generic refusal otherwise. The browser follows the redirect as
// a full navigation, so the code exchange never runs inside JavaScript.
//
// Every refusal carries the same body. The identity service already equalises
// timing between unknown address and wrong password, so nothing here can
// distinguish them either. Answering a challenge instead of a redirect does
// tell the caller the account has a factor, which is unavoidable: they have
// already proven the password by then.
func (s *Service) jsonSubmit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AuthRequest string `json:"auth_request"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	answer, err := s.authenticate(r.Context(), w, body.AuthRequest, body.Email, body.Password)
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, answer, http.StatusOK)
}

// refuse writes the one refusal both endpoints make. A refusal registre
// caused rather than the caller is logged and answered 500, with the same
// body, so an operator can find it without the client learning anything. A
// user who proved who they are but may not use the app answers 403 no_access:
// they already know their account exists, so saying so leaks nothing.
func (s *Service) refuse(w http.ResponseWriter, err error) {
	if errors.Is(err, access.ErrNoAccess) {
		encode(w, map[string]string{"error": "no_access"}, http.StatusForbidden)
		return
	}
	status := http.StatusUnauthorized
	switch {
	case errors.Is(err, identity.ErrInvalid), errors.Is(err, errAuthRequestGone),
		errors.Is(err, challenges.ErrInvalid), errors.Is(err, totp.ErrInvalid):
	case errors.Is(err, errNoSecondFactor):
		s.logger.Error("registre: an enrolled account met a build with no second factor service",
			slog.Any("error", err))
	default:
		s.logger.Error("registre: login verification failed", slog.Any("error", err))
		status = http.StatusInternalServerError
	}
	encode(w, map[string]string{"error": "invalid"}, status)
}

// encode writes one JSON object with the given status.
func encode(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, "response write failed", http.StatusInternalServerError)
	}
}

// authenticate validates the pending auth request and verifies the
// credentials. An account with no second factor is logged in here; one that
// has a factor gets a challenge and nothing else, because a session opened
// now is a session the password alone opened.
func (s *Service) authenticate(
	ctx context.Context, w http.ResponseWriter, id, email, password string,
) (decision, error) {
	authReq, err := s.pendingRequest(ctx, id)
	if err != nil {
		return decision{}, err
	}
	user, err := s.identity.Verify(ctx, email, password)
	if err != nil {
		return decision{}, err
	}
	if user.TOTPEnrolled() {
		if s.factor.Challenges == nil || s.factor.Codes == nil {
			return decision{}, errNoSecondFactor
		}
		token, err := s.factor.Challenges.Issue(ctx, user.ID, id)
		if err != nil {
			return decision{}, err
		}
		return decision{Challenge: token}, nil
	}
	return s.finish(ctx, w, user.ID, authReq)
}

func (s *Service) completeDirect(
	ctx context.Context, w http.ResponseWriter, userID string,
) (string, error) {
	sessionID, err := s.identity.StartSession(ctx, userID, "", s.now())
	if err != nil {
		return "", fmt.Errorf("identity: start the session: %w", err)
	}
	if err := s.writeAccountCookie(ctx, w, sessionID); err != nil {
		return "", err
	}
	if errors.Is(s.identity.ConsentOwed(ctx, userID), identity.ErrConsentRequired) {
		return ConsentPath, nil
	}
	return "/", nil
}

func (s *Service) complete(
	ctx context.Context, w http.ResponseWriter, userID string, authReq schemas.AuthRequest,
) (string, error) {
	sessionID, err := s.identity.StartSession(ctx, userID, authReq.ClientID, s.now())
	if err != nil {
		return "", fmt.Errorf("identity: start the session: %w", err)
	}
	completed := s.identity.Complete(ctx, authReq.ID, userID, sessionID, s.now())
	owesConsent := errors.Is(completed, identity.ErrConsentRequired)
	if completed != nil && !owesConsent {
		s.identity.DropSession(ctx, sessionID)
		return "", fmt.Errorf("identity: complete the login: %w", completed)
	}
	if err := s.writeAccountCookie(ctx, w, sessionID); err != nil {
		return "", err
	}
	if owesConsent {
		return consentURL(authReq.ID), nil
	}
	return "/authorize/callback?id=" + authReq.ID, nil
}

// writeAccountCookie hands the human the session registre's own account pages
// read. A deployment wired without the account service logs in exactly as it
// did before, so an absent service is a no-op here and not a nil dereference.
func (s *Service) writeAccountCookie(ctx context.Context, w http.ResponseWriter, sessionID string) error {
	if s.factor.Accounts == nil {
		return nil
	}
	token, err := s.factor.Accounts.Issue(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("accounts: issue the account session: %w", err)
	}
	if err := s.factor.Accounts.MarkFactorProven(ctx, sessionID); err != nil {
		return err
	}
	s.factor.Cookies.Write(w, token)
	return nil
}

// errAuthRequestGone marks an unknown, expired or already-used login link.
var errAuthRequestGone = errors.New("login: auth request is gone")

// browserConfig answers the client build's boot-time configuration. It is
// unauthenticated and must stay free of anything a visitor cannot already
// see: today that is only the browser Journal pair, which is the public,
// origin-locked kind of key rather than a secret.
func (s *Service) browserConfig(w http.ResponseWriter, _ *http.Request) {
	body := map[string]any{"signup": s.signup, "passkeys": s.passkeys != nil}
	if s.browser != nil {
		body["journal"] = s.browser
	}
	encode(w, body, http.StatusOK)
}
