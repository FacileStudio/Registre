package login

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The credential endpoint's budget is enforced before anything touches the
// database or argon2: a flood of guesses must cost the flooder nothing but
// 429s, not 64 MiB of hashing each.
func TestTheLoginEndpointRatelimitsPerAddress(t *testing.T) {
	router := chi.NewRouter()
	MountWithLimits(router, &Service{}, 1, 2)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	for i := 0; i < 2; i++ {
		response, err := postLogin(server.URL)
		if err != nil {
			t.Fatalf("POST %d: %v", i, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("request %d = %d, want 400 from the malformed body", i+1, response.StatusCode)
		}
	}
	response, err := postLogin(server.URL)
	if err != nil {
		t.Fatalf("the refused request: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request 3 = %d, want 429", response.StatusCode)
	}
	if response.Header.Get("Retry-After") == "" {
		t.Error("a refusal without Retry-After leaves the client guessing when to come back")
	}
}

// postLogin sends one malformed login with the CSRF header the route wants.
func postLogin(base string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodPost, base+"/api/login", strings.NewReader("{"))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Facile-CSRF", "1")
	return http.DefaultClient.Do(request)
}
