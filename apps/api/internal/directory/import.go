package directory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/spaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Import merges the shared spaces one app held before registre owned them,
// once per app, and answers the snapshot.
//
// Spaces merge by name, ignoring case, so the same team recreated by hand in
// five apps becomes one. A person keeps the highest role any app gave them,
// because nobody should lose access they already had. A person registre does
// not know, or has disabled, is skipped: they cannot sign in anywhere, so
// there is nobody to grant. A space left with no owner after the merge has its
// highest-ranked members promoted, because only an owner can create another.
//
// The import is recorded in the same transaction, and a second call from the
// same app changes nothing. That is the guarantee an app relies on when it
// imports at every boot until one succeeds: a member an owner removed here can
// never be brought back by an app restarting.
func (s *Service) Import(ctx context.Context, clientID string, spaces []wire.ImportSpace) (wire.Snapshot, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created := tx.Exec(`INSERT INTO directory_imports (client_id) VALUES (?)
			ON CONFLICT (client_id) DO NOTHING`, clientID)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return errAlreadyImported
		}
		for _, space := range spaces {
			if err := s.mergeSpace(tx, space); err != nil {
				return err
			}
		}
		return changed(tx)
	})
	switch {
	case errors.Is(err, errAlreadyImported):
	case err != nil:
		return wire.Snapshot{}, fmt.Errorf("directory: import from %s: %w", clientID, err)
	default:
		s.logger.Info("registre: an app imported its spaces")
		s.nudger.Fire()
	}
	return s.Snapshot(ctx)
}

// errAlreadyImported ends the import transaction without writing anything.
var errAlreadyImported = errors.New("directory: already imported")

// mergeSpace folds one imported space into the space of the same name.
func (s *Service) mergeSpace(tx *gorm.DB, in wire.ImportSpace) error {
	members := resolveMembers(tx, in.Members)
	name := strings.TrimSpace(in.Name)
	if name == "" || len(members) == 0 {
		return nil
	}
	spaceID, err := spaceNamed(tx, name)
	if err != nil {
		return err
	}
	for userID, role := range members {
		if err := s.grantAtLeast(tx, spaceID, userID, role); err != nil {
			return err
		}
	}
	return ensureOwner(tx, spaceID)
}

// resolveMembers maps imported people to registre users with their highest
// role, dropping anyone registre does not know or has disabled.
func resolveMembers(tx *gorm.DB, in []wire.ImportMember) map[string]string {
	out := map[string]string{}
	ladder := spaces.Default()
	for _, member := range in {
		userID := lookupUser(tx, member)
		role := normalRole(member.Role)
		if userID == "" {
			continue
		}
		if current, ok := out[userID]; !ok || ladder.AtLeast(spaces.Role(role), spaces.Role(current)) {
			out[userID] = role
		}
	}
	return out
}

// lookupUser finds the enabled user an imported member names, by subject
// first and email second, or "".
func lookupUser(tx *gorm.DB, member wire.ImportMember) string {
	var user schemas.User
	query := tx.Where("disabled_at IS NULL")
	switch {
	case member.Subject != "":
		query = query.Where("id = ?", member.Subject)
	case member.Email != "":
		query = query.Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(member.Email)))
	default:
		return ""
	}
	if err := query.Select("id").Take(&user).Error; err != nil {
		if member.Subject != "" && member.Email != "" {
			return lookupUser(tx, wire.ImportMember{Email: member.Email})
		}
		return ""
	}
	return user.ID
}

// spaceNamed returns the id of the space with this name, creating it.
func spaceNamed(tx *gorm.DB, name string) (string, error) {
	var existing schemas.Space
	err := tx.Where("lower(name) = lower(?) AND deleted_at IS NULL", name).Take(&existing).Error
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	return id, tx.Create(&schemas.Space{ID: id, Name: name}).Error
}

// grantAtLeast gives userID role in spaceID, raising an existing lower role
// and never lowering a higher one.
func (s *Service) grantAtLeast(tx *gorm.DB, spaceID, userID, role string) error {
	var current schemas.SpaceMember
	err := tx.Where("space_id = ? AND user_id = ?", spaceID, userID).Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&schemas.SpaceMember{SpaceID: spaceID, UserID: userID, Role: role}).Error
	}
	if err != nil || s.ladder.AtLeast(spaces.Role(current.Role), spaces.Role(role)) {
		return err
	}
	return tx.Model(&schemas.SpaceMember{}).
		Where("space_id = ? AND user_id = ?", spaceID, userID).Update("role", role).Error
}

// ensureOwner promotes the highest-ranked members of an ownerless space.
func ensureOwner(tx *gorm.DB, spaceID string) error {
	var owners int64
	if err := tx.Model(&schemas.SpaceMember{}).
		Where("space_id = ? AND role = ?", spaceID, wire.RoleOwner).Count(&owners).Error; err != nil {
		return err
	}
	if owners > 0 {
		return nil
	}
	for _, role := range []string{wire.RoleAdmin, wire.RoleMember} {
		result := tx.Model(&schemas.SpaceMember{}).
			Where("space_id = ? AND role = ?", spaceID, role).Update("role", wire.RoleOwner)
		if result.Error != nil || result.RowsAffected > 0 {
			return result.Error
		}
	}
	return nil
}

// normalRole maps an imported role onto the ladder; anything unknown, such as
// an app's viewer or editor, imports as member.
func normalRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case wire.RoleOwner:
		return wire.RoleOwner
	case wire.RoleAdmin:
		return wire.RoleAdmin
	default:
		return wire.RoleMember
	}
}
