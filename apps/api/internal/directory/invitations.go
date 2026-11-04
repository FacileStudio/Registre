package directory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/address"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/FacileStudio/porte/spaces"
	"gorm.io/gorm"
)

// InvitationTTL is how long an invitation into a space stays acceptable.
const InvitationTTL = 14 * 24 * time.Hour

// MaxMailsPerRecipient caps the invitation mails one address receives in a
// day, whoever sends them, and MaxMailsPerHour the invitation mails the whole
// instance sends in an hour. Past either cap the invitation is still written
// and answered the same way; only the mail is skipped.
const (
	MaxMailsPerRecipient = 3
	MaxMailsPerHour      = 200
)

// MaxPendingPerSpace caps the invitations one space may have outstanding, so
// a space cannot be used to mail thousands of strangers.
const MaxPendingPerSpace = 50

// Invitation errors. ErrInvalidEmail is decided from the string alone.
var (
	ErrInvalidEmail       = errors.New("this is not an email address")
	ErrTooManyInvitations = errors.New("this space has too many pending invitations")
	ErrInvitationNotFound = errors.New("invitation not found")
)

// Mailer sends one mail; recovery.MailerFunc satisfies it.
type Mailer interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Invitations is how invitation notices go out: through mailer, with links
// on issuer, and to addresses without an account only while sign-up is
// open.
type Invitations struct {
	Mailer     Mailer
	Issuer     string
	SignupOpen bool
}

// InvitationView is one pending invitation, as the space's admins see it.
type InvitationView struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MyInvitation is one invitation addressed to the signed-in person.
type MyInvitation struct {
	ID        string    `json:"id"`
	SpaceID   string    `json:"space_id"`
	SpaceName string    `json:"space_name"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	ExpiresAt time.Time `json:"expires_at"`
}

// WithInvitations wires the notices invitations send.
func (s *Service) WithInvitations(cfg Invitations) *Service {
	cfg.Issuer = strings.TrimRight(cfg.Issuer, "/")
	s.invitations = cfg
	return s
}

// Invite offers role in spaceID to email. The actor must be an admin or
// owner and may not offer a rank above their own. The answer is the same
// whether or not the address has an account; only a current member is
// refused, which the actor can already see in the member list. Inviting the
// same address again refreshes the invitation.
func (s *Service) Invite(ctx context.Context, actorID, spaceID, email, role string) error {
	if !validRole(role) {
		return ErrInvalidRole
	}
	clean, ok := address.Clean(email)
	if !ok {
		return ErrInvalidEmail
	}
	var notice invitationNotice
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		notice, err = s.offer(tx, actorID, spaceID, clean, role)
		return err
	})
	if err != nil {
		return err
	}
	s.notify(ctx, notice)
	return nil
}

// offer checks the actor and writes the invitation inside tx.
func (s *Service) offer(tx *gorm.DB, actorID, spaceID, email, role string) (invitationNotice, error) {
	actorRole, err := s.requireRank(tx, actorID, spaceID, "admin")
	if err != nil {
		return invitationNotice{}, err
	}
	if !s.ladder.AtLeast(spaces.Role(actorRole), spaces.Role(role)) {
		return invitationNotice{}, ErrForbidden
	}
	if err := admissible(tx, spaceID, email); err != nil {
		return invitationNotice{}, err
	}
	id, err := ids.New()
	if err != nil {
		return invitationNotice{}, err
	}
	now := time.Now().UTC()
	row := schemas.SpaceInvitation{
		ID: id, SpaceID: spaceID, Email: email, Role: role, InvitedBy: actorID,
		CreatedAt: now, ExpiresAt: now.Add(InvitationTTL),
	}
	err = tx.Exec(`INSERT INTO space_invitations (id, space_id, email, role, invited_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (space_id, lower(email)) DO UPDATE SET role = EXCLUDED.role,
			invited_by = EXCLUDED.invited_by, created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at`,
		row.ID, row.SpaceID, row.Email, row.Role, row.InvitedBy, row.CreatedAt, row.ExpiresAt).Error
	if err != nil {
		return invitationNotice{}, fmt.Errorf("directory: record the invitation: %w", err)
	}
	return s.noticeFor(tx, row)
}

// admissible refuses an address already in the space, and a new invitation
// past the space's cap.
func admissible(tx *gorm.DB, spaceID, email string) error {
	var members int64
	err := tx.Raw(`SELECT count(*) FROM space_members m JOIN users u ON u.id = m.user_id
		WHERE m.space_id = ? AND lower(u.email) = ?`, spaceID, email).Scan(&members).Error
	if err != nil {
		return err
	}
	if members > 0 {
		return ErrAlreadyMember
	}
	return capPending(tx, spaceID, email)
}

// capPending refuses a new invitation past MaxPendingPerSpace live ones; a
// refresh of an existing one is always allowed.
func capPending(tx *gorm.DB, spaceID, email string) error {
	var pending int64
	err := tx.Model(&schemas.SpaceInvitation{}).
		Where("space_id = ? AND expires_at > ? AND lower(email) <> ?", spaceID, time.Now().UTC(), email).
		Count(&pending).Error
	if err != nil {
		return err
	}
	if pending >= MaxPendingPerSpace {
		return ErrTooManyInvitations
	}
	return nil
}

// Revoke withdraws a pending invitation. Admins and owners may.
func (s *Service) Revoke(ctx context.Context, actorID, spaceID, invitationID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.requireRank(tx, actorID, spaceID, "admin"); err != nil {
			return err
		}
		result := tx.Where("id = ? AND space_id = ?", invitationID, spaceID).Delete(&schemas.SpaceInvitation{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvitationNotFound
		}
		return nil
	})
}

// pendingFor lists a space's live invitations for its admins.
func (s *Service) pendingFor(tx *gorm.DB, spaceID string) ([]InvitationView, error) {
	out := []InvitationView{}
	err := tx.Model(&schemas.SpaceInvitation{}).
		Where("space_id = ? AND expires_at > ?", spaceID, time.Now().UTC()).
		Order("created_at").Scan(&out).Error
	return out, err
}
