// Package social signs people in with an account they already hold elsewhere.
//
// A provider proves who a person is; Registre decides what that proof is
// worth. The package owns the provider clients, the one-time state that ties a
// callback to the browser that started it, and the rules that turn a provider
// profile into a Registre user. It owns no routes: modules/login serves the
// sign-in and modules/account serves linking.
package social

import (
	"context"
	"errors"
	"slices"
)

// Known is every provider this build can speak. The seed validates a client's
// login_providers against it, so a typo fails the boot rather than leaving a
// button that never appears.
var Known = []string{"discord", "github"}

// IsKnown reports whether name is a provider this build can speak.
func IsKnown(name string) bool { return slices.Contains(Known, name) }

// Profile is what a provider says about the person who just authorised.
// Subject is the provider's stable id for them and the only field Registre
// trusts to recognise them again; the rest is advisory.
type Profile struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
}

// Provider is one external identity service.
type Provider interface {
	// Name is the provider's key in URLs, seeds and the connections claim.
	Name() string

	// AuthURL is where the browser goes to authorise, carrying the state and
	// the PKCE challenge.
	AuthURL(state, challenge, redirectURI string) string

	// Exchange trades the authorisation code for the person's profile.
	Exchange(ctx context.Context, code, verifier, redirectURI string) (Profile, error)
}

var (
	// ErrUnknownProvider is a provider that is not configured here.
	ErrUnknownProvider = errors.New("social: unknown provider")

	// ErrInvalidState is a state that is missing, spent, expired or not the
	// one this browser holds.
	ErrInvalidState = errors.New("social: invalid state")

	// ErrDisabled is a sign-in for an account that is disabled.
	ErrDisabled = errors.New("social: the account is disabled")

	// ErrSignupClosed is a sign-in that would create an account while public
	// sign-up is off.
	ErrSignupClosed = errors.New("social: sign-up is closed")

	// ErrUnverifiedEmail is a profile with no verified address, which is
	// neither enough to match an account nor to create one.
	ErrUnverifiedEmail = errors.New("social: the provider did not vouch for an email address")

	// ErrTaken is an external account already linked to another user.
	ErrTaken = errors.New("social: the account is linked to another user")

	// ErrAlreadyLinked is a user who already holds a different account of the
	// same provider.
	ErrAlreadyLinked = errors.New("social: the user already has an account of this provider")

	// ErrLastFactor is an unlink that would leave no way to sign in.
	ErrLastFactor = errors.New("social: this is the last way to sign in")

	// ErrNotLinked is an unlink of an account that is not linked.
	ErrNotLinked = errors.New("social: nothing is linked")
)
