package netguard

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// Control is a net.Dialer Control hook: it refuses the connection when the
// address the socket is about to connect to is internal, whatever name led
// there.
func Control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || IsInternal(ap.Addr()) {
		return block(CodeInternal, "connection to an internal address refused")
	}
	return nil
}

// DialContext resolves and checks the host in address, then connects to the
// checked address itself, so a DNS answer that changes between the check and
// the connection (rebinding) is never used.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, block(CodeURL, "invalid address")
	}
	ip, err := g.Resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	target := net.JoinHostPort(ip.String(), port)
	if g.Dial != nil {
		return g.Dial(ctx, network, target)
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: Control}
	return dialer.DialContext(ctx, network, target)
}

// Client returns an HTTP client that ignores proxy settings, dials only
// through the guard and follows no redirect. roots is nil in production; a
// test passes its own TLS server's pool.
func (g *Guard) Client(timeout time.Duration, roots *tls.Config) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         g.DialContext,
			TLSClientConfig:     roots,
			TLSHandshakeTimeout: timeout,
			MaxIdleConns:        8,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("netguard: redirects are not followed")
		},
	}
}
