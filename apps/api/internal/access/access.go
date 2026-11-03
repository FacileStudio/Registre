// Package access decides who may use which application and with which roles.
//
// An application is open to every enabled user until an operator sets its
// policy to assigned, after which only users holding a grant may sign in.
// Roles live on the grant, per user and per app, and what reaches a token is
// the grant's roles intersected with the client's visible roles (SPEC §6).
// Every path that hands a human a token for an app asks Allowed first.
package access

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNoAccess means the user may not use the application.
var ErrNoAccess = errors.New("access: the user may not use this application")

// ErrSelfLocked refuses restricting registre's own client, which would lock
// every operator out of the dashboard.
var ErrSelfLocked = errors.New("access: registre's own client stays open to everyone")

// ErrUnknownRole refuses a role the application does not declare as visible.
var ErrUnknownRole = errors.New("access: the application does not declare that role")

// ErrUnknownPolicy refuses a policy other than everyone or assigned.
var ErrUnknownPolicy = errors.New("access: unknown access policy")

// Service reads and changes access. self is registre's own client id.
type Service struct {
	db   *gorm.DB
	self string
}

// New builds the access service.
func New(db *gorm.DB, self string) *Service {
	return &Service{db: db, self: self}
}

// Allowed reports whether userID may sign in to clientID. Only an application
// set to AccessAssigned restricts anyone: a service account has no human
// policy, and a client id with no row carries no policy either, since every
// flow that reaches here was already started by a registered client.
func (s *Service) Allowed(ctx context.Context, clientID, userID string) (bool, error) {
	var client schemas.Client
	err := s.db.WithContext(ctx).Select("kind", "access_policy").Where("client_id = ?", clientID).Take(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("access: read the client: %w", err)
	}
	if client.Kind != schemas.KindApplication || client.AccessPolicy != schemas.AccessAssigned {
		return true, nil
	}
	var count int64
	err = s.db.WithContext(ctx).Model(&schemas.AppGrant{}).
		Where("client_id = ? AND user_id = ?", clientID, userID).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("access: read the grant: %w", err)
	}
	return count > 0, nil
}

// Require answers ErrNoAccess when Allowed is false.
func (s *Service) Require(ctx context.Context, clientID, userID string) error {
	ok, err := s.Allowed(ctx, clientID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoAccess
	}
	return nil
}

// Roles answers the roles userID holds in clientID that the client may see,
// never nil, so a token can carry an honest empty claim.
func (s *Service) Roles(ctx context.Context, clientID, userID string) ([]string, error) {
	var client schemas.Client
	err := s.db.WithContext(ctx).Select("visible_roles").Where("client_id = ?", clientID).Take(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("access: read the client: %w", err)
	}
	var grant schemas.AppGrant
	err = s.db.WithContext(ctx).Where("client_id = ? AND user_id = ?", clientID, userID).Take(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("access: read the grant: %w", err)
	}
	visible := []string{}
	for _, role := range grant.Roles {
		if client.VisibleRoles.Contains(role) {
			visible = append(visible, role)
		}
	}
	return visible, nil
}

// Grant gives userID access to clientID with roles, replacing any earlier
// grant. Every role must be one the client declares as visible.
func (s *Service) Grant(ctx context.Context, clientID, userID string, roles []string, by string) (schemas.AppGrant, error) {
	var client schemas.Client
	if err := s.db.WithContext(ctx).Select("visible_roles").Where("client_id = ?", clientID).Take(&client).Error; err != nil {
		return schemas.AppGrant{}, fmt.Errorf("access: read the client: %w", err)
	}
	for _, role := range roles {
		if !client.VisibleRoles.Contains(role) {
			return schemas.AppGrant{}, fmt.Errorf("%w: %q", ErrUnknownRole, role)
		}
	}
	grant := schemas.AppGrant{
		ClientID: clientID, UserID: userID, Roles: schemas.StringList(slices.Compact(slices.Sorted(slices.Values(roles)))),
		GrantedAt: time.Now().UTC(), GrantedBy: by,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"roles", "granted_at", "granted_by"}),
	}).Create(&grant).Error
	if err != nil {
		return schemas.AppGrant{}, fmt.Errorf("access: store the grant: %w", err)
	}
	return grant, nil
}

// Revoke removes userID's grant to clientID. When the app is restricted it
// also revokes their refresh tokens for it, so access ends within one access
// token lifetime instead of lasting as long as the refresh token.
func (s *Service) Revoke(ctx context.Context, clientID, userID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("client_id = ? AND user_id = ?", clientID, userID).Delete(&schemas.AppGrant{}).Error; err != nil {
			return fmt.Errorf("access: delete the grant: %w", err)
		}
		var policy string
		if err := tx.Model(&schemas.Client{}).Select("access_policy").Where("client_id = ?", clientID).Scan(&policy).Error; err != nil {
			return fmt.Errorf("access: read the policy: %w", err)
		}
		if policy != schemas.AccessAssigned {
			return nil
		}
		return revokeRefresh(tx, clientID, "user_id = ?", userID)
	})
}

// SetPolicy changes who may use clientID. Restricting it revokes the refresh
// tokens of every user without a grant. Registre's own client cannot be
// restricted.
func (s *Service) SetPolicy(ctx context.Context, clientID, policy string) error {
	if policy != schemas.AccessEveryone && policy != schemas.AccessAssigned {
		return ErrUnknownPolicy
	}
	if clientID == s.self && policy == schemas.AccessAssigned {
		return ErrSelfLocked
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&schemas.Client{}).Where("client_id = ? AND kind = ?", clientID, schemas.KindApplication).
			Update("access_policy", policy)
		if result.Error != nil {
			return fmt.Errorf("access: set the policy: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if policy != schemas.AccessAssigned {
			return nil
		}
		return revokeRefresh(tx, clientID,
			"user_id NOT IN (SELECT user_id FROM app_grants WHERE client_id = ?)", clientID)
	})
}
