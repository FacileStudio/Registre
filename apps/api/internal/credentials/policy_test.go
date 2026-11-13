package credentials

import (
	"errors"
	"strings"
	"testing"
)

// TestTheLengthPolicyFollowsNIST: eight to 128 characters counted as
// characters, and no composition rule.
func TestTheLengthPolicyFollowsNIST(t *testing.T) {
	cases := map[string]error{
		strings.Repeat("a", 7):   ErrPasswordTooShort,
		strings.Repeat("é", 7):   ErrPasswordTooShort,
		strings.Repeat("a", 8):   nil,
		strings.Repeat("é", 8):   nil,
		strings.Repeat("a", 128): nil,
		strings.Repeat("a", 129): ErrPasswordTooLong,
	}
	for password, want := range cases {
		if err := CheckLength(password); !errors.Is(err, want) {
			t.Errorf("CheckLength(%d chars) = %v, want %v", len([]rune(password)), err, want)
		}
	}
}
