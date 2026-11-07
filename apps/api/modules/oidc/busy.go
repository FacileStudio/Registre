package oidc

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
)

type busyKey struct{}

// busyAs503 turns the library's refusal into 503 when the refusal was caused by
// a saturated verifier.
//
// zitadel/oidc maps any storage error at client authentication to
// invalid_client and writes it itself, so the storage cannot choose the
// status. The storage records ErrBusy on a flag carried by the request context
// and this wrapper rewrites the status line before the library's body goes
// out. A request that was not busy passes through untouched.
func busyAs503(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flag := new(atomic.Bool)
		ctx := context.WithValue(r.Context(), busyKey{}, flag)
		next.ServeHTTP(&busyWriter{ResponseWriter: w, busy: flag}, r.WithContext(ctx))
	})
}

func noteBusy(ctx context.Context, err error) {
	if !errors.Is(err, credentials.ErrBusy) {
		return
	}
	if flag, ok := ctx.Value(busyKey{}).(*atomic.Bool); ok {
		flag.Store(true)
	}
}

type busyWriter struct {
	http.ResponseWriter
	busy    *atomic.Bool
	swapped bool
	started bool
}

func (b *busyWriter) WriteHeader(status int) {
	if b.started {
		return
	}
	b.started = true
	if !b.busy.Load() {
		b.ResponseWriter.WriteHeader(status)
		return
	}
	b.swapped = true
	header := b.ResponseWriter.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Retry-After", "2")
	header.Del("Content-Length")
	b.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
	_, _ = b.ResponseWriter.Write([]byte(`{"error":"temporarily_unavailable"}` + "\n"))
}

func (b *busyWriter) Write(p []byte) (int, error) {
	if !b.started {
		b.WriteHeader(http.StatusOK)
	}
	if b.swapped {
		return len(p), nil
	}
	return b.ResponseWriter.Write(p)
}
