package metadoc

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/netguard"
)

func TestValidateKeepsOnlyAPublicCodeFlowClient(t *testing.T) {
	ok := Metadata{RedirectURIs: []string{"https://claude.ai/api/mcp/auth_callback", "http://localhost/callback",
		"http://127.0.0.1/callback", "com.example.app:/oauth"}, ClientName: "  Claude‮\u0000 "}
	got, err := Validate(ok)
	if err != nil {
		t.Fatalf("a valid client was refused: %v", err)
	}
	if got.ApplicationType != "native" || got.TokenEndpointAuthMethod != "none" || got.ClientName != "Claude" {
		t.Errorf("defaults = %+v", got)
	}
	refused := map[string]Metadata{
		"a secret":             {RedirectURIs: []string{"https://a.test/cb"}, ClientSecret: "x"},
		"a secret auth method": {RedirectURIs: []string{"https://a.test/cb"}, TokenEndpointAuthMethod: "client_secret_basic"},
		"the implicit grant":   {RedirectURIs: []string{"https://a.test/cb"}, GrantTypes: []string{"implicit"}},
		"a token response":     {RedirectURIs: []string{"https://a.test/cb"}, ResponseTypes: []string{"token"}},
		"no redirect":          {},
		"plain http elsewhere": {RedirectURIs: []string{"http://evil.test/cb"}},
		"a fragment":           {RedirectURIs: []string{"https://a.test/cb#x"}},
		"a web custom scheme":  {RedirectURIs: []string{"com.example.app:/cb"}, ApplicationType: "web"},
		"a bare custom scheme": {RedirectURIs: []string{"javascript:alert(1)"}},
		"an unknown app type":  {RedirectURIs: []string{"https://a.test/cb"}, ApplicationType: "browser"},
		"a relative redirect":  {RedirectURIs: []string{"/cb"}},
		"credentials in a URI": {RedirectURIs: []string{"https://u:p@a.test/cb"}},
	}
	for name, metadata := range refused {
		if _, err := Validate(metadata); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Validate = %v, want ErrInvalid", name, err)
		}
	}
}

func TestIsDocumentURL(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://claude.ai/oauth/claude-code-client-metadata": true,
		"https://a.test/":            false,
		"https://a.test":             false,
		"http://a.test/client":       false,
		"https://a.test/client?x=1":  false,
		"https://a.test/client#x":    false,
		"https://u@a.test/client":    false,
		"https://a.test/./client":    false,
		"https://a.test/x/../client": false,
		"bulle":                      false,
	} {
		if got := IsDocumentURL(raw); got != want {
			t.Errorf("IsDocumentURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestFreshnessFollowsCacheControlWithinBounds(t *testing.T) {
	for header, want := range map[string]time.Duration{
		"":                   DefaultCache,
		"max-age=600":        10 * time.Minute,
		"public, max-age=10": MinCache,
		"max-age=9999999":    MaxCache,
		"no-store":           MinCache,
		"max-age=nonsense":   DefaultCache,
	} {
		if got := freshness(header); got != want {
			t.Errorf("freshness(%q) = %s, want %s", header, got, want)
		}
	}
}

type hosts map[string]string

func (h hosts) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if address, ok := h[host]; ok {
		return []netip.Addr{netip.MustParseAddr(address)}, nil
	}
	return nil, errors.New("no such host")
}

// documentServer serves body at /client over TLS and returns a fetcher whose
// guard resolves example.com to a public address and dials the test server
// instead, so the SSRF guard runs exactly as in production.
func documentServer(t *testing.T, body string) (*Fetcher, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		_, _ = w.Write([]byte(strings.ReplaceAll(body, "SELF", "https://example.com"+r.URL.Path)))
	}))
	t.Cleanup(server.Close)
	guard := &netguard.Guard{
		Resolver: hosts{"example.com": "93.184.215.14", "internal.example.org": "10.0.0.5"},
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	roots := server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	return &Fetcher{Client: guard.Client(5*time.Second, &tls.Config{RootCAs: roots})}, "https://example.com/client"
}

func TestFetchReadsAValidDocument(t *testing.T) {
	fetcher, url := documentServer(t, `{"client_id":"SELF","client_name":"Test","redirect_uris":["http://localhost/cb"]}`)
	got, fresh, err := fetcher.Fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("Fetch = %v", err)
	}
	if got.ClientName != "Test" || time.Until(fresh) < 9*time.Minute {
		t.Errorf("Fetch = %+v fresh until %s", got, fresh)
	}
}

func TestFetchRefusesAMismatchOrAnInternalHost(t *testing.T) {
	fetcher, url := documentServer(t, `{"client_id":"https://other.test/client","redirect_uris":["http://localhost/cb"]}`)
	if _, _, err := fetcher.Fetch(context.Background(), url); !errors.Is(err, ErrInvalid) {
		t.Errorf("a document naming another client_id: %v", err)
	}
	if _, _, err := fetcher.Fetch(context.Background(), "https://internal.example.org/client"); err == nil {
		t.Error("a host resolving to a private address was fetched")
	} else if blocked, ok := netguard.AsBlocked(err); !ok || blocked.Code != netguard.CodeInternal {
		t.Errorf("the refusal is %v, want the guard's", err)
	}
	if _, _, err := fetcher.Fetch(context.Background(), "https://127.0.0.1/client"); err == nil {
		t.Error("a loopback literal was fetched")
	}
	big, url := documentServer(t, `{"client_id":"SELF","client_name":"`+strings.Repeat("x", MaxDocument)+`"}`)
	if _, _, err := big.Fetch(context.Background(), url); !errors.Is(err, ErrInvalid) {
		t.Errorf("an oversized document: %v", err)
	}
}
