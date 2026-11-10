package metadoc

import (
	"errors"
	"net"
	"strings"
)

// IsLoopback reports whether host is localhost or a loopback address. The
// consent screen warns when every redirect URI is one: any program on the
// person's machine can listen there.
func IsLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// clean trims a display string and cuts it to limit runes, dropping control
// characters, so a hostile name cannot break the consent screen's layout.
func clean(value string, limit int) string {
	kept := []rune{}
	for _, r := range strings.TrimSpace(value) {
		if r < 0x20 || r == 0x7f || (r >= 0x200b && r <= 0x202e) {
			continue
		}
		kept = append(kept, r)
		if len(kept) == limit {
			break
		}
	}
	return string(kept)
}

// invalid is an ErrInvalid that says why.
func invalid(why string) error { return errors.Join(ErrInvalid, errors.New(why)) }

// invalidRedirect is an ErrInvalid about a redirect URI.
func invalidRedirect(why string) error { return errors.Join(ErrInvalid, ErrRedirect, errors.New(why)) }
