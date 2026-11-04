package directory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"

	wire "github.com/FacileStudio/porte/directory"
	"gorm.io/gorm"
)

// ErrInvalidEntitlements is a push carrying a revision below 1, a key outside
// the suite's app.feature format, too many keys in one scope, or a value that
// is neither a bool nor an integer of at least -1.
var ErrInvalidEntitlements = errors.New("invalid entitlements")

// StaleRevisionError is a push older than the revision already applied.
// Current is that revision, so the writer can tell a replay from a gap.
type StaleRevisionError struct{ Current int64 }

// Error describes the refusal without the payload.
func (e *StaleRevisionError) Error() string {
	return fmt.Sprintf("stale revision, current is %d", e.Current)
}

// SpaceEntitlements is what one space may do, keyed by registre space id.
type SpaceEntitlements struct {
	ID           string                     `json:"id"`
	Entitlements map[string]json.RawMessage `json:"entitlements"`
}

// PersonEntitlements is what one person may do with their personal
// resources. Subject wins; Email is the fallback and only matches a verified
// address.
type PersonEntitlements struct {
	Subject      string                     `json:"sub"`
	Email        string                     `json:"email"`
	Entitlements map[string]json.RawMessage `json:"entitlements"`
}

// Applied counts what a push stored and what it skipped because Registre
// does not know the space or the person.
type Applied struct {
	Spaces        int
	People        int
	SkippedSpaces int
	SkippedPeople int
}

// SetEntitlements replaces every stored entitlement with the push, in one
// transaction that also bumps the directory version, and nudges the apps
// once it commits.
//
// The push is the whole truth: a scope it leaves out loses its values. A
// revision equal to the one applied is accepted so a writer retrying after a
// lost answer converges; an older one answers *StaleRevisionError and changes
// nothing, and so does one more than MaxRevisionJump ahead, which would
// otherwise let a single bad push lock out every honest one after it. An
// unknown space or person is skipped and counted, never fatal, because
// Ardoise may learn of a space a moment before or after Registre. The audit
// row for actorID is written in the same transaction, so a push is never
// applied without its record nor recorded without being applied.
func (s *Service) SetEntitlements(ctx context.Context, actorID string, revision int64,
	spaces []SpaceEntitlements, people []PersonEntitlements) (Applied, error) {
	if err := validatePush(revision, spaces, people); err != nil {
		return Applied{}, err
	}
	var out Applied
	err := s.commit(ctx, func(tx *gorm.DB) error {
		if err := advanceRevision(tx, revision); err != nil {
			return err
		}
		var err error
		if out.Spaces, out.SkippedSpaces, err = replaceSpaces(tx, spaces); err != nil {
			return err
		}
		if out.People, out.SkippedPeople, err = replacePeople(tx, people); err != nil {
			return err
		}
		return recordPush(tx, actorID, revision, out)
	})
	if err != nil {
		return Applied{}, err
	}
	s.logger.Info("registre: entitlements applied", slog.Int64("revision", revision),
		slog.Int("spaces", out.Spaces), slog.Int("people", out.People),
		slog.Int("skipped_spaces", out.SkippedSpaces), slog.Int("skipped_people", out.SkippedPeople))
	return out, nil
}

// advanceRevision locks the revision row and moves it to revision, refusing
// anything older than what is stored. The row is recreated when missing,
// as directory_state is, so an emptied table means revision zero.
func advanceRevision(tx *gorm.DB, revision int64) error {
	var current int64
	if err := tx.Exec(`INSERT INTO entitlements_state (id, revision) VALUES (1, 0)
		ON CONFLICT (id) DO NOTHING`).Error; err != nil {
		return err
	}
	if err := tx.Raw("SELECT revision FROM entitlements_state WHERE id = 1 FOR UPDATE").Scan(&current).Error; err != nil {
		return err
	}
	if revision < current {
		return &StaleRevisionError{Current: current}
	}
	if revision-current > MaxRevisionJump {
		return fmt.Errorf("%w: revision %d is more than %d past %d", ErrInvalidEntitlements,
			revision, MaxRevisionJump, current)
	}
	return tx.Exec("UPDATE entitlements_state SET revision = ? WHERE id = 1", revision).Error
}

// recordPush writes the push's audit row inside its transaction.
func recordPush(tx *gorm.DB, actorID string, revision int64, out Applied) error {
	return audit.Write(tx, actorID, "entitlements.set",
		audit.Target{Type: "entitlements", ID: strconv.FormatInt(revision, 10)},
		map[string]any{
			"spaces": out.Spaces, "people": out.People,
			"skipped_spaces": out.SkippedSpaces, "skipped_people": out.SkippedPeople,
		})
}

// populate fills the snapshot's spaces with their members and entitlements,
// stamps the last applied entitlements revision (0 before any push), and
// lists the people who have personal values.
func (s *Service) populate(tx *gorm.DB, out *wire.Snapshot, byID map[string]*wire.Space) error {
	if err := s.attachMembers(tx, byID); err != nil {
		return err
	}
	err := tx.Raw("SELECT COALESCE((SELECT revision FROM entitlements_state WHERE id = 1), 0)").
		Scan(&out.EntitlementsRevision).Error
	if err != nil {
		return err
	}
	return fillEntitlements(tx, out, byID)
}

// fillEntitlements puts the stored values on each snapshot space and lists
// every enabled person who has some.
func fillEntitlements(tx *gorm.DB, out *wire.Snapshot, byID map[string]*wire.Space) error {
	var spaces []entitlementRow
	if err := tx.Raw(`SELECT space_id AS id, "values" FROM space_entitlements`).Scan(&spaces).Error; err != nil {
		return err
	}
	for _, row := range spaces {
		if space, ok := byID[row.ID]; ok {
			space.Entitlements = row.decode()
		}
	}
	var people []entitlementRow
	err := tx.Raw(`SELECT e.user_id AS id, e."values" FROM user_entitlements e
		JOIN users u ON u.id = e.user_id WHERE u.disabled_at IS NULL ORDER BY e.user_id`).Scan(&people).Error
	if err != nil {
		return err
	}
	for _, row := range people {
		out.People = append(out.People, wire.Person{Subject: row.ID, Entitlements: row.decode()})
	}
	return nil
}
