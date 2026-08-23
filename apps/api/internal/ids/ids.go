// Package ids mints the opaque identifiers registre hands out.
//
// Subjects and key ids are UUIDv4 in shape but not in provenance: nothing here
// is derived from a MAC address or a clock, only from crypto/rand. The shape is
// kept because relying parties and operators recognise it, and because the
// migration tool in SPEC §9 has to sit a fresh subject next to an Authentik
// one in the same column.
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a random RFC 4122 version 4 identifier in its canonical
// hyphenated text form.
func New() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	encoded := make([]byte, 32)
	hex.Encode(encoded, raw)
	return string(encoded[0:8]) + "-" + string(encoded[8:12]) + "-" +
		string(encoded[12:16]) + "-" + string(encoded[16:20]) + "-" + string(encoded[20:32]), nil
}

// Secret returns a 32-byte random value, hex encoded. It is the shape a client
// secret is minted in when registre generates one rather than being handed one.
func Secret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
