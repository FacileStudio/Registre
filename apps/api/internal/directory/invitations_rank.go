package directory

import (
	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/spaces"
	"gorm.io/gorm"
)

// stillEntitled reports whether the person who sent row is still in the
// space at a rank that may offer its role. An invitation is the inviter's
// authority, held in escrow; it is worth nothing once that authority is gone.
func (s *Service) stillEntitled(tx *gorm.DB, row schemas.SpaceInvitation) bool {
	role, _, err := s.membership(tx, row.InvitedBy, row.SpaceID)
	if err != nil {
		return false
	}
	return s.ladder.AtLeast(spaces.Role(role), spaces.Role(wire.RoleAdmin)) &&
		s.ladder.AtLeast(spaces.Role(role), spaces.Role(row.Role))
}

// pruneInvitations withdraws the pending invitations userID sent in spaceID
// that they could no longer send: all of them when they left or were
// removed (role empty) or fell below admin, and the ones offering a rank
// above their new one otherwise.
func (s *Service) pruneInvitations(tx *gorm.DB, spaceID, userID, role string) error {
	scope := tx.Where("space_id = ? AND invited_by = ?", spaceID, userID)
	if role != "" && s.ladder.AtLeast(spaces.Role(role), spaces.Role(wire.RoleAdmin)) {
		var above []string
		for _, offered := range []string{wire.RoleOwner, wire.RoleAdmin, wire.RoleMember} {
			if !s.ladder.AtLeast(spaces.Role(role), spaces.Role(offered)) {
				above = append(above, offered)
			}
		}
		if len(above) == 0 {
			return nil
		}
		scope = scope.Where("role IN ?", above)
	}
	return scope.Delete(&schemas.SpaceInvitation{}).Error
}
