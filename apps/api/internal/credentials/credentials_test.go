package credentials

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTheCeilingIsTheMemoryCeiling is the finding this package exists for. Each
// verification allocates 64 MiB of argon2 memory, so the number running at once
// is the transient memory the process may hold. Before the ceiling, a hundred
// concurrent bad credentials against a public endpoint meant 6.4 GB.
func TestTheCeilingIsTheMemoryCeiling(t *testing.T) {
	const ceiling = 3
	const callers = 40

	verifier := New(ceiling)
	var live, peak atomic.Int64
	var group sync.WaitGroup

	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			release, err := verifier.hold(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			now := live.Add(1)
			for {
				was := peak.Load()
				if now <= was || peak.CompareAndSwap(was, now) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			live.Add(-1)
			release()
		}()
	}
	group.Wait()

	if peak.Load() > ceiling {
		t.Fatalf("%d verifications ran at once, ceiling is %d", peak.Load(), ceiling)
	}
	if peak.Load() < ceiling {
		t.Errorf("peak was %d with %d callers: the ceiling is throttling more than it was asked to", peak.Load(), ceiling)
	}
}

// TestACallerThatGivesUpLeavesTheQueue is what a blocking acquire cannot do.
// Under a flood the queue is mostly attackers who have already hung up, and a
// goroutine waiting for a slot it will never use is memory too.
func TestACallerThatGivesUpLeavesTheQueue(t *testing.T) {
	verifier := New(1)
	release, err := verifier.hold(context.Background())
	if err != nil {
		t.Fatalf("taking the only slot: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan error, 1)
	go func() { _, err := verifier.hold(ctx); waiting <- err }()

	cancel()
	select {
	case err := <-waiting:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("a cancelled caller got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled caller is still queued")
	}
}

// TestAnEmptyHashCostsWhatAMatchCosts is the timing-oracle guard, folded into
// the verifier so that no caller can forget it. A missing client, a disabled
// one and a wrong secret all arrive here holding an empty hash; answering in a
// microsecond where a real account takes sixty milliseconds would say which.
func TestAnEmptyHashCostsWhatAMatchCosts(t *testing.T) {
	verifier := New(1)
	ctx := context.Background()

	hash, err := verifier.Hash(ctx, "the-real-secret")
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}

	start := time.Now()
	if _, err := verifier.Matches(ctx, "wrong", hash); err != nil {
		t.Fatalf("verifying: %v", err)
	}
	known := time.Since(start)

	start = time.Now()
	matched, err := verifier.Matches(ctx, "wrong", "")
	if err != nil {
		t.Fatalf("equalising: %v", err)
	}
	unknown := time.Since(start)

	if matched {
		t.Fatal("an empty hash matched a secret")
	}
	if unknown < known/2 {
		t.Errorf("an unknown account answered in %s where a known one took %s: that is an existence oracle", unknown, known)
	}
}

func TestAMatchingSecretMatches(t *testing.T) {
	verifier := New(1)
	ctx := context.Background()
	hash, err := verifier.Hash(ctx, "correct horse")
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	matched, err := verifier.Matches(ctx, "correct horse", hash)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if !matched {
		t.Fatal("the secret that was hashed does not verify")
	}
}

// TestAZeroCeilingIsRaisedRatherThanDeadlocking guards the one way this
// package could take the whole provider down instead of protecting it.
func TestAZeroCeilingIsRaisedRatherThanDeadlocking(t *testing.T) {
	for _, concurrency := range []int{0, -1} {
		verifier := New(concurrency)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if _, err := verifier.Matches(ctx, "x", ""); err != nil {
			t.Errorf("New(%d) blocks forever: %v", concurrency, err)
		}
		cancel()
	}
}
