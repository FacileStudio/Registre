// Package pwned screens a new password against the Have I Been Pwned
// corpus, as NIST SP 800-63B-4 §3.1.1.2 requires for a blocklist check.
//
// It uses the k-anonymity range API: only the first five hex characters of
// the password's SHA-1 leave this process, the answer lists every suffix
// sharing that prefix, and the comparison happens here. Add-Padding asks the
// service to pad the answer with zero-count decoys so its size says nothing
// about the prefix either.
//
// A sign-up must never depend on a third party, so an outage, a timeout or a
// malformed answer is logged as a warning and the password passes.
package pwned

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// RangeURL is the fixed base of the range API. It is a constant, not
// configuration: the only host a password prefix is ever sent to.
const RangeURL = "https://api.pwnedpasswords.com/range/"

// Timeout bounds one lookup.
const Timeout = 3 * time.Second

// maxBody caps the answer. A padded range is around 40 KB.
const maxBody = 1 << 20

// Checker answers whether a password appears in a known breach.
type Checker struct {
	base   string
	client *http.Client
	logger *slog.Logger
}

// New builds a checker against RangeURL.
func New(logger *slog.Logger) *Checker {
	return NewAt(RangeURL, logger)
}

// NewAt builds a checker against another base URL. Tests use it to point at
// a local server.
func NewAt(base string, logger *slog.Logger) *Checker {
	return &Checker{base: base, client: &http.Client{Timeout: Timeout}, logger: logger}
}

// Breached reports whether password is in the corpus. A nil Checker, and
// every failure to ask, answers false: screening is skipped, never fatal.
func (c *Checker) Breached(ctx context.Context, password string) bool {
	if c == nil {
		return false
	}
	sum := sha1.Sum([]byte(password))
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))
	found, err := c.lookup(ctx, digest[:5], digest[5:])
	if err != nil {
		c.logger.Warn("registre: breached-password screening skipped", slog.Any("error", err))
		return false
	}
	return found
}

// lookup asks for one prefix and scans the answer for suffix.
func (c *Checker) lookup(ctx context.Context, prefix, suffix string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+prefix, nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("Add-Padding", "true")
	request.Header.Set("User-Agent", "registre")
	response, err := c.client.Do(request)
	if err != nil {
		return false, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("pwned: range answered %d", response.StatusCode)
	}
	return scan(io.LimitReader(response.Body, maxBody), suffix)
}

// scan reads SUFFIX:COUNT lines. A padded decoy carries a count of zero and
// is never a match.
func scan(body io.Reader, suffix string) (bool, error) {
	lines := bufio.NewScanner(body)
	for lines.Scan() {
		hash, count, ok := strings.Cut(strings.TrimSpace(lines.Text()), ":")
		if ok && strings.EqualFold(hash, suffix) && count != "0" {
			return true, nil
		}
	}
	return false, lines.Err()
}
