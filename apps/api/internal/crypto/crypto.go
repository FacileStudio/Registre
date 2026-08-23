// Package crypto seals the signing keys at rest.
//
// A signing key is the one secret in this database that is worth more than the
// database: whoever holds it mints tokens for every app in the suite without
// touching registre again. It is stored sealed under a root key that lives in
// the environment, so a dump of the database alone does not carry it.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// KeyLength is the root key size in bytes: AES-256.
const KeyLength = 32

// Seal encrypts plaintext with AES-256-GCM, returning the ciphertext and its
// random nonce separately so the nonce can be stored beside the value.
func Seal(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

// Open decrypts ciphertext with the nonce that sealed it.
func Open(ciphertext, nonce, key []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// KeyFromBase64 decodes a 32-byte base64 root key, rejecting anything that is
// not exactly one.
func KeyFromBase64(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != KeyLength {
		return nil, fmt.Errorf("encryption key must be %d bytes, got %d", KeyLength, len(key))
	}
	return key, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return gcm, nil
}
