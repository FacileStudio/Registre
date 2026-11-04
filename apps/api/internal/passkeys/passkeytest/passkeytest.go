// Package passkeytest is a software authenticator for tests: it answers the
// options Registre hands navigator.credentials.create and .get the way a
// browser with a platform passkey would, with "none" attestation and a P-256
// key, so registration and sign-in run through go-webauthn's real
// verification end to end.
//
// go-webauthn ships no virtual authenticator, so this is the smallest one
// that satisfies it. It is imported by tests only.
package passkeytest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
)

// Flag bits of the authenticator data.
const (
	flagUserPresent  = 0x01
	flagUserVerified = 0x04
	flagAttested     = 0x40
)

// Authenticator holds one discoverable credential once Create has run.
type Authenticator struct {
	RPID   string
	Origin string

	key          *ecdsa.PrivateKey
	credentialID []byte
	userHandle   []byte
	counter      uint32
}

// New builds an authenticator for a relying party served at origin.
func New(rpID, origin string) *Authenticator {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	id := make([]byte, 32)
	_, _ = rand.Read(id)
	return &Authenticator{RPID: rpID, Origin: origin, key: key, credentialID: id}
}

// options is the part of the creation and request options the authenticator
// reads.
type options struct {
	Challenge string `json:"challenge"`
	User      struct {
		ID string `json:"id"`
	} `json:"user"`
}

// Create answers creation options with a registration response.
func (a *Authenticator) Create(publicKey any) (json.RawMessage, error) {
	opts, err := read(publicKey)
	if err != nil {
		return nil, err
	}
	if a.userHandle, err = base64.RawURLEncoding.DecodeString(opts.User.ID); err != nil {
		return nil, fmt.Errorf("passkeytest: user id: %w", err)
	}
	client := a.clientData("webauthn.create", opts.Challenge)
	attestation, err := webauthncbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": a.attestedData(),
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"id": encode(a.credentialID), "rawId": encode(a.credentialID), "type": "public-key",
		"response": map[string]any{"clientDataJSON": encode(client), "attestationObject": encode(attestation)},
	})
}

// Get answers request options with an assertion signed by the credential.
func (a *Authenticator) Get(publicKey any) (json.RawMessage, error) {
	opts, err := read(publicKey)
	if err != nil {
		return nil, err
	}
	a.counter++
	client := a.clientData("webauthn.get", opts.Challenge)
	data := a.header(flagUserPresent | flagUserVerified)
	digest := sha256.Sum256(client)
	signed := sha256.Sum256(append(append([]byte{}, data...), digest[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, a.key, signed[:])
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"id": encode(a.credentialID), "rawId": encode(a.credentialID), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": encode(client), "authenticatorData": encode(data),
			"signature": encode(signature), "userHandle": encode(a.userHandle),
		},
	})
}

// clientData is the browser's collected client data.
func (a *Authenticator) clientData(kind, challenge string) []byte {
	raw, _ := json.Marshal(map[string]any{"type": kind, "challenge": challenge, "origin": a.Origin})
	return raw
}

// header is rpIdHash, flags and the signature counter.
func (a *Authenticator) header(flags byte) []byte {
	rpHash := sha256.Sum256([]byte(a.RPID))
	out := append([]byte{}, rpHash[:]...)
	out = append(out, flags)
	return binary.BigEndian.AppendUint32(out, a.counter)
}

// attestedData is the registration authenticator data: the header, a zero
// AAGUID, the credential id and its COSE public key.
func (a *Authenticator) attestedData() []byte {
	out := a.header(flagUserPresent | flagUserVerified | flagAttested)
	out = append(out, make([]byte, 16)...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(a.credentialID)))
	out = append(out, a.credentialID...)
	public, _ := a.key.PublicKey.ECDH()
	point := public.Bytes()
	cose, _ := webauthncbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: point[1:33], -3: point[33:]})
	return append(out, cose...)
}

// read decodes the options a passkeys.Options carries.
func read(publicKey any) (options, error) {
	raw, err := json.Marshal(publicKey)
	if err != nil {
		return options{}, err
	}
	var opts options
	if err := json.Unmarshal(raw, &opts); err != nil {
		return options{}, err
	}
	return opts, nil
}

// encode is base64url without padding, as WebAuthn JSON uses.
func encode(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }
