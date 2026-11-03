package directory

import (
	"context"
	"errors"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/FacileStudio/porte/spaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Summary is one space as its member sees it in a list.
type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	MemberCount int    `json:"member_count"`
}

// Detail is one space with everyone in it.
type Detail struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Role    string       `json:"role"`
	Members []MemberView `json:"members"`
}

// MemberView is one person in a space, as the spaces page shows them.
type MemberView struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Role    string `json:"role"`
}

// Mine lists the spaces userID is in, with their own role in each.
func (s *Service) Mine(ctx context.Context, userID string) ([]Summary, error) {
	out := []Summary{}
	err := s.db.WithContext(ctx).Raw(`SELECT s.id, s.name, m.role,
			(SELECT count(*) FROM space_members c JOIN users u ON u.id = c.user_id
			 WHERE c.space_id = s.id AND u.disabled_at IS NULL) AS member_count
		FROM spaces s JOIN space_members m ON m.space_id = s.id
		WHERE m.user_id = ? ORDER BY lower(s.name)`, userID).Scan(&out).Error
	return out, err
}

// Create makes a space with userID as its owner.
func (s *Service) Create(ctx context.Context, userID, name string) (Summary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Summary{}, ErrInvalidName
	}
	id, err := newID()
	if err != nil {
		return Summary{}, err
	}
	err = s.commit(ctx, func(tx *gorm.DB) error {
		var taken int64
		if err := tx.Model(&schemas.Space{}).
			Where("lower(name) = lower(?) AND deleted_at IS NULL", name).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			return ErrNameTaken
		}
		if err := tx.Create(&schemas.Space{ID: id, Name: name}).Error; err != nil {
			return err
		}
		return tx.Create(&schemas.SpaceMember{SpaceID: id, UserID: userID, Role: wire.RoleOwner}).Error
	})
	return Summary{ID: id, Name: name, Role: wire.RoleOwner, MemberCount: 1}, err
}

// Get answers one space for one of its members.
func (s *Service) Get(ctx context.Context, userID, spaceID string) (Detail, error) {
	role, space, err := s.membership(s.db.WithContext(ctx), userID, spaceID)
	if err != nil {
		return Detail{}, err
	}
	var rows []memberRow
	err = s.db.WithContext(ctx).Raw(`SELECT m.space_id, m.user_id, u.email, u.name, m.role
		FROM space_members m JOIN users u ON u.id = m.user_id
		WHERE m.space_id = ? AND u.disabled_at IS NULL ORDER BY lower(u.email)`, spaceID).Scan(&rows).Error
	if err != nil {
		return Detail{}, err
	}
	members := make([]MemberView, 0, len(rows))
	for _, row := range rows {
		members = append(members, MemberView{
			UserID: row.UserID, Email: row.Email, Name: row.Name,
			Picture: s.picture(row.UserID), Role: row.Role,
		})
	}
	return Detail{ID: space.ID, Name: space.Name, Role: role, Members: members}, nil
}

// Rename changes a space's name everywhere. Admins and owners may.
func (s *Service) Rename(ctx context.Context, userID, spaceID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidName
	}
	return s.commit(ctx, func(tx *gorm.DB) error {
		if _, err := s.requireRank(tx, userID, spaceID, wire.RoleAdmin); err != nil {
			return err
		}
		var taken int64
		if err := tx.Model(&schemas.Space{}).
			Where("lower(name) = lower(?) AND id <> ? AND deleted_at IS NULL", name, spaceID).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			return ErrNameTaken
		}
		return tx.Model(&schemas.Space{}).Where("id = ?", spaceID).Update("name", name).Error
	})
}

// membership reads userID's role in spaceID, answering ErrNotFound when they
// are not in it, so a non-member cannot learn the space exists.
func (s *Service) membership(tx *gorm.DB, userID, spaceID string) (string, schemas.Space, error) {
	var space schemas.Space
	if err := tx.Where("id = ? AND deleted_at IS NULL", spaceID).Take(&space).Error; err != nil {
		return "", schemas.Space{}, notFound(err)
	}
	var member schemas.SpaceMember
	if err := tx.Where("space_id = ? AND user_id = ?", spaceID, userID).Take(&member).Error; err != nil {
		return "", schemas.Space{}, notFound(err)
	}
	return member.Role, space, nil
}

// requireRank locks the space row and answers the actor's role when it is at
// least min. The lock serialises every change to one space, which is what
// makes the last-owner count trustworthy.
func (s *Service) requireRank(tx *gorm.DB, userID, spaceID, min string) (string, error) {
	var locked schemas.Space
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", spaceID).Take(&locked).Error; err != nil {
		return "", notFound(err)
	}
	role, _, err := s.membership(tx, userID, spaceID)
	if err != nil {
		return "", err
	}
	if !s.ladder.AtLeast(spaces.Role(role), spaces.Role(min)) {
		return "", ErrForbidden
	}
	return role, nil
}

// notFound maps a missing row to ErrNotFound and passes anything else on.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
