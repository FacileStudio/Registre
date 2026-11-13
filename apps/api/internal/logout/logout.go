// Package logout delivers back-channel logout tokens to relying parties.
//
// It owns no routes: the delivery is triggered by the OIDC storage when a
// session ends and by operators disabling an account, so it lives under
// internal/ by the same rule that keeps clients and keys there.
//
// The token shape is OpenID Connect Back-Channel Logout 1.0 §2.4: a signed
// JWT carrying iss, aud, iat, exp, jti, events naming the logout event, and
// the subject — and deliberately no nonce, because a logout token that looks
// like an ID token lets whoever holds it log somebody out.
package logout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"

	"crypto/rsa"
	jose "github.com/go-jose/go-jose/v4"
)

// backchannelLogoutEvent is the event key the specification fixes.
const backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"

// tokenTTL is how long a logout token verifies for after it was minted. The
// receiver acts the moment it arrives; the lifetime exists so a replayed one
// eventually fails cleanly rather than forever.
const tokenTTL = 2 * time.Minute

// postTimeout bounds one delivery. A slow app must not hold up ending the
// session that asked for the notification. The Service's shared http.Client
// carries it, reusing connections across deliveries.
const postTimeout = 5 * time.Second

// errTransient marks a delivery failure worth retrying: the app was
// unreachable, timed out, or answered 5xx or 429. A 4xx means the app read the
// token and refused it, and sending it again changes nothing.
var errTransient = errors.New("logout: transient")

// retryDelays spaces the retries after a transient first failure: four
// attempts in all, the last starting about a minute after the logout. The
// window stays short on purpose, because porte revokes every session of the
// user and a late retry would also end one the user opened after logging out.
var retryDelays = []time.Duration{2 * time.Second, 10 * time.Second, 45 * time.Second}

// Service mints and delivers logout tokens.
type Service struct {
	keys        *keys.Service
	register    *clients.Service
	issuer      string
	logger      *slog.Logger
	now         func() time.Time
	deliveries  *http.Client
	retryDelays []time.Duration
}

// New builds the service over the keyring and the register.
func New(keyring *keys.Service, register *clients.Service, issuer string, logger *slog.Logger) *Service {
	return &Service{
		keys: keyring, register: register, issuer: issuer,
		logger: logger, now: func() time.Time { return time.Now().UTC() },
		deliveries: &http.Client{Timeout: postTimeout}, retryDelays: retryDelays,
	}
}

// Fire sends a back-channel logout token to each of clientIDs that registered
// a logout URI. The first attempt runs inline, so the caller waits for one
// delivery per app and no more. When it fails transiently (network error,
// timeout, 5xx, 429), a background goroutine retries on retryDelays, detached
// from ctx so a retry outlives the request that triggered it. A 4xx is final.
// An app that never answers keeps its own session until it expires; the give-up
// lands in Journal at Error. A process that exits right after Fire, like the
// user-disable command, drops its pending retries.
func (s *Service) Fire(ctx context.Context, userID string, clientIDs []string) error {
	if len(clientIDs) == 0 {
		return nil
	}
	for _, clientID := range clientIDs {
		row, err := s.register.ByClientID(ctx, clientID)
		if err != nil {
			s.logger.Warn("registre: back-channel logout skipped, unknown client",
				slog.String("client_id", clientID), slog.Any("error", err))
			continue
		}
		if row.BackchannelLogoutURI == "" {
			continue
		}
		if err := s.deliver(ctx, userID, row.ClientID, row.BackchannelLogoutURI); err != nil {
			s.logger.Warn("registre: back-channel logout failed",
				slog.String("client_id", row.ClientID),
				slog.String("uri", row.BackchannelLogoutURI), slog.Any("error", err))
			if errors.Is(err, errTransient) {
				go s.retry(context.WithoutCancel(ctx), userID, row.ClientID, row.BackchannelLogoutURI)
			}
		}
	}
	return nil
}

// retry re-sends a logout token after each of retryDelays until one delivery
// succeeds, one fails permanently, or the delays run out.
func (s *Service) retry(ctx context.Context, userID, clientID, uri string) {
	for attempt, delay := range s.retryDelays {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		err := s.deliver(ctx, userID, clientID, uri)
		if err == nil {
			return
		}
		if !errors.Is(err, errTransient) {
			s.logger.Warn("registre: back-channel logout retry refused",
				slog.String("client_id", clientID), slog.String("uri", uri),
				slog.Int("attempts", attempt+2), slog.Any("error", err))
			return
		}
	}
	s.logger.Error("registre: back-channel logout gave up",
		slog.String("client_id", clientID), slog.String("uri", uri),
		slog.Int("attempts", len(s.retryDelays)+1))
}

// deliver mints one token and POSTs it as the specification's form parameter.
func (s *Service) deliver(ctx context.Context, userID, clientID, uri string) error {
	token, err := s.token(userID, clientID)
	if err != nil {
		return err
	}
	post, err := http.NewRequestWithContext(ctx, http.MethodPost, uri,
		strings.NewReader(url.Values{"logout_token": {token}}.Encode()))
	if err != nil {
		return fmt.Errorf("logout: build the request: %w", err)
	}
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sent := time.Now()
	response, err := s.deliveries.Do(post)
	if err != nil {
		return fmt.Errorf("logout: deliver: %w: %w", errTransient, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("logout: %w: %s answered %d", errTransient, uri, response.StatusCode)
	}
	if response.StatusCode >= 300 {
		return fmt.Errorf("logout: %s answered %d", uri, response.StatusCode)
	}
	s.logger.Info("registre: back-channel logout delivered",
		slog.String("client_id", clientID), slog.Duration("took", time.Since(sent)))
	return nil
}

// token signs one logout token naming sub and audience.
func (s *Service) token(userID, clientID string) (string, error) {
	claims, err := s.claims(userID, clientID, tokenTTL)
	if err != nil {
		return "", err
	}
	claims["events"] = map[string]any{backchannelLogoutEvent: map[string]any{}}
	return s.sign(claims, "JWT")
}

// claims is the envelope every token this package signs carries.
func (s *Service) claims(userID, clientID string, ttl time.Duration) (map[string]any, error) {
	jti, err := ids.New()
	if err != nil {
		return nil, fmt.Errorf("logout: mint a jti: %w", err)
	}
	issued := s.now()
	return map[string]any{
		"iss": s.issuer,
		"aud": []string{clientID},
		"iat": issued.Unix(),
		"exp": issued.Add(ttl).Unix(),
		"jti": jti,
		"sub": userID,
	}, nil
}

// sign signs claims with the active key, with the given typ header.
func (s *Service) sign(claims map[string]any, typ string) (string, error) {
	signing, err := s.keys.Signing(context.Background())
	if err != nil {
		return "", fmt.Errorf("logout: read the signing key: %w", err)
	}
	private, ok := signing.Key().(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("logout: signing key is %T, not RSA", signing.Key())
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("logout: encode the claims: %w", err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: private},
		(&jose.SignerOptions{}).WithType(jose.ContentType(typ)).WithHeader("kid", signing.ID()))
	if err != nil {
		return "", fmt.Errorf("logout: build the signer: %w", err)
	}
	object, err := signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("logout: sign the token: %w", err)
	}
	return object.CompactSerialize()
}
