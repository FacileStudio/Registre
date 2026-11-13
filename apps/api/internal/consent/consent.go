// Package consent knows which legal documents exist, which version of each is
// current, and whether a person has accepted the current version of the ones
// that need acceptance.
//
// Bumping a version here is the whole of asking everyone again: the next
// sign-in of every person whose latest acceptance names an older version is
// held at the consent screen until they accept (identity.Complete asks
// Outdated). The texts live in modules/legal; the versions live here because
// sign-in, sign-up and the legal pages all need them and only one of those is
// a module.
package consent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// Document names, as they appear in /legal/{document} URLs.
const (
	CGU         = "cgu"
	CGV         = "cgv"
	Privacy     = "privacy"
	LegalNotice = "legal-notice"
)

// Versions answers the current version of every document. A version is the
// date the text took effect; change it in the same commit as the text.
func Versions() map[string]string {
	return map[string]string{
		CGU:         "2026-11-13",
		CGV:         "2026-11-04",
		Privacy:     "2026-11-04",
		LegalNotice: "2026-11-04",
	}
}

// Draft reports whether the texts are drafts pending legal review. While it
// is true nobody is held at the consent screen: asking every existing person
// to accept unreviewed terms would bind them to a text nobody has checked.
// Acceptances given anyway (at sign-up) are recorded with draft set, and are
// asked again once the reviewed versions land. Flip it, and bump Versions, in
// the commit that lands the reviewed texts.
func Draft() bool { return true }

// Required answers the documents a person must accept to hold an account.
// The CGV bind only a paying customer and are accepted at checkout; the
// legal notice is information, not an agreement.
func Required() []string { return []string{CGU, Privacy} }

// Service records and checks acceptances.
type Service struct {
	db    *gorm.DB
	key   []byte
	now   func() time.Time
	draft func() bool
}

// New builds the service. key is what source addresses are digested under,
// so the stored digest cannot be reversed by hashing all four billion IPv4
// addresses.
func New(db *gorm.DB, key []byte) *Service {
	return &Service{db: db, key: key, now: func() time.Time { return time.Now().UTC() }, draft: Draft}
}

// Record appends an acceptance of the current version of every required
// document, inside tx when one is given.
func (s *Service) Record(ctx context.Context, tx *gorm.DB, userID, locale, ip string) error {
	if tx == nil {
		tx = s.db
	}
	versions := Versions()
	rows := make([]schemas.Consent, 0, len(Required()))
	for _, document := range Required() {
		rows = append(rows, schemas.Consent{
			UserID: userID, Document: document, Version: versions[document],
			Locale: locale, AcceptedAt: s.now(), IPHash: s.digest(ip), Draft: s.draft(),
		})
	}
	if err := tx.WithContext(ctx).Create(&rows).Error; err != nil {
		return fmt.Errorf("consent: record: %w", err)
	}
	return nil
}

// Outdated answers the required documents whose current version userID has
// not accepted. Empty means nothing is owed, which is always the answer
// while the texts are drafts: only a reviewed version can be required.
func (s *Service) Outdated(ctx context.Context, userID string) ([]string, error) {
	if s.draft() {
		return nil, nil
	}
	var accepted []schemas.Consent
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND document IN ?", userID, Required()).
		Find(&accepted).Error
	if err != nil {
		return nil, fmt.Errorf("consent: read: %w", err)
	}
	have := map[string]bool{}
	versions := Versions()
	for _, row := range accepted {
		if row.Version == versions[row.Document] && !row.Draft {
			have[row.Document] = true
		}
	}
	var owed []string
	for _, document := range Required() {
		if !have[document] {
			owed = append(owed, document)
		}
	}
	return owed, nil
}

// History answers every acceptance userID made, oldest first, for the
// personal data export.
func (s *Service) History(ctx context.Context, userID string) ([]schemas.Consent, error) {
	var rows []schemas.Consent
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("accepted_at, id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("consent: history: %w", err)
	}
	return rows, nil
}

// digest keys the source address so equal addresses match without being
// readable.
func (s *Service) digest(ip string) string {
	if ip == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("consent-ip:" + ip))
	return hex.EncodeToString(mac.Sum(nil))
}

// Enforcing answers a copy of s that requires the current versions as if
// they were reviewed. Tests use it to exercise the consent screen before the
// texts are final.
func (s *Service) Enforcing() *Service {
	copied := *s
	copied.draft = func() bool { return false }
	return &copied
}
