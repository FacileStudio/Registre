package billing

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// stripeOrigins are the only places a checkout or portal URL may point.
// It is a function rather than a package variable so nothing can widen it
// at runtime.
func stripeOrigins() map[string]bool {
	return map[string]bool{"https://checkout.stripe.com": true, "https://billing.stripe.com": true}
}

// Catalog is Ardoise's published plans.
type Catalog struct {
	Plans []Plan `json:"plans"`
}

// Plan is one published plan with what it grants and what it costs.
type Plan struct {
	Key      string                     `json:"key"`
	Name     string                     `json:"name"`
	Features map[string]json.RawMessage `json:"features"`
	Prices   []Price                    `json:"prices"`
}

// Price is one way to pay for a plan. Amount is in the currency's minor
// unit, so 1200 eur is 12.00 EUR.
type Price struct {
	Interval  string `json:"interval"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	LookupKey string `json:"lookup_key"`
}

// SpacePlan is the plan one space is on, as Ardoise resolved it.
type SpacePlan struct {
	Plan             string                     `json:"plan"`
	Status           string                     `json:"status"`
	CurrentPeriodEnd string                     `json:"current_period_end"`
	Comped           bool                       `json:"comped"`
	Entitlements     map[string]json.RawMessage `json:"entitlements"`
}

// CheckoutRequest starts a subscription for a space. Registre fills every
// field from server-held state except LookupKey, which Ardoise resolves
// against its published catalog.
type CheckoutRequest struct {
	SpaceID    string `json:"space_id"`
	SpaceName  string `json:"space_name"`
	OwnerEmail string `json:"owner_email"`
	LookupKey  string `json:"lookup_key"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

// redirect is Ardoise's answer to checkout and portal.
type redirect struct {
	URL string `json:"url"`
}

// allowed answers the URL when its origin is exactly one of Stripe's, and
// ErrUnavailable otherwise.
func (r redirect) allowed() (string, error) {
	parsed, err := url.Parse(r.URL)
	if err != nil || parsed.User != nil || parsed.Opaque != "" {
		return "", fmt.Errorf("%w: redirect is not a plain URL", ErrUnavailable)
	}
	if !stripeOrigins()[parsed.Scheme+"://"+parsed.Host] {
		return "", fmt.Errorf("%w: redirect origin is not Stripe's", ErrUnavailable)
	}
	return parsed.String(), nil
}
