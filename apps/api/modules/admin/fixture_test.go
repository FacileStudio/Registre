package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

var root = []byte("0123456789abcdef0123456789abcdef")

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

type fixture struct {
	router      chi.Router
	operator    schemas.User
	verifier    *credentials.Verifier
	register    *clients.Service
	users       *identity.Service
	invitations *invites.Service
	db          *gorm.DB
}

func newFixture(t *testing.T) fixture {
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
	verifier := credentials.New(2)
	register := clients.New(db, verifier)
	users := identity.New(db, verifier)
	keyring := keys.New(db, root, keys.Config{
		RotateEvery: time.Hour, PublishDelay: 10 * time.Minute,
		Retention: 2 * time.Hour,
	}, discard())
	if err := keyring.Ensure(context.Background()); err != nil {
		t.Fatalf("ensuring the keyring: %v", err)
	}
	invitations := invites.New(db, verifier)
	service := New(Services{
		Register: register, Users: users, Keys: keyring, Invites: invitations,
		Access: access.New(db, clients.SelfClientID), Audit: audit.New(db), Self: clients.SelfClientID,
		Eraser: gdpr.New(gdpr.Deps{DB: db, Sessions: users, Audit: audit.New(db), Logger: discard()}, "http://registre.test"),
	}, discard())
	router := chi.NewRouter()
	Mount(router, service)

	operator, err := users.Create(context.Background(), identity.NewUser{
		Email: "operator@example.com", Password: "operator password", Admin: true,
	})
	if err != nil {
		t.Fatalf("creating the operator: %v", err)
	}
	return fixture{router, operator, verifier, register, users, invitations, db}
}

func (f fixture) call(t *testing.T, method, path string, body any, auth bool) (*http.Response, map[string]any) {
	t.Helper()
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding the body: %v", err)
		}
		payload = bytes.NewReader(raw)
	}
	request := httptest.NewRequest(method, path, payload)
	if auth {
		request.SetBasicAuth("operator@example.com", "operator password")
	}
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	response := recorder.Result()
	var decoded map[string]any
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		decoded = nil
	}
	return response, decoded
}
