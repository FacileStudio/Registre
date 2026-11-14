package logout

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// lockedBuffer is a log sink the retry goroutine and the test read safely.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// receiver is an app's back-channel logout endpoint answering a scripted
// sequence of statuses, repeating the last one, and counting what it got.
type receiver struct {
	server *httptest.Server
	hits   atomic.Int32
}

func newReceiver(t *testing.T, statuses ...int) *receiver {
	t.Helper()
	r := &receiver{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := int(r.hits.Add(1))
		if req.PostFormValue("logout_token") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(statuses[min(n, len(statuses))-1])
	}))
	t.Cleanup(r.server.Close)
	return r
}

// awaitHits waits until the receiver saw want requests or the deadline passes.
func (r *receiver) awaitHits(want int32) int32 {
	deadline := time.Now().Add(5 * time.Second)
	for r.hits.Load() < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	return r.hits.Load()
}

// firing builds a Service over a test database holding one app whose logout
// URI points at uri, with the given retry delays.
func firing(t *testing.T, uri string, delays []time.Duration) (*Service, *lockedBuffer) {
	t.Helper()
	keyring, register := registering(t, uri)
	logs := &lockedBuffer{}
	service := New(keyring, register, "https://sso.example", slog.New(slog.NewTextHandler(logs, nil)))
	service.retryDelays = delays
	return service, logs
}

// registering opens a fresh test database with a signing key and one app
// whose back-channel logout URI is uri.
func registering(t *testing.T, uri string) (*keys.Service, *clients.Service) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	keyring := keys.New(db, []byte("0123456789abcdef0123456789abcdef"),
		keys.Config{RotateEvery: time.Hour, PublishDelay: time.Minute, Retention: 2 * time.Hour},
		slog.New(slog.DiscardHandler))
	if err := keyring.Ensure(t.Context()); err != nil {
		t.Fatalf("minting a signing key: %v", err)
	}
	register := clients.New(db, credentials.New(2))
	if _, err := register.Upsert(t.Context(), schemas.Client{
		ClientID: "flapping", Subject: "sub-flapping", Name: "Flapping", Kind: schemas.KindApplication,
		GrantTypes: schemas.StringList{"authorization_code"}, Scopes: schemas.StringList{"openid"},
		Audiences:    schemas.StringList{"flapping"},
		RedirectURIs: schemas.StringList{"https://flapping.example/callback"}, BackchannelLogoutURI: uri,
	}); err != nil {
		t.Fatalf("registering the app: %v", err)
	}
	return keyring, register
}

// TestFireRetriesATransientFailureInTheBackground proves a 503 is retried
// until the app answers 200, and that Fire returns after the first attempt.
func TestFireRetriesATransientFailureInTheBackground(t *testing.T) {
	app := newReceiver(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK)
	service, logs := firing(t, app.server.URL, []time.Duration{time.Second, time.Millisecond, time.Millisecond})
	if err := service.Fire(context.Background(), "user-1", []string{"flapping"}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if got := app.hits.Load(); got != 1 {
		t.Fatalf("Fire returned after %d deliveries, want 1: retries must not run inline", got)
	}
	if got := app.awaitHits(3); got != 3 {
		t.Fatalf("the app got %d deliveries, want 3", got)
	}
	time.Sleep(50 * time.Millisecond)
	if got := app.hits.Load(); got != 3 {
		t.Errorf("the app got %d deliveries after a 200, want the retries to stop at 3", got)
	}
	if strings.Contains(logs.String(), "gave up") {
		t.Errorf("a delivered logout logged a give-up:\n%s", logs)
	}
}

// TestFireDoesNotRetryAPermanentRefusal proves a 4xx is final: the app read
// the token and refused it, and a resend would be refused the same way.
func TestFireDoesNotRetryAPermanentRefusal(t *testing.T) {
	app := newReceiver(t, http.StatusUnauthorized)
	service, _ := firing(t, app.server.URL, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})
	if err := service.Fire(context.Background(), "user-1", []string{"flapping"}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := app.hits.Load(); got != 1 {
		t.Errorf("the app got %d deliveries for a 401, want exactly 1", got)
	}
}

// TestFireGivesUpAfterTheLastDelay proves the retries are bounded and that
// giving up is logged at Error for Journal.
func TestFireGivesUpAfterTheLastDelay(t *testing.T) {
	app := newReceiver(t, http.StatusBadGateway)
	service, logs := firing(t, app.server.URL, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	if err := service.Fire(ctx, "user-1", []string{"flapping"}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	cancel()
	if got := app.awaitHits(4); got != 4 {
		t.Fatalf("the app got %d deliveries, want 4 despite the caller's context ending", got)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "gave up") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	out := logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "back-channel logout gave up") ||
		!strings.Contains(out, "attempts=4") {
		t.Fatalf("no Error-level give-up with attempts=4 in the log:\n%s", out)
	}
	time.Sleep(50 * time.Millisecond)
	if got := app.hits.Load(); got != 4 {
		t.Errorf("the app got %d deliveries, want the retries to stop at 4", got)
	}
}

// TestFireRefusesToFollowARedirect proves a 307 is a final failure and the
// token never reaches the host the redirect names.
func TestFireRefusesToFollowARedirect(t *testing.T) {
	elsewhere := newReceiver(t, http.StatusOK)
	var hits atomic.Int32
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, elsewhere.server.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(app.Close)
	service, logs := firing(t, app.URL, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})
	if err := service.Fire(context.Background(), "user-1", []string{"flapping"}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := hits.Load(); got != 1 {
		t.Errorf("the app got %d deliveries for a 307, want exactly 1", got)
	}
	if got := elsewhere.hits.Load(); got != 0 {
		t.Errorf("the redirect target got %d deliveries, want none", got)
	}
	if !strings.Contains(logs.String(), "answered 307") {
		t.Errorf("no final 307 failure in the log:\n%s", logs)
	}
}

// TestShutdownAbandonsPendingRetries proves the end of the Service's lifetime
// stops a pending retry and logs it at Warn.
func TestShutdownAbandonsPendingRetries(t *testing.T) {
	app := newReceiver(t, http.StatusServiceUnavailable)
	service, logs := firing(t, app.server.URL, []time.Duration{time.Hour, time.Hour, time.Hour})
	lifetime, shutdown := context.WithCancel(context.Background())
	service.WithLifetime(lifetime)
	if err := service.Fire(context.Background(), "user-1", []string{"flapping"}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	shutdown()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "abandoned at shutdown") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	out := logs.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "retry abandoned at shutdown") ||
		!strings.Contains(out, "attempts=1") {
		t.Fatalf("no Warn-level abandon with attempts=1 in the log:\n%s", out)
	}
	if got := app.hits.Load(); got != 1 {
		t.Errorf("the app got %d deliveries, want only the first", got)
	}
}

// TestJitterStaysWithinTwentyPercent bounds the spread of every retry delay.
func TestJitterStaysWithinTwentyPercent(t *testing.T) {
	for range 1000 {
		if got := jitter(10 * time.Second); got < 8*time.Second || got > 12*time.Second {
			t.Fatalf("jitter(10s) = %v, want within 8s..12s", got)
		}
	}
}
