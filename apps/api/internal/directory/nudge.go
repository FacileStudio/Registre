package directory

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// nudgeTimeout bounds one nudge. A slow app must not hold anything up, and the
// app catches up on its own pull interval anyway.
const nudgeTimeout = 5 * time.Second

// Nudger tells every app that declared a directory sync URI that the spaces
// changed. It sends nothing but the POST itself: the app answers by pulling
// the snapshot with its own credentials, so a nudge needs no signature and a
// lost one costs at most one pull interval.
type Nudger struct {
	targets func(ctx context.Context) ([]schemas.Client, error)
	client  *http.Client
	logger  *slog.Logger
}

// NewNudger builds a nudger over a function listing the register's clients.
func NewNudger(targets func(ctx context.Context) ([]schemas.Client, error), logger *slog.Logger) *Nudger {
	return &Nudger{targets: targets, client: &http.Client{Timeout: nudgeTimeout}, logger: logger}
}

// Fire nudges every enabled app with a sync URI, in the background. It never
// blocks the change that caused it and never fails it.
func (n *Nudger) Fire() {
	if n == nil {
		return
	}
	go n.deliver(context.Background())
}

// deliver POSTs one nudge to each target and logs the ones that failed.
func (n *Nudger) deliver(ctx context.Context) {
	clients, err := n.targets(ctx)
	if err != nil {
		n.logger.Warn("registre: directory nudge skipped, cannot list apps", slog.Any("error", err))
		return
	}
	for _, client := range clients {
		if client.DirectorySyncURI == "" || client.DisabledAt != nil {
			continue
		}
		if err := n.post(ctx, client.DirectorySyncURI); err != nil {
			n.logger.Warn("registre: directory nudge failed",
				slog.String("client_id", client.ClientID), slog.Any("error", err))
		}
	}
}

// post sends one empty nudge.
func (n *Nudger) post(ctx context.Context, uri string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uri, nil)
	if err != nil {
		return err
	}
	response, err := n.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 300 {
		return &statusError{code: response.StatusCode}
	}
	return nil
}

// statusError is a nudge an app answered with a non-2xx status.
type statusError struct{ code int }

// Error describes the status.
func (e *statusError) Error() string { return "app answered " + http.StatusText(e.code) }
