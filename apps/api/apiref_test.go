package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/modules/docs"
	"github.com/FacileStudio/tronc/apiref"
	troncenv "github.com/FacileStudio/tronc/env"
	"github.com/go-chi/chi/v5"
)

func buildTestRouter(t *testing.T) chi.Router {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("testdb.Open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}

	root := make([]byte, 32)
	for i := range root {
		root[i] = byte(i + 1)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	verifier := credentials.New(2)

	cfg := env.Config{
		Core:            troncenv.Core{AppEnv: troncenv.Development},
		Issuer:          "http://localhost:8080",
		RotationEvery:   time.Hour,
		PublishDelay:    time.Minute,
		KeyRetention:    24 * time.Hour,
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}

	router, err := assemble(context.Background(), db, root, cfg, services{
		verifier: verifier,
		log:      logger,
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	return router
}

func TestRegistryIsComplete(t *testing.T) {
	if issues := apiref.Incomplete(
		docs.Reference(),
		"/oauth/v2/authorize",
		"/oauth/v2/sessions/end",
		"/api/device/authenticate",
		"/api/account/logout",
		"/api/account/totp/begin",
		"/api/account/totp/recovery-codes",
		"/api/admin/clients/{clientID}/secret",
		"/api/admin/users/{userID}/disable",
		"/oauth/v2/userinfo",
	); len(issues) > 0 {
		t.Errorf("incomplete documentation routes: %v", issues)
	}
}

func TestEveryRouteIsDocumented(t *testing.T) {
	router := buildTestRouter(t)
	if missing := apiref.Undocumented(router, docs.Reference(), append(zitadelOP(), "/version", "/api/version")...); len(missing) > 0 {
		t.Errorf("routes missing from the API registry: %v", missing)
	}
}

func TestReferenceIsServedAtDocs(t *testing.T) {
	router := buildTestRouter(t)

	page := httptest.NewRecorder()
	router.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("GET /docs = %d, want 200", page.Code)
	}

	spec := httptest.NewRecorder()
	router.ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil))
	if spec.Code != http.StatusOK {
		t.Fatalf("GET /docs/openapi.json = %d, want 200", spec.Code)
	}
	var document struct {
		OpenAPI string         `json:"openapi"`
		Paths   map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(spec.Body.Bytes(), &document); err != nil {
		t.Fatalf("spec is not JSON: %v", err)
	}
	if document.OpenAPI == "" || len(document.Paths) == 0 {
		t.Fatalf("spec is empty: %+v", document)
	}
}

// zitadelOP is where zitadel's provider handler is mounted for every HTTP
// method at once. The protocol endpoints are documented once, under the oidc
// module's /oauth/v2 paths; listing each method of each mount would document
// the library's router rather than registre's API.
func zitadelOP() []string {
	return []string{
		"/.well-known/openid-configuration", "/authorize", "/device_authorization", "/end_session",
		"/keys", "/oauth/introspect", "/oauth/token", "/revoke", "/userinfo",
	}
}
