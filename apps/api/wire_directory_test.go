package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	spacesdir "github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
	"github.com/FacileStudio/Registre/apps/api/schemas"
	jose "github.com/go-jose/go-jose/v4"
)

// TestWireDirectory pins what every app polls: the space snapshot and the
// erasure feed, with their refusals.
func TestWireDirectory(t *testing.T) {
	w := newWire(t)
	w.seedClients()
	var owner schemas.User
	if err := w.db.Where("email = ?", "golden@facile.studio").First(&owner).Error; err != nil {
		t.Fatal(err)
	}
	space, err := spacesdir.New(w.db, nil, nil, slog.Default()).Create(context.Background(), owner.ID, "Golden")
	if err != nil {
		t.Fatal(err)
	}
	w.hide(space.ID, "{space}")
	stamp := time.Date(2026, 11, 1, 12, 0, 0, 123456000, time.UTC)
	for _, subject := range []string{"erased-one", "erased-two"} {
		w.db.Exec(`INSERT INTO deletion_notices (client_id, subject, created_at) VALUES ('golden-app', ?, ?)`,
			subject, stamp)
	}
	w.basic("snapshot", "/api/directory/snapshot", "golden-app")
	w.basic("snapshot without credentials", "/api/directory/snapshot", "")
	w.basic("deletions", "/api/directory/deletions?since=2026-10-01T00:00:00Z", "golden-app")
	w.basic("deletions after the last", "/api/directory/deletions?since=2026-11-02T00:00:00Z", "golden-app")
	w.basic("deletions bad since", "/api/directory/deletions?since=yesterday", "golden-app")
	w.basic("deletions bad cursor", "/api/directory/deletions?cursor=%%%", "golden-app")
	w.basic("deletions without credentials", "/api/directory/deletions", "")
	w.basic("deletions wrong secret", "/api/directory/deletions", "golden-app:wrong")
	w.basic("deletions as a service account", "/api/directory/deletions", "golden-agent")
	w.golden("directory")
}

// basic sends a GET authenticated as client (or client:secret) with HTTP
// Basic, or anonymously when client is empty.
func (w *wire) basic(label, path, client string) {
	request, _ := http.NewRequest(http.MethodGet, w.url+path, nil)
	if client != "" {
		id, secret, found := strings.Cut(client, ":")
		if !found {
			secret = wireSecret
		}
		request.SetBasicAuth(id, secret)
	}
	w.do(label, request)
}

// TestWireDeletionPush pins the signed user.deleted event an app's
// deletion_uri receives, header and claims byte for byte, and proves it
// verifies against the key set registre serves.
func TestWireDeletionPush(t *testing.T) {
	w := newWire(t)
	w.seedClients()
	received := make(chan *http.Request, 1)
	bodies := make(chan string, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- r
		bodies <- string(body)
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer receiver.Close()
	keyring := keys.New(w.db, wireRoot(), keys.Config{RotateEvery: time.Hour, PublishDelay: time.Minute,
		Retention: 24 * time.Hour}, slog.Default())
	pusher := logout.New(keyring, nil, w.url, slog.Default())
	if err := pusher.DeliverDeleted(context.Background(), "erased-one", "golden-app", receiver.URL+"/api/registre/deletions"); err != nil {
		t.Fatalf("DeliverDeleted: %v", err)
	}
	w.hideKeys()
	w.recordPush(<-received, <-bodies)
	w.golden("deletion-push")
}

// recordPush writes the push's request line, type and decoded JOSE header
// and claims to the transcript, then checks its signature.
func (w *wire) recordPush(request *http.Request, token string) {
	w.t.Helper()
	w.transcript.WriteString(request.Method + " " + request.URL.RequestURI() + "\n")
	w.transcript.WriteString("Content-Type: " + request.Header.Get("Content-Type") + "\n\n")
	for _, part := range strings.Split(token, ".")[:2] {
		decoded, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			w.t.Fatalf("decoding the token: %v", err)
		}
		w.transcript.WriteString(string(decoded) + "\n")
	}
	w.verifies(token)
}

// verifies checks a token against the key set /keys serves.
func (w *wire) verifies(token string) {
	w.t.Helper()
	signed, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		w.t.Fatalf("parsing the push: %v", err)
	}
	request, _ := http.NewRequest(http.MethodGet, w.url+"/keys", nil)
	response, err := w.client.Do(request)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var set jose.JSONWebKeySet
	if err := json.NewDecoder(response.Body).Decode(&set); err != nil {
		w.t.Fatal(err)
	}
	if _, err := signed.Verify(&set); err != nil {
		w.t.Errorf("the push does not verify against /keys: %v", err)
	}
}
