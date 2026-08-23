// Package keys owns registre's token signing keys and their rotation.
//
// SPEC §7 calls key rotation the most likely cause of a suite-wide outage, and
// the reason is a race nobody sees in review: a relying party caches the key
// set, so a key that starts signing before it is published 401s every request
// until each party refetches, and a key dropped from the set while its tokens
// are still valid does the same in reverse. The lifecycle here exists to close
// both ends — published before it signs, retained after it stops.
package keys

import (
	"crypto/rsa"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Algorithm is the one signature algorithm registre issues under. RS256 is
// what every relying party in the suite already verifies — porte builds its
// verifier from discovery and Authentik signed RS256 before it — so the move
// changes no client. A second algorithm is a phase of its own, not a flag.
const Algorithm = jose.RS256

// bits is the RSA modulus size. 2048 is the floor every conformance suite and
// every JOSE library agrees on; 4096 costs signing latency on every token for
// margin nothing in this threat model asks for.
const bits = 2048

// Config is the key lifecycle, in durations.
type Config struct {
	// RotateEvery is how long a key signs before it is replaced.
	RotateEvery time.Duration

	// PublishDelay is how long a new key sits in the JWKS before it starts
	// signing. It must outlast the key set cache of the slowest relying
	// party in the suite.
	PublishDelay time.Duration

	// Retention is how long a retired public key stays in the JWKS after it
	// stops signing. Below one access token lifetime it drops a key while
	// tokens it signed are still valid; env.Load refuses that.
	Retention time.Duration

	// Now is the clock the lifecycle reads. Nil means time.Now in UTC. It
	// is a field rather than a setter so that nothing can move the clock
	// out from under the rotation goroutine after it has started.
	Now func() time.Time
}

// signingKey adapts a decrypted private key to what zitadel/oidc signs with.
type signingKey struct {
	id  string
	key *rsa.PrivateKey
}

// SignatureAlgorithm reports the algorithm this key signs under.
func (k signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return Algorithm }

// Key returns the private key itself.
func (k signingKey) Key() any { return k.key }

// ID returns the kid a relying party matches a token against.
func (k signingKey) ID() string { return k.id }

// publicKey adapts a stored public key to one entry of the JWKS.
type publicKey struct {
	id  string
	key *rsa.PublicKey
}

// ID returns the kid this entry is published under.
func (k publicKey) ID() string { return k.id }

// Algorithm reports the algorithm tokens signed by its private half carry.
func (k publicKey) Algorithm() jose.SignatureAlgorithm { return Algorithm }

// Use reports what the key is published for: signature verification.
func (k publicKey) Use() string { return "sig" }

// Key returns the public key itself.
func (k publicKey) Key() any { return k.key }
