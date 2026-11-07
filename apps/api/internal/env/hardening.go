package env

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	troncenv "github.com/FacileStudio/tronc/env"
)

// Defaults for the database pool and the per-address budgets of the two
// endpoints that pay for an argon2 verification.
//
// The pool default is well under Postgres's stock max_connections of 100: at
// 128 concurrent requests an uncapped pool opened a connection per request and
// the database answered "too many clients" to the API and to everything else
// that shares it. Requests past the cap queue for a connection instead.
const (
	DefaultDBMaxOpen     = 40
	DefaultDBMaxIdle     = 10
	DefaultDBMaxLifetime = 30 * time.Minute
	DefaultLoginRate     = 1.0
	DefaultLoginBurst    = 10
	DefaultTokenRate     = 1.0
	DefaultTokenBurst    = 10
	DefaultVerifyWait    = 3 * time.Second
)

// Hardening is the overload and abuse configuration: the database pool, the
// per-address budgets of the credential endpoints, and how long a request
// queues for an argon2 slot before it is answered 503.
type Hardening struct {
	// VerifyConcurrency is how many argon2 verifications run at once. It is
	// a memory ceiling wearing a count: multiply it by 64 MiB to get the
	// transient memory the token endpoint may hold under load.
	VerifyConcurrency int

	DBMaxOpen     int
	DBMaxIdle     int
	DBMaxLifetime time.Duration

	LoginRate  float64
	LoginBurst int
	TokenRate  float64
	TokenBurst int

	VerifyWait time.Duration
}

// loadHardening reads the REGISTRE_DB_*, REGISTRE_LOGIN_*, REGISTRE_TOKEN_*
// and REGISTRE_VERIFY_QUEUE_WAIT variables. An empty value means the default,
// so the compose file can pass them through unset.
func loadHardening() (Hardening, error) {
	var h Hardening
	if err := h.loadCounts(); err != nil {
		return Hardening{}, err
	}
	var err error
	if h.VerifyWait, err = troncenv.Duration("REGISTRE_VERIFY_QUEUE_WAIT", DefaultVerifyWait); err != nil {
		return Hardening{}, err
	}
	if h.LoginRate, err = rate("REGISTRE_LOGIN_RATE", DefaultLoginRate); err != nil {
		return Hardening{}, err
	}
	h.TokenRate, err = rate("REGISTRE_TOKEN_RATE", DefaultTokenRate)
	return h, err
}

// loadCounts reads the integer settings and the connection lifetime.
func (h *Hardening) loadCounts() error {
	var err error
	ints := []struct {
		key      string
		fallback int
		into     *int
		min      int
	}{
		{"REGISTRE_DB_MAX_OPEN_CONNS", DefaultDBMaxOpen, &h.DBMaxOpen, 1},
		{"REGISTRE_DB_MAX_IDLE_CONNS", DefaultDBMaxIdle, &h.DBMaxIdle, 0},
		{"REGISTRE_LOGIN_BURST", DefaultLoginBurst, &h.LoginBurst, 1},
		{"REGISTRE_TOKEN_BURST", DefaultTokenBurst, &h.TokenBurst, 1},
	}
	for _, field := range ints {
		if *field.into, err = troncenv.Int(field.key, field.fallback); err != nil {
			return err
		}
		if *field.into < field.min {
			return fmt.Errorf("%s must be at least %d, got %d", field.key, field.min, *field.into)
		}
	}
	if h.DBMaxIdle > h.DBMaxOpen {
		h.DBMaxIdle = h.DBMaxOpen
	}
	h.DBMaxLifetime, err = troncenv.Duration("REGISTRE_DB_CONN_MAX_LIFETIME", DefaultDBMaxLifetime)
	return err
}

// rate reads a per-second request rate. Zero or negative is refused: a limiter
// at zero would reject every request, and a typo should fail the boot rather
// than the login page.
func rate(key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(troncenv.String(key, ""))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive number of requests per second, got %q", key, raw)
	}
	return value, nil
}
