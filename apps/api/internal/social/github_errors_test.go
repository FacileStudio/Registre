package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func statusServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGitHubExchange500Errors(t *testing.T) {
	server := statusServer(t, http.StatusInternalServerError)
	for _, target := range []string{"token", "user", "emails"} {
		g := fakeGitHub(t, `{"access_token":"tok","token_type":"bearer"}`, `{"id":1,"login":"u"}`, `[{"email":"a@b.com","verified":true,"primary":true}]`)
		switch target {
		case "token":
			g.tokenURL = server.URL
		case "user":
			g.userURL = server.URL
		case "emails":
			g.emailsURL = server.URL
		}
		if _, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb"); err == nil {
			t.Fatalf("expected error on 500 for %s", target)
		}
	}
}

func TestGitHubExchangeTokenErrors(t *testing.T) {
	for _, tokenBody := range []string{
		`{"error":"bad_verification_code"}`,
		`{"access_token":"","token_type":"bearer"}`,
		`{"access_token":"tok","token_type":"mac"}`,
	} {
		g := fakeGitHub(t, tokenBody, `{"id":1,"login":"u"}`, `[]`)
		if _, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb"); err == nil {
			t.Fatalf("expected error for token body %s", tokenBody)
		}
	}
}

func TestGitHubExchangeNonPositiveUserID(t *testing.T) {
	for _, userBody := range []string{
		`{"id":0,"login":"u"}`,
		`{"id":-1,"login":"u"}`,
	} {
		g := fakeGitHub(t, `{"access_token":"tok","token_type":"bearer"}`, userBody, `[{"email":"a@b.com","verified":true,"primary":true}]`)
		if _, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb"); err == nil {
			t.Fatalf("expected error for user body %s", userBody)
		}
	}
}

func TestGitHubExchangeTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
	}))
	t.Cleanup(server.Close)
	g := NewGitHub("id", "sec")
	g.tokenURL = server.URL
	g.client = &http.Client{Timeout: 5 * time.Millisecond}
	if _, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb"); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGitHubExchangeSizeCapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pad := strings.Repeat("x", maxProviderBody+100)
		_, _ = w.Write([]byte(`{"access_token":"` + pad + `"}`))
	}))
	t.Cleanup(server.Close)
	g := NewGitHub("id", "sec")
	g.tokenURL = server.URL
	if _, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb"); err == nil {
		t.Fatal("expected error due to size capping")
	}
}
