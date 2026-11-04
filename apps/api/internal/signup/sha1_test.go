package signup

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// sha1Hex is the uppercase SHA-1 the range API keys on.
func sha1Hex(password string) string {
	sum := sha1.Sum([]byte(password))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
