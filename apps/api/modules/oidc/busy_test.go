package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/go-chi/chi/v5"
)

func TestABusyVerifierBecomes503OnTheTokenEndpoint(t *testing.T) {
	handler := busyAs503(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noteBusy(r.Context(), fmt.Errorf("clients: %w", credentials.ErrBusy))
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/oauth/token", nil))

	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("answer = %d Retry-After %q, want 503 with a Retry-After", recorder.Code, recorder.Header().Get("Retry-After"))
	}
	if body := recorder.Body.String(); body != "{\"error\":\"temporarily_unavailable\"}\n" {
		t.Fatalf("body = %q, want the library's invalid_client swallowed", body)
	}
}

func TestAnOrdinaryRefusalPassesThroughUntouched(t *testing.T) {
	handler := busyAs503(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noteBusy(r.Context(), context.Canceled)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/oauth/token", nil))

	if recorder.Code != http.StatusUnauthorized || recorder.Body.String() != `{"error":"invalid_client"}` {
		t.Fatalf("answer = %d %q, want the original 401", recorder.Code, recorder.Body.String())
	}
}

func TestTheTokenEndpointHasAPerAddressBudget(t *testing.T) {
	f := newFixture(t, defaultKeyConfig())
	router := chi.NewRouter()
	MountWithTokenLimit(router, f.provider, Binding{Storage: f.storage}, 1.0/3600, 2)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	statuses := map[int]int{}
	for range 4 {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/oauth/token",
			strings.NewReader("grant_type=client_credentials"))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.SetBasicAuth("nobody", "wrong")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		statuses[response.StatusCode]++
	}
	if statuses[http.StatusTooManyRequests] != 2 {
		t.Fatalf("statuses %v: a burst of 2 should leave exactly two of four calls refused with 429", statuses)
	}
}
