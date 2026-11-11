package limits

import (
	"net/http/httptest"
	"testing"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// TestAnIPv6HostIsOneAddress pins what PerAddress keys on: a /64 is one
// key and IPv4 stays whole.
func TestAnIPv6HostIsOneAddress(t *testing.T) {
	key := func(remote string) string {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		return troncmiddleware.RemoteAddrKey64(r)
	}
	if key("[2001:db8:1:2::1]:443") != key("[2001:db8:1:2:ffff::9]:80") {
		t.Fatal("two addresses in one /64 got two budgets")
	}
	if key("[2001:db8:1:2::1]:443") == key("[2001:db8:1:3::1]:443") {
		t.Fatal("two /64s share a budget")
	}
	if key("192.0.2.1:1") == key("192.0.2.2:1") {
		t.Fatal("two IPv4 addresses share a budget")
	}
	if len(Bucket("any space id")) != 3 {
		t.Fatal("Bucket does not fold to 4096 buckets")
	}
}
