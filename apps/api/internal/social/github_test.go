package social

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func fakeGitHub(t *testing.T, tokenResp, userResp, emailsResp string) *GitHub {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tokenResp))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(userResp))
	})
	mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(emailsResp))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	g := NewGitHub("client-id", "client-secret")
	g.tokenURL, g.userURL, g.emailsURL = server.URL+"/token", server.URL+"/user", server.URL+"/emails"
	return g
}

func TestGitHubAuthURL(t *testing.T) {
	verifier, err := newVerifier()
	if err != nil {
		t.Fatal(err)
	}
	raw := NewGitHub("gh-id", "gh-secret").AuthURL("state-1", challenge(verifier), "https://sso.test/cb")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("the URL does not parse: %v", err)
	}
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != "https://github.com/login/oauth/authorize" {
		t.Errorf("authorize endpoint = %s", got)
	}
	query := parsed.Query()
	sum := sha256.Sum256([]byte(verifier))
	want := map[string]string{
		"client_id": "gh-id", "redirect_uri": "https://sso.test/cb",
		"scope": "read:user user:email", "state": "state-1", "code_challenge_method": "S256",
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	for key, value := range want {
		if query.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, query.Get(key), value)
		}
	}
	if strings.Contains(raw, "gh-secret") {
		t.Error("the client secret leaked into the authorize URL")
	}
}

func TestGitHubHeadersAndForm(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Header.Get("Accept") != "application/json" || r.PostForm.Get("code") != "the-code" {
			t.Errorf("token request unexpected: %v", r.PostForm)
		}
		_, _ = w.Write([]byte(`{"access_token":"gh_tok","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gh_tok" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Errorf("user headers auth=%s ver=%s", r.Header.Get("Authorization"), r.Header.Get("X-GitHub-Api-Version"))
		}
		_, _ = w.Write([]byte(`{"id":42,"login":"octo"}`))
	})
	mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("User-Agent") != "Registre" {
			t.Errorf("emails headers accept=%s ua=%s", r.Header.Get("Accept"), r.Header.Get("User-Agent"))
		}
		_, _ = w.Write([]byte(`[{"email":"octo@test.com","primary":true,"verified":true}]`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	g := NewGitHub("gh-id", "gh-sec")
	g.tokenURL, g.userURL, g.emailsURL = server.URL+"/token", server.URL+"/user", server.URL+"/emails"
	if _, err := g.Exchange(context.Background(), "the-code", "the-verifier", "https://sso.test/cb"); err != nil {
		t.Fatalf("exchange: %v", err)
	}
}

func TestGitHubExchangeSuccess(t *testing.T) {
	tests := []struct {
		name, user, emails string
		want               Profile
	}{
		{
			name:   "primary verified",
			user:   `{"id":42,"login":"octo","name":"Mona","avatar_url":"https://a.test/42"}`,
			emails: `[{"email":"alt@test.com","verified":true},{"email":"mona@test.com","primary":true,"verified":true}]`,
			want:   Profile{Subject: "42", Email: "mona@test.com", EmailVerified: true, Name: "Mona", AvatarURL: "https://a.test/42"},
		},
		{
			name:   "fallback to first verified",
			user:   `{"id":99,"login":"dev"}`,
			emails: `[{"email":"bad@test.com","primary":true,"verified":false},{"email":"dev@test.com","verified":true}]`,
			want:   Profile{Subject: "99", Email: "dev@test.com", EmailVerified: true, Name: "dev"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := fakeGitHub(t, `{"access_token":"tok","token_type":"bearer"}`, tc.user, tc.emails)
			got, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb")
			if err != nil || got != tc.want {
				t.Fatalf("exchange = %+v, %v, want %+v", got, err, tc.want)
			}
		})
	}
}

func TestGitHubExchangeUnverifiedEmail(t *testing.T) {
	for _, emails := range []string{
		`[]`,
		`[{"email":"bad@test.com","primary":true,"verified":false}]`,
	} {
		g := fakeGitHub(t, `{"access_token":"tok","token_type":"bearer"}`, `{"id":1,"login":"u"}`, emails)
		if _, err := g.Exchange(context.Background(), "c", "v", "https://sso.test/cb"); !errors.Is(err, ErrUnverifiedEmail) {
			t.Fatalf("expected ErrUnverifiedEmail, got %v", err)
		}
	}
}
