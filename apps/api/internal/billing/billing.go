// Package billing is registre's server-to-server client for Ardoise, the
// suite's billing app. The spaces pages use it to show a space's plan and to
// start a Stripe checkout or billing portal session for an owner.
//
// Every call goes to the one base URL from the environment with the shared
// bearer, under a fixed timeout and a response size cap. The bearer never
// leaves this package: errors name the route and the status, never the
// token or the body. A checkout or portal URL Ardoise answers is only handed
// on when its origin is exactly Stripe's, so a compromised or misconfigured
// Ardoise cannot turn registre into an open redirect.
package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/env"
)

// Timeout bounds one call to Ardoise. The page shows "billing unavailable"
// rather than waiting on a slow upstream.
const Timeout = 5 * time.Second

// maxResponseBytes caps what registre reads back from Ardoise.
const maxResponseBytes = 1 << 20

// ErrUnavailable is any failure to get a usable answer from Ardoise. The
// cause is wrapped for the server log; callers show a generic message.
var ErrUnavailable = errors.New("billing unavailable")

// Client calls Ardoise's billing API.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// New builds a client over Ardoise's base URL and the shared bearer token.
func New(base, token string) *Client {
	return &Client{base: base, token: token, http: &http.Client{
		Timeout:       Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// FromConfig builds the client the plan section uses, or nil when the
// section is off. A token shorter than env.MinBillingTokenLength counts as
// absent and is reported without its value.
func FromConfig(cfg env.Billing, log *slog.Logger) *Client {
	if !cfg.PlansEnabled() {
		if cfg.ArdoiseURL != "" || cfg.Token != "" {
			log.Warn("registre: plan section off, ARDOISE_BILLING_URL and a BILLING_API_TOKEN " +
				"of at least 32 characters must both be set")
		}
		return nil
	}
	return New(cfg.ArdoiseURL, cfg.Token)
}

// Catalog answers the published plans, the only ones that can be bought.
func (c *Client) Catalog(ctx context.Context) (Catalog, error) {
	var out Catalog
	return out, c.call(ctx, http.MethodGet, "/api/catalog", nil, &out)
}

// Space answers the plan a space is on.
func (c *Client) Space(ctx context.Context, spaceID string) (SpacePlan, error) {
	var out SpacePlan
	return out, c.call(ctx, http.MethodGet, "/billing/spaces/"+url.PathEscape(spaceID), nil, &out)
}

// Checkout opens a Stripe checkout session and answers its URL, which is
// always on https://checkout.stripe.com.
func (c *Client) Checkout(ctx context.Context, in CheckoutRequest) (string, error) {
	var out redirect
	if err := c.call(ctx, http.MethodPost, "/billing/checkout", in, &out); err != nil {
		return "", err
	}
	return out.allowed()
}

// Portal opens a Stripe billing portal session and answers its URL, which
// is always on https://billing.stripe.com.
func (c *Client) Portal(ctx context.Context, spaceID, returnURL string) (string, error) {
	var out redirect
	body := map[string]string{"space_id": spaceID, "return_url": returnURL}
	if err := c.call(ctx, http.MethodPost, "/billing/portal", body, &out); err != nil {
		return "", err
	}
	return out.allowed()
}

// call sends one request and decodes a 2xx JSON answer into out.
func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	var payload io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("%w: encode %s: %w", ErrUnavailable, path, err)
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base+path, payload)
	if err != nil {
		return fmt.Errorf("%w: build %s", ErrUnavailable, path)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	if in != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %s %s: %w", ErrUnavailable, method, path, err)
	}
	defer func() { _ = response.Body.Close() }()
	return decode(response, path, out)
}

// decode reads at most maxResponseBytes of a 2xx answer into out.
func decode(response *http.Response, path string, out any) error {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		return fmt.Errorf("%w: %s answered %d", ErrUnavailable, path, response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) > maxResponseBytes {
		return fmt.Errorf("%w: %s answered an unreadable or oversized body", ErrUnavailable, path)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %s answered malformed JSON", ErrUnavailable, path)
	}
	return nil
}
