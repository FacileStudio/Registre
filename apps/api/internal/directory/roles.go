package directory

import (
	"context"
	"errors"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/spaces"
	"gorm.io/gorm"
)

// AddMember puts the Registre user with email into the space at role. The
// actor must be an admin or owner and may not grant a rank above their own.
func (s *Service) AddMember(ctx context.Context, actorID, spaceID, email, role string) (MemberView, error) {
	if !validRole(role) {
		return MemberView{}, ErrInvalidRole
	}
	var added MemberView
	err := s.commit(ctx, func(tx *gorm.DB) error {
		actorRole, err := s.requireRank(tx, actorID, spaceID, wire.RoleAdmin)
		if err != nil {
			return err
		}
		if !s.ladder.AtLeast(spaces.Role(actorRole), spaces.Role(role)) {
			return ErrForbidden
		}
		var user schemas.User
		err = tx.Where("lower(email) = ? AND disabled_at IS NULL", strings.ToLower(strings.TrimSpace(email))).Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNoSuchUser
		}
		if err != nil {
			return err
		}
		if _, _, err := s.membership(tx, user.ID, spaceID); err == nil {
			return ErrAlreadyMember
		}
		added = MemberView{UserID: user.ID, Email: user.Email, Name: user.Name, Picture: s.picture(user.ID), Role: role}
		return tx.Create(&schemas.SpaceMember{SpaceID: spaceID, UserID: user.ID, Role: role}).Error
	})
	return added, err
}

// SetRole moves targetID to role. The actor must rank at or above both the
// role taken away and the role granted, and the last owner stays an owner.
func (s *Service) SetRole(ctx context.Context, actorID, spaceID, targetID, role string) error {
	if !validRole(role) {
		return ErrInvalidRole
	}
	return s.commit(ctx, func(tx *gorm.DB) error {
		current, err := s.guardChange(tx, actorID, spaceID, targetID)
		if err != nil {
			return err
		}
		if !s.ladder.AtLeast(spaces.Role(s.mustRole(tx, actorID, spaceID)), spaces.Role(role)) {
			return ErrForbidden
		}
		if current == wire.RoleOwner && role != wire.RoleOwner {
			if err := keepsAnOwner(tx, spaceID); err != nil {
				return err
			}
		}
		return tx.Model(&schemas.SpaceMember{}).
			Where("space_id = ? AND user_id = ?", spaceID, targetID).Update("role", role).Error
	})
}

// RemoveMember takes targetID out of the space, under the same rank rule as
// SetRole, never leaving the space without an owner.
func (s *Service) RemoveMember(ctx context.Context, actorID, spaceID, targetID string) error {
	return s.commit(ctx, func(tx *gorm.DB) error {
		current, err := s.guardChange(tx, actorID, spaceID, targetID)
		if err != nil {
			return err
		}
		return s.drop(tx, spaceID, targetID, current)
	})
}

// Leave takes the caller out of the space. The last owner cannot leave.
func (s *Service) Leave(ctx context.Context, userID, spaceID string) error {
	return s.commit(ctx, func(tx *gorm.DB) error {
		role, err := s.requireRank(tx, userID, spaceID, wire.RoleMember)
		if err != nil {
			return err
		}
		return s.drop(tx, spaceID, userID, role)
	})
}

// guardChange checks an admin-or-owner actor outranks or equals the target's
// current role, and answers that role.
func (s *Service) guardChange(tx *gorm.DB, actorID, spaceID, targetID string) (string, error) {
	actorRole, err := s.requireRank(tx, actorID, spaceID, wire.RoleAdmin)
	if err != nil {
		return "", err
	}
	current, _, err := s.membership(tx, targetID, spaceID)
	if err != nil {
		return "", err
	}
	if !s.ladder.AtLeast(spaces.Role(actorRole), spaces.Role(current)) {
		return "", ErrForbidden
	}
	return current, nil
}

// drop deletes one membership, refusing to remove the last owner.
func (s *Service) drop(tx *gorm.DB, spaceID, userID, role string) error {
	if role == wire.RoleOwner {
		if err := keepsAnOwner(tx, spaceID); err != nil {
			return err
		}
	}
	return tx.Where("space_id = ? AND user_id = ?", spaceID, userID).Delete(&schemas.SpaceMember{}).Error
}

// mustRole reads a role already proven to exist inside the same transaction.
func (s *Service) mustRole(tx *gorm.DB, userID, spaceID string) string {
	role, _, _ := s.membership(tx, userID, spaceID)
	return role
}

// keepsAnOwner refuses a change that would take away the space's only owner.
func keepsAnOwner(tx *gorm.DB, spaceID string) error {
	var owners int64
	err := tx.Model(&schemas.SpaceMember{}).
		Where("space_id = ? AND role = ?", spaceID, wire.RoleOwner).Count(&owners).Error
	if err != nil {
		return err
	}
	if owners <= 1 {
		return ErrLastOwner
	}
	return nil
}

// validRole reports whether role is on the suite ladder.
func validRole(role string) bool {
	return role == wire.RoleOwner || role == wire.RoleAdmin || role == wire.RoleMember
}
