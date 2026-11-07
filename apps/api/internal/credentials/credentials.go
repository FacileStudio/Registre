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
// costs 64 MiB each. Callers past the ceiling wait, which is also what
// Keycloak chose, but only for DefaultQueueWait: past that they get ErrBusy,
// which the HTTP layers answer 503. An unbounded queue under a flood is a box
// that answers nobody; a short queue and an honest 503 is a box that still
// answers its own health check. ErrBusy is never a credential verdict, so no
// caller may read it as "wrong secret".
//
// This is the only package that may import porte/local. scripts/check.sh
// enforces that, because a second call site is a second uncapped path.
package credentials

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/FacileStudio/porte/local"
)

// MinPasswordLength is the floor every path that sets a human password
// enforces: the account page, the emailed reset, and invitation redemption.
//
// It lives here rather than in each module because it was three numbers
// before, and they disagreed. A reset accepted eight characters while the
// account page demanded twelve, which made the emailed link the way around
// the stricter form. A floor that one path undercuts is the floor.
//
// It is fifteen because NIST SP 800-63B-4 §3.1.1.2 asks for fifteen when the
// password is the only factor, and one floor for every account is simpler
// than one that moves when a second factor is enrolled or removed.
//
// Break-glass keeps a higher one of its own, for a reason that does not
// generalise: that password is chosen under pressure and opens every app.
const MinPasswordLength = 15

// DefaultConcurrency is how many verifications run at once when the
// configuration says nothing: two per core.
//
// porte hashes with two lanes, so one verification per core already keeps the
// machine busy; the second slot per core absorbs the gaps while a lane waits
// on memory. The number that matters is the product: concurrency multiplied by
// 64 MiB is the transient memory an operator is agreeing to.
func DefaultConcurrency() int { return runtime.GOMAXPROCS(0) * 2 }

// DefaultQueueWait is how long a caller waits for a slot before it is told the
// server is busy.
//
// A verification takes about 150 ms of CPU, so three seconds is a queue of
// roughly twenty callers per core: long enough to ride out a burst, short
// enough that the answer arrives before the proxy and the 15 second write
// timeout give up. Past it, waiting longer only moves the failure to a place
// where it is harder to read.
const DefaultQueueWait = 3 * time.Second

// ErrBusy means every verification slot stayed taken for the whole queue wait.
// It is the server's overload, not the caller's mistake, and the HTTP layers
// answer it 503 rather than a credential refusal.
var ErrBusy = errors.New("credentials: all verification slots are busy")

// Verifier checks secrets against stored hashes and seals new ones.
type Verifier struct {
	slots chan struct{}
	wait  time.Duration
}

// New returns a verifier admitting concurrency verifications at once. A value
// below one is raised to one: a ceiling of zero would block every login
// forever rather than limit it.
func New(concurrency int) *Verifier {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Verifier{slots: make(chan struct{}, concurrency), wait: DefaultQueueWait}
}

// WithQueueWait sets how long a caller queues for a slot before ErrBusy. A
// value below one millisecond is raised to one, so a mistyped zero cannot turn
// every verification into an immediate refusal while a slot is free.
func (v *Verifier) WithQueueWait(wait time.Duration) *Verifier {
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	v.wait = wait
	return v
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

// hold takes a slot, or gives up when the caller does or the queue wait ends.
//
// Selecting on the context is what a blocking acquire cannot do: when a client
// disconnects, its request context is cancelled and the goroutine leaves the
// queue immediately instead of waiting for a slot it will never use. The timer
// is what turns a long queue into ErrBusy instead of a hang.
func (v *Verifier) hold(ctx context.Context) (release func(), err error) {
	select {
	case v.slots <- struct{}{}:
		return func() { <-v.slots }, nil
	default:
	}
	timer := time.NewTimer(v.wait)
	defer timer.Stop()
	select {
	case v.slots <- struct{}{}:
		return func() { <-v.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, ErrBusy
	}
}
