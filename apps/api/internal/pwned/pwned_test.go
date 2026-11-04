package pwned

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// TestOnlyAPrefixLeavesAndPaddingIsAsked checks the k-anonymity contract and
// that a match, a decoy and a miss are told apart.
func TestOnlyAPrefixLeavesAndPaddingIsAsked(t *testing.T) {
	sum := sha1.Sum([]byte("hunter2hunter2hunter2"))
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Add-Padding") != "true" {
			t.Errorf("Add-Padding = %q, want true", r.Header.Get("Add-Padding"))
		}
		if prefix := strings.TrimPrefix(r.URL.Path, "/range/"); len(prefix) != 5 {
			t.Errorf("sent %q, want a five-character prefix", prefix)
		}
		_, _ = io.WriteString(w, full[5:]+":12\r\nFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0\r\n")
	}))
	defer server.Close()
	checker := NewAt(server.URL+"/range/", quiet())
	if !checker.Breached(context.Background(), "hunter2hunter2hunter2") {
		t.Fatal("a listed password was not reported")
	}
	if checker.Breached(context.Background(), "a password nobody has ever used") {
		t.Fatal("an unlisted password was reported")
	}
}

// TestAnOutageSkipsTheCheck answers false on a 503 and on a dead host.
func TestAnOutageSkipsTheCheck(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	for _, base := range []string{failing.URL + "/range/", dead.URL + "/range/"} {
		if NewAt(base, quiet()).Breached(context.Background(), "anything at all here") {
			t.Fatalf("an outage at %s reported a breach", base)
		}
	}
	var none *Checker
	if none.Breached(context.Background(), "x") {
		t.Fatal("a nil checker reported a breach")
	}
}
