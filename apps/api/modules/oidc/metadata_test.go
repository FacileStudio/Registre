package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAuthorizationServerMetadataIsTheHonestDiscoveryDocument(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	metadata := fetchDocument(t, fixture.issuer+MetadataPath)
	discovery := fetchDocument(t, fixture.issuer+"/.well-known/openid-configuration")

	if metadata["issuer"] != fixture.issuer {
		t.Errorf("issuer = %v, want %s: RFC 8414 §3.3 requires the exact issuer", metadata["issuer"], fixture.issuer)
	}
	if metadata["authorization_response_iss_parameter_supported"] != true {
		t.Error("RFC 9207 support is not advertised")
	}
	if methods := strings.Join(texts(metadata["code_challenge_methods_supported"]), " "); methods != "S256" {
		t.Errorf("code_challenge_methods_supported = %q, want S256 alone", methods)
	}
	if types := strings.Join(texts(metadata["response_types_supported"]), " "); types != "code" {
		t.Errorf("response_types_supported = %q, want code alone", types)
	}
	if !strings.Contains(strings.Join(texts(metadata["token_endpoint_auth_methods_supported"]), " "), "none") {
		t.Error("public clients (auth method none) are not advertised")
	}
	if metadata["token_endpoint"] != discovery["token_endpoint"] || metadata["jwks_uri"] != discovery["jwks_uri"] {
		t.Error("the two documents disagree about where the endpoints are")
	}
}

func TestTheTokenEndpointAnswersACrossOriginPreflight(t *testing.T) {
	guarded := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	}
	router := chi.NewRouter()
	router.Use(CORS([]string{"/oauth/token"}, guarded))
	router.Post("/oauth/token", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	router.Post("/api/account", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	preflight := httptest.NewRequest(http.MethodOptions, "/oauth/token", nil)
	preflight.Header.Set("Origin", "http://localhost:6274")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, preflight)
	if recorder.Code != http.StatusNoContent || recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight = %d with origin %q, want 204 and *",
			recorder.Code, recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Error("the public endpoints must never allow credentials")
	}

	private := httptest.NewRequest(http.MethodPost, "/api/account", nil)
	private.Header.Set("Origin", "http://localhost:6274")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, private)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("registre's own API went through the public policy: %d", recorder.Code)
	}
}

func TestPublicClientsHaveABudgetOfTheirOwn(t *testing.T) {
	f := newFixture(t, defaultKeyConfig())
	router := chi.NewRouter()
	MountWithLimits(router, f.provider, Binding{Storage: f.storage}, Limits{
		Token: 1, TokenBurst: 1, PublicToken: 1, PublicTokenBurst: 3,
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	call := func(withSecret bool) int {
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/oauth/token",
			strings.NewReader("grant_type=refresh_token&client_id=nobody&refresh_token=x"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if withSecret {
			request.SetBasicAuth("nobody", "wrong")
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		return response.StatusCode
	}
	if call(true) == http.StatusTooManyRequests || call(true) != http.StatusTooManyRequests {
		t.Fatal("the secret budget of burst 1 should refuse the second call")
	}
	for i := range 3 {
		if status := call(false); status == http.StatusTooManyRequests {
			t.Fatalf("public call %d was throttled by the secret budget", i+1)
		}
	}
	if call(false) != http.StatusTooManyRequests {
		t.Error("the public budget of burst 3 should refuse the fourth call")
	}
}

// fetchDocument GETs a JSON document and decodes it.
func fetchDocument(t *testing.T, url string) map[string]any {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("GET %s = %d %s", url, response.StatusCode, response.Header.Get("Content-Type"))
	}
	doc := map[string]any{}
	if err := json.NewDecoder(response.Body).Decode(&doc); err != nil {
		t.Fatalf("decoding %s: %v", url, err)
	}
	return doc
}

// texts reads a JSON array of strings.
func texts(value any) []string {
	return audiences(value)
}
