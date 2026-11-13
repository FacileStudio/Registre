package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	troncenv "github.com/FacileStudio/tronc/env"
	"gorm.io/gorm"
)

// wire is a whole assembled registre served over real HTTP, for the golden
// tests that pin what relying parties and apps read: status, headers and body,
// byte for byte, with only the values that are random per run masked.
// WIRE_UPDATE=1 rewrites the files under testdata/wire instead of comparing.
type wire struct {
	t          *testing.T
	db         *gorm.DB
	url        string
	client     *http.Client
	masks      []string
	transcript strings.Builder

	lastLocation *url.URL
}

// wireIgnored are the response headers whose value changes on every request.
var wireIgnored = map[string]bool{"Date": true, "X-Request-Id": true, "Content-Length": true}

// wireScrubs mask what is random per run but whose position is fixed.
var wireScrubs = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`\b(code|auth_request|id)=[A-Za-z0-9_.~%-]+`), "$1={random}"},
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`), "{jwt}"},
	{regexp.MustCompile(`"(iat|exp|nbf|auth_time|expires_in)":\d+`), `"$1":0`},
	{regexp.MustCompile(`"jti":"[^"]+"`), `"jti":"{jti}"`},
	{regexp.MustCompile(`registre_flow_[0-9a-f]+`), "registre_flow_{digest}"},
	{regexp.MustCompile(`(Set-Cookie: [^=]+)=[^;]*`), "$1={value}"},
	{regexp.MustCompile(`Expires=[^;]+`), "Expires={time}"},
}

// init pins time.Local to UTC, the zone of the distroless production image:
// timestamps read back from Postgres carry the process zone, and a golden
// recorded on a +01:00 laptop failed in CI and pinned an offset production
// never sends. It runs before any test so no background goroutine a test
// leaves behind can read the zone while a later test rewrites it.
func init() { time.Local = time.UTC }

// newWire boots registre on a fresh schema with the budgets opened wide, so
// a golden run never meets a rate limit.
func newWire(t *testing.T) *wire {
	t.Helper()
	db := wireDB(t)
	slog.SetDefault(slog.New(slog.DiscardHandler))
	server := httptest.NewUnstartedServer(nil)
	issuer := "http://" + server.Listener.Addr().String()
	router, err := assemble(context.Background(), db, wireRoot(), wireConfig(issuer),
		services{verifier: credentials.New(2), log: slog.Default()})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	server.Config.Handler = router
	server.Start()
	t.Cleanup(server.Close)
	jar, _ := cookiejar.New(nil)
	stop := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	w := &wire{t: t, db: db, url: issuer, client: &http.Client{Jar: jar, CheckRedirect: stop}}
	w.hide(issuer, "{issuer}")
	w.hide(url.QueryEscape(issuer), "{issuer}")
	return w
}

// wireDB is a fresh schema on the test database, or a skip without one.
func wireDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(dsn)
	if err != nil || testdb.Truncate(db) != nil {
		t.Fatalf("preparing the test database: %v", err)
	}
	return db
}

// wireConfig is a development instance at issuer with every budget wide open.
func wireConfig(issuer string) env.Config {
	open := env.Hardening{LoginRate: 1000, LoginBurst: 1000, TokenRate: 1000, TokenBurst: 1000,
		PublicTokenRate: 1000, PublicTokenBurst: 1000}
	return env.Config{Core: troncenv.Core{AppEnv: troncenv.Development}, Issuer: issuer,
		RotationEvery: time.Hour, PublishDelay: time.Minute, KeyRetention: 24 * time.Hour,
		AccessTokenTTL: time.Hour, RefreshTokenTTL: 30 * 24 * time.Hour, Hardening: open}
}

// wireRoot is the throwaway root key the signing keys are sealed under.
func wireRoot() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

// hide masks every later occurrence of a value random per run.
func (w *wire) hide(value, name string) {
	if value != "" {
		w.masks = append(w.masks, value, name)
	}
}

// do sends one request, appends its masked exchange to the transcript and
// returns the raw body for the caller to read values out of.
func (w *wire) do(label string, request *http.Request) (int, []byte) {
	w.t.Helper()
	response, err := w.client.Do(request)
	if err != nil {
		w.t.Fatalf("%s: %v", label, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		w.t.Fatalf("%s: reading the body: %v", label, err)
	}
	if location, err := response.Location(); err == nil {
		w.lastLocation = location
	}
	names := make([]string, 0, len(response.Header))
	for name := range response.Header {
		if !wireIgnored[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	w.transcript.WriteString("### " + label + "\n" + request.Method + " " + request.URL.RequestURI() + "\n")
	w.transcript.WriteString(response.Status + "\n")
	for _, name := range names {
		for _, value := range response.Header[name] {
			w.transcript.WriteString(name + ": " + value + "\n")
		}
	}
	w.transcript.WriteString("\n" + string(body) + "\n\n")
	return response.StatusCode, body
}

// golden compares the masked transcript with testdata/wire/<name>.golden.
func (w *wire) golden(name string) {
	w.t.Helper()
	got := w.scrub(w.transcript.String())
	path := filepath.Join("testdata", "wire", name+".golden")
	if os.Getenv("WIRE_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			w.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			w.t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		w.t.Fatalf("reading %s (WIRE_UPDATE=1 writes it): %v", path, err)
	}
	if got != string(want) {
		w.t.Errorf("%s changed; diff it against WIRE_UPDATE=1 output:\n%s", path, got)
	}
}

// scrub applies the value masks, then the positional ones.
func (w *wire) scrub(text string) string {
	text = strings.NewReplacer(w.masks...).Replace(text)
	for _, scrub := range wireScrubs {
		text = scrub.pattern.ReplaceAllString(text, scrub.with)
	}
	return text
}
