// Package netguard refuses URLs, names and connections that would reach an
// internal network: loopback, private, link-local, CGNAT, ULA, multicast,
// reserved space and internal host names. It backs every server-side fetch of
// a URL a client supplied, such as a client ID metadata document. Copied from
// sonar internal/netguard.
package netguard

import (
	"net/netip"
)

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/96", "::ffff:0:0:0/96", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32",
	"3fff::/20", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8",
)

var (
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour = netip.MustParsePrefix("2002::/16")
)

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}

// IsInternal reports whether addr must never be reached from a public-only
// fetch. IPv4-mapped, NAT64 and 6to4 addresses are judged by the IPv4
// address they embed. An invalid address counts as internal.
func IsInternal(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap().WithZone("")
	if embedded, ok := embeddedV4(addr); ok {
		return IsInternal(embedded)
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() || addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func embeddedV4(addr netip.Addr) (netip.Addr, bool) {
	b := addr.As16()
	switch {
	case !addr.Is6():
		return netip.Addr{}, false
	case nat64.Contains(addr):
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	case sixToFour.Contains(addr):
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
	}
	return netip.Addr{}, false
}
