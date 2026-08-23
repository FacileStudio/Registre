package keys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/crypto"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

// ErrNoActiveKey is returned when nothing can be signed because the register
// holds no active key. It is a boot-order failure, not a request failure.
var ErrNoActiveKey = errors.New("keys: no active signing key")

// Service reads, mints and rotates the signing keys.
type Service struct {
	db     *gorm.DB
	root   []byte
	cfg    Config
	logger *slog.Logger
	now    func() time.Time

	// parsed caches decrypted private keys by kid. Decryption and PKCS#8
	// parsing on every token issue is pure waste — the row is immutable
	// once written, so a hit can never be stale.
	mu     sync.RWMutex
	parsed map[string]*rsa.PrivateKey
}

// New builds the service over db, sealing private keys under root.
func New(db *gorm.DB, root []byte, cfg Config, logger *slog.Logger) *Service {
	clock := cfg.Now
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		db:     db,
		root:   root,
		cfg:    cfg,
		logger: logger,
		now:    clock,
		parsed: map[string]*rsa.PrivateKey{},
	}
}

// Ensure guarantees the register has a key to sign with, minting one active
// immediately when it does not.
//
// The condition is "no active key", not "no keys at all". On a fresh register
// those are the same thing and skipping the publish delay is safe, because no
// relying party has ever fetched a key set and there is no cache to be ahead
// of. They stop being the same thing after any accident that leaves rows
// behind but none of them active — a hand-run UPDATE, a partial restore — and
// the earlier condition turned that into a provider that could not sign, at
// all, across every restart, recoverable only by editing the database that
// holds every credential in the suite.
//
// Minting into that state does publish and sign in one step, which the
// lifecycle otherwise refuses. It is the right trade: relying parties refetch
// on an unrecognised kid, so the cost is one refetch, and the alternative is
// an identity provider that stays down until somebody notices.
func (s *Service) Ensure(ctx context.Context) error {
	var count int64
	err := s.db.WithContext(ctx).Model(&schemas.SigningKey{}).
		Where("state = ?", schemas.KeyActive).Count(&count).Error
	if err != nil {
		return fmt.Errorf("keys: count the active keys: %w", err)
	}
	if count > 0 {
		return s.readable(ctx)
	}
	row, err := s.mint(schemas.KeyActive)
	if err != nil {
		return err
	}
	activated := s.now()
	row.ActivatedAt = &activated
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("keys: create the first key: %w", err)
	}
	s.logger.Info("registre: minted the first signing key", slog.String("kid", row.ID))
	return nil
}

// readable proves the root key still opens what the register holds.
//
// A changed ENCRYPTION_KEY is not a configuration error the process can see:
// the rows are all there, the JWKS still serves their public halves, and
// everything looks healthy until the first token request answers 500 with
// "message authentication failed". A restore from a backup taken under a
// different root key produces exactly that, which is a very bad hour to spend.
// One unseal at boot turns it into a refusal to start, naming the variable.
func (s *Service) readable(ctx context.Context) error {
	if _, err := s.Signing(ctx); err != nil {
		return fmt.Errorf(
			"keys: the active signing key cannot be opened with this ENCRYPTION_KEY (%w) — "+
				"the key changed, or this database was restored from a backup taken under a different one",
			err)
	}
	return nil
}

// Signing returns the key tokens are signed with right now.
func (s *Service) Signing(ctx context.Context) (op.SigningKey, error) {
	var row schemas.SigningKey
	err := s.db.WithContext(ctx).Where("state = ?", schemas.KeyActive).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoActiveKey
	}
	if err != nil {
		return nil, fmt.Errorf("keys: read the active key: %w", err)
	}
	private, err := s.private(row)
	if err != nil {
		return nil, err
	}
	return signingKey{id: row.ID, key: private}, nil
}

// Set returns every key that belongs in the JWKS: the active one, anything
// published ahead of it, and anything retired but still within retention.
func (s *Service) Set(ctx context.Context) ([]op.Key, error) {
	var rows []schemas.SigningKey
	err := s.db.WithContext(ctx).
		Where("state <> ? OR expires_at IS NULL OR expires_at > ?", schemas.KeyRetired, s.now()).
		Order("created_at").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("keys: read the key set: %w", err)
	}
	set := make([]op.Key, 0, len(rows))
	for _, row := range rows {
		public, err := publicFrom(row)
		if err != nil {
			s.logger.Error("registre: unreadable public key, skipped",
				slog.String("kid", row.ID), slog.Any("error", err))
			continue
		}
		set = append(set, public)
	}
	return set, nil
}

// Algorithms reports the signature algorithms discovery advertises.
func (s *Service) Algorithms() []jose.SignatureAlgorithm {
	return []jose.SignatureAlgorithm{Algorithm}
}

// mint generates a key pair and returns the row that stores it, sealed.
func (s *Service) mint(state string) (*schemas.SigningKey, error) {
	id, err := ids.New()
	if err != nil {
		return nil, fmt.Errorf("keys: mint an id: %w", err)
	}
	private, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("keys: generate: %w", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, fmt.Errorf("keys: marshal the private key: %w", err)
	}
	pkix, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("keys: marshal the public key: %w", err)
	}
	sealed, nonce, err := crypto.Seal(pkcs8, s.root)
	if err != nil {
		return nil, fmt.Errorf("keys: seal the private key: %w", err)
	}
	return &schemas.SigningKey{
		ID: id, Algorithm: string(Algorithm), PublicKey: pkix,
		PrivateKey: sealed, PrivateKeyNonce: nonce, State: state, CreatedAt: s.now(),
	}, nil
}

// private decrypts and parses a stored private key, caching the result.
func (s *Service) private(row schemas.SigningKey) (*rsa.PrivateKey, error) {
	s.mu.RLock()
	cached, ok := s.parsed[row.ID]
	s.mu.RUnlock()
	if ok {
		return cached, nil
	}

	pkcs8, err := crypto.Open(row.PrivateKey, row.PrivateKeyNonce, s.root)
	if err != nil {
		return nil, fmt.Errorf("keys: unseal %s: %w", row.ID, err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, fmt.Errorf("keys: parse %s: %w", row.ID, err)
	}
	private, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("keys: %s is a %T, not an RSA key", row.ID, parsed)
	}

	s.mu.Lock()
	s.parsed[row.ID] = private
	s.mu.Unlock()
	return private, nil
}

// publicFrom parses the stored PKIX public half of a key.
func publicFrom(row schemas.SigningKey) (publicKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(row.PublicKey)
	if err != nil {
		return publicKey{}, fmt.Errorf("parse %s: %w", row.ID, err)
	}
	public, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return publicKey{}, fmt.Errorf("%s is a %T, not an RSA key", row.ID, parsed)
	}
	return publicKey{id: row.ID, key: public}, nil
}
