package login

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
)

// Service is registre's login backend: it checks credentials for the client
// app, which owns every page a human sees between the app that sent them here
// and the code that sends them back.
type Service struct {
	identity *identity.Service
	logger   *slog.Logger
	now      func() time.Time
}

// New builds the login service over identity.
func New(identityService *identity.Service, logger *slog.Logger) *Service {
	return &Service{identity: identityService, logger: logger, now: func() time.Time { return time.Now().UTC() }}
}

// jsonSubmit answers the client app's fetch with the decision that matters:
// a redirect target on success, one generic refusal otherwise. The browser
// follows the redirect as a full navigation, so the code exchange never runs
// inside JavaScript.
//
// Every refusal carries the same body. The identity service already equalises
// timing between unknown address and wrong password, so nothing here can
// distinguish them either.
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
	redirect, err := s.authenticate(r.Context(), body.AuthRequest, body.Email, body.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if !errors.Is(err, identity.ErrInvalid) && !errors.Is(err, errAuthRequestGone) {
			s.logger.Error("registre: login verification failed", slog.Any("error", err))
			status = http.StatusInternalServerError
		}
		encode(w, map[string]string{"error": "invalid"}, status)
		return
	}
	encode(w, map[string]string{"redirect": redirect}, http.StatusOK)
}

// encode writes one JSON object with the given status.
func encode(w http.ResponseWriter, body map[string]string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, "response write failed", http.StatusInternalServerError)
	}
}

// authenticate validates the pending auth request, verifies the credentials,
// marks the flow authenticated and returns the callback to send the browser
// to, which is where zitadel issues the code and redirects into the app.
func (s *Service) authenticate(ctx context.Context, id, email, password string) (string, error) {
	authReq, err := s.identity.AuthRequestByID(ctx, id)
	if err != nil || !authReq.ExpiresAt.After(s.now()) || authReq.Done() {
		return "", errAuthRequestGone
	}
	user, err := s.identity.Verify(ctx, email, password)
	if err != nil {
		return "", err
	}
	if err := s.identity.Complete(ctx, id, user.ID, s.now()); err != nil {
		return "", fmt.Errorf("identity: complete the login: %w", err)
	}
	return "/authorize/callback?id=" + id, nil
}

// errAuthRequestGone marks an unknown, expired or already-used login link.
var errAuthRequestGone = errors.New("login: auth request is gone")
