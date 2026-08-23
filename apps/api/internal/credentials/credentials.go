// Package credentials verifies a secret against its stored hash, under a
// process-wide ceiling on how many verifications run at once.
//
// argon2id is memory-hard on purpose: porte's parameters allocate 64 MiB per
// verification, and an attacker brute-forcing a stolen hash pays that too. The
// cost nobody plans for is that an unauthenticated endpoint which must hash
// before it can refuse turns every concurrent request into 64 MiB of its own
// server's memory. Measured against this code before the ceiling existed: 148
// MB resident for one bad credential, 3.2 GB at fifty, 11.1 GB at two hundred,
// from a single host, against the endpoint the whole internet is invited to
// call.
//
// The ceiling is the fix Keycloak shipped for the same bug — keycloak#28477,
// PR #28478 — and for the same reason: argon2 is CPU bound, so hashing more
// values at once than the machine has cores buys no throughput at all and
// costs 64 MiB each. Callers past the ceiling wait rather than fail, which is
// also what Keycloak chose; a refusal here would surface to the client as
// invalid_client, and telling an agent its credentials are wrong when the
// server is merely busy is worse than making it wait.
//
// This is the only package that may import porte/local. scripts/check.sh
// enforces that, because a second call site is a second uncapped path.
package credentials

import (
	"context"
	"runtime"

	"github.com/FacileStudio/porte/local"
)

// DefaultConcurrency is how many verifications run at once when the
// configuration says nothing: one per core.
//
// porte hashes with two lanes, so this deliberately oversubscribes the
// machine rather than undersubscribing it. The number that matters is the
// product: concurrency multiplied by 64 MiB is the transient memory an
// operator is agreeing to.
func DefaultConcurrency() int { return runtime.GOMAXPROCS(0) }

// Verifier checks secrets against stored hashes and seals new ones.
type Verifier struct {
	slots chan struct{}
}

// New returns a verifier admitting concurrency verifications at once. A value
// below one is raised to one: a ceiling of zero would block every login
// forever rather than limit it.
func New(concurrency int) *Verifier {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Verifier{slots: make(chan struct{}, concurrency)}
}

// Matches reports whether secret matches hash.
//
// An empty hash never matches, and costs what a match costs. Skipping the work
// for an account that does not exist would answer in a microsecond where a real
// one takes sixty milliseconds, which is an account-existence oracle. It is
// folded in here rather than left to the caller because a caller can forget to
// equalise and the omission is invisible in review.
func (v *Verifier) Matches(ctx context.Context, secret, hash string) (bool, error) {
	release, err := v.hold(ctx)
	if err != nil {
		return false, err
	}
	defer release()

	if hash == "" {
		local.EqualizeTiming(secret)
		return false, nil
	}
	return local.VerifyPassword(secret, hash), nil
}

// Hash seals a secret for storage, at porte's parameters and under the same
// ceiling. Hashing costs the same memory as verifying, and the seed is not the
// only caller it will ever have.
func (v *Verifier) Hash(ctx context.Context, secret string) (string, error) {
	release, err := v.hold(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return local.HashPassword(secret)
}

// hold takes a slot, or gives up when the caller does.
//
// Selecting on the context is what a blocking acquire cannot do: when a client
// disconnects, its request context is cancelled and the goroutine leaves the
// queue immediately instead of waiting for a slot it will never use.
func (v *Verifier) hold(ctx context.Context) (release func(), err error) {
	select {
	case v.slots <- struct{}{}:
		return func() { <-v.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
