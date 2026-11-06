package social

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDiscordAuthURLCarriesStateScopesAndAnS256Challenge(t *testing.T) {
	verifier, err := newVerifier()
	if err != nil {
		t.Fatal(err)
	}
	raw := NewDiscord("app-id", "app-secret").AuthURL("the-state", challenge(verifier), "https://sso.test/cb")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("the URL does not parse: %v", err)
	}
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != "https://discord.com/oauth2/authorize" {
		t.Errorf("authorize endpoint = %s", got)
	}
	query := parsed.Query()
	sum := sha256.Sum256([]byte(verifier))
	want := map[string]string{
		"client_id": "app-id", "redirect_uri": "https://sso.test/cb", "response_type": "code",
		"scope": "identify email", "state": "the-state", "code_challenge_method": "S256",
		"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	for key, value := range want {
		if query.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, query.Get(key), value)
		}
	}
	if strings.Contains(raw, "app-secret") {
		t.Error("the client secret leaked into the authorize URL")
	}
}

func fakeDiscord(t *testing.T, profile string) *Discord {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token form: %v", err)
		}
		for key, want := range map[string]string{
			"grant_type": "authorization_code", "code": "the-code", "code_verifier": "the-verifier",
			"redirect_uri": "https://sso.test/cb", "client_id": "app-id", "client_secret": "app-secret",
		} {
			if r.PostForm.Get(key) != want {
				t.Errorf("token %s = %q, want %q", key, r.PostForm.Get(key), want)
			}
		}
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer"}`))
	})
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("profile authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(profile))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	d := NewDiscord("app-id", "app-secret")
	d.tokenURL, d.userinfoURL = server.URL+"/token", server.URL+"/me"
	return d
}

func TestDiscordExchangeMapsTheProfile(t *testing.T) {
	d := fakeDiscord(t, `{"id":"123456789012345678","username":"yann","global_name":"Yann D","avatar":"abc",
		"email":"Yann@Facile.Studio","verified":true}`)
	got, err := d.Exchange(context.Background(), "the-code", "the-verifier", "https://sso.test/cb")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	want := Profile{
		Subject: "123456789012345678", Email: "yann@facile.studio", EmailVerified: true, Name: "Yann D",
		AvatarURL: "https://cdn.discordapp.com/avatars/123456789012345678/abc.png",
	}
	if got != want {
		t.Errorf("profile = %+v, want %+v", got, want)
	}
}

func TestDiscordExchangeRefusesAnIdThatIsNotASnowflake(t *testing.T) {
	d := fakeDiscord(t, `{"id":"../admin","username":"x"}`)
	if _, err := d.Exchange(context.Background(), "the-code", "the-verifier", "https://sso.test/cb"); err == nil {
		t.Fatal("a non-numeric Discord id was accepted")
	}
}
