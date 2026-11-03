// Package directory owns the suite's shared spaces: who is in which team, at
// which rank, for every app at once.
//
// Apps never edit this state. They pull Snapshot and make their own tables
// match it, so a space created here, a mate added here or a role changed here
// reaches every app without anyone recreating anything. Every change bumps
// one version counter in the same transaction and then nudges each app that
// declared a sync URI; the nudge carries nothing, the authenticated pull is
// the part that matters.
//
// It owns no routes: modules/spaces serves the humans and modules/directory
// serves the apps, by the same rule that keeps clients and keys here.
package directory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/spaces"
	"gorm.io/gorm"
)

// Errors a caller maps to HTTP answers. Each is safe to show the person who
// caused it: it describes their own request, never somebody else's data.
var (
	ErrNotFound      = errors.New("space not found")
	ErrForbidden     = errors.New("your role does not allow this")
	ErrNameTaken     = errors.New("a space with this name already exists")
	ErrNoSuchUser    = errors.New("no Registre account has this email")
	ErrLastOwner     = errors.New("a space must keep at least one owner")
	ErrInvalidRole   = errors.New("role must be owner, admin or member")
	ErrInvalidName   = errors.New("a space needs a name")
	ErrAlreadyMember = errors.New("this person is already in the space")
)

// Service reads and changes shared spaces.
type Service struct {
	db      *gorm.DB
	picture func(userID string) string
	nudger  *Nudger
	logger  *slog.Logger
	ladder  spaces.Ladder
}

// New builds the service. picture names a human's stable picture URL, and
// nudger tells the apps a change happened; either may be nil in tests.
func New(db *gorm.DB, picture func(string) string, nudger *Nudger, logger *slog.Logger) *Service {
	if picture == nil {
		picture = func(string) string { return "" }
	}
	return &Service{db: db, picture: picture, nudger: nudger, logger: logger, ladder: spaces.Default()}
}

// Snapshot is every space with its enabled members, as apps receive it.
// Disabled humans are left out, which is what makes disabling somebody
// remove them from every space in every app.
func (s *Service) Snapshot(ctx context.Context) (wire.Snapshot, error) {
	var out wire.Snapshot
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw("SELECT version FROM directory_state WHERE id = 1").Scan(&out.Version).Error; err != nil {
			return err
		}
		spacesByID, order, err := allSpaces(tx)
		if err != nil {
			return err
		}
		if err := s.attachMembers(tx, spacesByID); err != nil {
			return err
		}
		out.Spaces = make([]wire.Space, 0, len(order))
		for _, id := range order {
			out.Spaces = append(out.Spaces, *spacesByID[id])
		}
		return nil
	})
	if err != nil {
		return wire.Snapshot{}, fmt.Errorf("directory: read the snapshot: %w", err)
	}
	return out, nil
}

// memberRow is one membership joined to its human.
type memberRow struct {
	SpaceID string
	UserID  string
	Email   string
	Name    string
	Role    string
}

// allSpaces reads every space in name order.
func allSpaces(tx *gorm.DB) (map[string]*wire.Space, []string, error) {
	var rows []schemas.Space
	if err := tx.Order("lower(name)").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	byID := make(map[string]*wire.Space, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		byID[row.ID] = &wire.Space{ID: row.ID, Name: row.Name, Members: []wire.Member{}}
		order = append(order, row.ID)
	}
	return byID, order, nil
}

// attachMembers fills each space with its enabled members.
func (s *Service) attachMembers(tx *gorm.DB, byID map[string]*wire.Space) error {
	var rows []memberRow
	err := tx.Raw(`SELECT m.space_id, m.user_id, u.email, u.name, m.role
		FROM space_members m JOIN users u ON u.id = m.user_id
		WHERE u.disabled_at IS NULL ORDER BY lower(u.email)`).Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		if space, ok := byID[row.SpaceID]; ok {
			space.Members = append(space.Members, s.member(row))
		}
	}
	return nil
}

// member renders one membership on the wire.
func (s *Service) member(row memberRow) wire.Member {
	return wire.Member{
		Subject: row.UserID, Email: row.Email, Name: row.Name,
		Picture: s.picture(row.UserID), Role: row.Role,
	}
}

// changed bumps the version inside tx. Every write calls it, so an app never
// skips a snapshot that differs from the last one it applied.
func changed(tx *gorm.DB) error {
	return tx.Exec(`INSERT INTO directory_state (id, version) VALUES (1, 2)
		ON CONFLICT (id) DO UPDATE SET version = directory_state.version + 1`).Error
}

// commit runs write in a transaction that bumps the version, then nudges the
// apps once it has committed.
func (s *Service) commit(ctx context.Context, write func(tx *gorm.DB) error) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := write(tx); err != nil {
			return err
		}
		return changed(tx)
	})
	if err != nil {
		return err
	}
	s.nudger.Fire()
	return nil
}

// newID mints a space id.
func newID() (string, error) { return ids.New() }
