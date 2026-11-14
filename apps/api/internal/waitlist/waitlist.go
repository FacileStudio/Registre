// Package waitlist keeps the addresses that asked to hear when a product
// ships: Scribe's and Pastille's landing pages.
//
// It stores the address, the product, the page's language and the version
// of the consent line, and nothing else. Join is idempotent per address and
// product and answers the same whether or not the address was already on the
// list, so the form cannot be used to learn who signed up. Nothing is mailed
// on join: a public form that mails any address it is given would let anyone
// send Facile's mail to a stranger. Unsubscribing deletes the row, through a
// token derived from the row id under a key, so no token is stored.
//
// It owns no routes; modules/waitlist does.
package waitlist

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Products are the waitlists that exist.
func Products() []string { return []string{"scribe", "pastille"} }

// Locales are the languages a landing page is written in.
func Locales() []string { return []string{"fr", "en", "es", "de"} }

// MaxEmailLength is the longest address RFC 5321 lets through a mail path.
const MaxEmailLength = 254

var (
	// ErrEmail is a missing or malformed address.
	ErrEmail = errors.New("waitlist: not an email address")
	// ErrProduct is a product with no waitlist.
	ErrProduct = errors.New("waitlist: unknown product")
	// ErrLocale is a language no landing page is written in.
	ErrLocale = errors.New("waitlist: unknown locale")
	// ErrConsent is a request without a recognisable consent version.
	ErrConsent = errors.New("waitlist: consent required")
)

var consentVersion = regexp.MustCompile(`^[a-z0-9.-]{1,32}$`)

// Entry is one request to join a waitlist.
type Entry struct {
	Email   string
	Product string
	Locale  string
	Consent string
}

// Service reads and writes the waitlists.
type Service struct {
	db  *gorm.DB
	key []byte
	now func() time.Time
}

// New builds the service over db. key derives the unsubscribe tokens; it
// must stay the same for as long as links mailed under it should work.
func New(db *gorm.DB, key []byte) *Service {
	return &Service{db: db, key: unsubscribeKey(key), now: func() time.Time { return time.Now().UTC() }}
}

// Validate normalises an entry and reports the first field that is wrong.
func Validate(in Entry) (Entry, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	parsed, err := mail.ParseAddress(in.Email)
	if err != nil || parsed.Address != in.Email || len(in.Email) > MaxEmailLength || !dottedDomain(in.Email) {
		return Entry{}, ErrEmail
	}
	if !slices.Contains(Products(), in.Product) {
		return Entry{}, ErrProduct
	}
	if !slices.Contains(Locales(), in.Locale) {
		return Entry{}, ErrLocale
	}
	if !consentVersion.MatchString(in.Consent) {
		return Entry{}, ErrConsent
	}
	return in, nil
}

// dottedDomain reports whether the address's domain has a dot, which
// net/mail does not require and no deliverable public address lacks.
func dottedDomain(address string) bool {
	at := strings.LastIndex(address, "@")
	domain := address[at+1:]
	return at > 0 && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

// Join records an entry. An address already on that product's list keeps
// its first row, and the call succeeds all the same.
func (s *Service) Join(ctx context.Context, in Entry) error {
	in, err := Validate(in)
	if err != nil {
		return err
	}
	id, err := ids.New()
	if err != nil {
		return fmt.Errorf("waitlist: mint the entry id: %w", err)
	}
	row := schemas.WaitlistEntry{
		ID: id, Email: in.Email, Product: in.Product, Locale: in.Locale,
		ConsentVersion: in.Consent, CreatedAt: s.now(),
	}
	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("waitlist: record the entry: %w", err)
	}
	return nil
}

// Delete removes an address from one product's list, or from every list
// when product is empty, and reports how many rows went.
func (s *Service) Delete(ctx context.Context, email, product string) (int64, error) {
	query := s.db.WithContext(ctx).Where("email = ?", strings.ToLower(strings.TrimSpace(email)))
	if product != "" {
		query = query.Where("product = ?", product)
	}
	result := query.Delete(&schemas.WaitlistEntry{})
	if result.Error != nil {
		return 0, fmt.Errorf("waitlist: delete the entry: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// List returns one product's entries, or every entry when product is empty,
// oldest first.
func (s *Service) List(ctx context.Context, product string) ([]schemas.WaitlistEntry, error) {
	query := s.db.WithContext(ctx).Order("created_at, id")
	if product != "" {
		query = query.Where("product = ?", product)
	}
	var rows []schemas.WaitlistEntry
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("waitlist: list the entries: %w", err)
	}
	return rows, nil
}
