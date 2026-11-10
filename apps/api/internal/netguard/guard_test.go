package netguard

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func codeOf(err error) string {
	if b, ok := AsBlocked(err); ok {
		return b.Code
	}
	if err != nil {
		return "other"
	}
	return ""
}

func TestResolveRefusesAnyInternalAnswer(t *testing.T) {
	g := testGuard()
	cases := map[string]string{
		"public.test":  "",
		"mixed.test":   CodeInternal,
		"private.test": CodeInternal,
		"meta.test":    CodeInternal,
		"nowhere.test": CodeUnresolved,
		"glouton-db":   CodeURL,
	}
	for host, want := range cases {
		_, err := g.Resolve(context.Background(), host)
		if got := codeOf(err); got != want {
			t.Errorf("%s: got %q, want %q", host, got, want)
		}
	}
}

func TestControlChecksTheConnectedAddress(t *testing.T) {
	for _, a := range []string{"127.0.0.1:80", "[::1]:443", "10.0.0.1:6379", "169.254.169.254:80", "junk"} {
		if Control("tcp", a, nil) == nil {
			t.Errorf("%s should be refused", a)
		}
	}
	if err := Control("tcp4", fakePublic+":443", nil); err != nil {
		t.Errorf("public address refused: %v", err)
	}
}

func TestDefaultDialerRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	_, err := (&Guard{}).DialContext(context.Background(), "tcp", srv.Listener.Addr().String())
	if codeOf(err) != CodeInternal {
		t.Fatalf("dialer reached loopback: %v", err)
	}
}

func TestClientFollowsNoRedirect(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "public page") })
	mux.HandleFunc("/away", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://public.test:"+port+"/ok", http.StatusFound)
	})
	client := testGuard().Client(5*1e9, nil)
	response, err := client.Get("http://public.test:" + port + "/ok")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("a public page through the guard: %v %v", response, err)
	}
	_ = response.Body.Close()
	if response, err := client.Get("http://public.test:" + port + "/away"); err == nil {
		_ = response.Body.Close()
		t.Fatal("the client followed a redirect")
	}
	if _, err := client.Get("http://private.test:" + port + "/ok"); codeOf(err) != CodeInternal {
		t.Fatalf("a private name was reached: %v", err)
	}
}
