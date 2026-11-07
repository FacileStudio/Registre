package throttle

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
)

func TestTheDelayCurveIsFreeThenDoublingToTheCap(t *testing.T) {
	p := DefaultPolicy()
	want := map[int]time.Duration{
		1: 0, 4: 0,
		5: 30 * time.Second, 6: time.Minute, 7: 2 * time.Minute, 8: 4 * time.Minute,
		9: 8 * time.Minute, 10: 15 * time.Minute, 11: 15 * time.Minute, 500: 15 * time.Minute,
	}
	for n, d := range want {
		if got := delay(p, n); got != d {
			t.Errorf("delay after attempt %d = %s, want %s", n, got, d)
		}
	}
}

func TestKeyNormalisesTheIdentityAndSeparatesKinds(t *testing.T) {
	if Key("password", " Bob@Example.test ") != Key("password", "bob@example.test") {
		t.Error("case and padding change the budget")
	}
	if Key("password", "bob@example.test") == Key("totp", "bob@example.test") {
		t.Error("two kinds share a budget: a password success would reset the code guesses")
	}
}

func near(got, want time.Duration) bool { return got > want-time.Millisecond && got <= want }

type clock struct{ now atomic.Pointer[time.Time] }

func (c *clock) read() time.Time { return *c.now.Load() }
func (c *clock) advance(d time.Duration) {
	next := c.read().Add(d)
	c.now.Store(&next)
}

func newService(t *testing.T) (*Service, *clock) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC()
	c := &clock{}
	c.now.Store(&start)
	service := New(db)
	service.now = c.read
	return service, c
}

func TestFiveFreeAttemptsThenTheDelaysDoubleToTheCap(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	key := Key("password", "a@example.test")

	for i := 1; i <= 5; i++ {
		if wait, err := s.Attempt(ctx, key); err != nil || wait != 0 {
			t.Fatalf("attempt %d = %s, %v, want free", i, wait, err)
		}
	}
	wait, err := s.Attempt(ctx, key)
	if err != nil || !near(wait, 30*time.Second) {
		t.Fatalf("attempt 6 = %s, %v, want 30s", wait, err)
	}
	c.advance(29 * time.Second)
	if wait, _ := s.Attempt(ctx, key); !near(wait, time.Second) {
		t.Fatalf("a refused attempt moved the clock: waits %s, want 1s", wait)
	}
	c.advance(time.Second)
	if wait, _ := s.Attempt(ctx, key); wait != 0 {
		t.Fatalf("attempt after the delay = %s, want allowed", wait)
	}
	if wait, _ := s.Attempt(ctx, key); !near(wait, time.Minute) {
		t.Fatalf("the next delay = %s, want 1m", wait)
	}
}

func TestTheDelayStopsDoublingAtTheCap(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	key := Key("password", "cap@example.test")
	for range 6 {
		_, _ = s.Attempt(ctx, key)
		c.advance(time.Minute)
	}
	for range 8 {
		c.advance(time.Hour)
		if wait, _ := s.Attempt(ctx, key); wait != 0 {
			t.Fatalf("an attempt an hour on was refused for %s", wait)
		}
	}
	if wait, _ := s.Attempt(ctx, key); !near(wait, 15*time.Minute) {
		t.Fatalf("the delay = %s, want the 15m cap", wait)
	}
}

func TestResetFreesTheAccountAndOtherAccountsAreUntouched(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	hot, cold := Key("password", "hot@example.test"), Key("password", "cold@example.test")
	for range 6 {
		_, _ = s.Attempt(ctx, hot)
	}
	if wait, _ := s.Attempt(ctx, hot); wait == 0 {
		t.Fatal("setup: the account is not throttled")
	}
	if wait, _ := s.Attempt(ctx, cold); wait != 0 {
		t.Fatal("one account's failures throttled another")
	}
	if err := s.Reset(ctx, hot); err != nil {
		t.Fatal(err)
	}
	if wait, _ := s.Attempt(ctx, hot); wait != 0 {
		t.Fatal("a reset account is still throttled")
	}
}

func TestParallelGuessesAreCountedExactly(t *testing.T) {
	s, _ := newService(t)
	key := Key("password", "race@example.test")
	var allowed atomic.Int64
	var group sync.WaitGroup
	for range 40 {
		group.Add(1)
		go func() {
			defer group.Done()
			if wait, err := s.Attempt(context.Background(), key); err == nil && wait == 0 {
				allowed.Add(1)
			}
		}()
	}
	group.Wait()
	if allowed.Load() != 5 {
		t.Fatalf("%d of 40 parallel guesses went through, want exactly the 5 free ones", allowed.Load())
	}
}

func TestAnIdentityIsForgottenAfterTheWindow(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	key := Key("password", "old@example.test")
	for range 5 {
		_, _ = s.Attempt(ctx, key)
	}
	c.advance(DefaultPolicy().Window + time.Minute)
	for i := 1; i <= 5; i++ {
		if wait, _ := s.Attempt(ctx, key); wait != 0 {
			t.Fatalf("attempt %d after the window was refused for %s", i, wait)
		}
	}
	if err := s.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeDeletesOnlyStaleRows(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	_, _ = s.Attempt(ctx, "stale")
	c.advance(DefaultPolicy().Window + time.Minute)
	_, _ = s.Attempt(ctx, "fresh")
	if err := s.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var keys []string
	if err := s.db.Raw(`SELECT key FROM login_throttles`).Scan(&keys).Error; err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "fresh" {
		t.Fatalf("rows after the purge = %v, want only fresh", keys)
	}
}
