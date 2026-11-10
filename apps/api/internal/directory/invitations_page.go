package directory

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ExpiredRetention is how long an expired invitation is kept, so its page
// can still say "expired" rather than "not found".
const ExpiredRetention = 30 * 24 * time.Hour

// The states an invitation page can find besides a live invitation.
var (
	ErrInvitationExpired      = errors.New("this invitation has expired")
	ErrInvitationWrongAccount = errors.New("this invitation was sent to another address")
)

// Invitation reads one invitation for its page, for userID, without
// changing anything. It answers ErrInvitationWrongAccount when the
// invitation is addressed to someone else (and says nothing more about it),
// ErrInvitationExpired when it is the caller's but too old, and
// ErrInvitationNotFound when it never existed, was withdrawn or answered, its
// space was deleted or its sender may no longer offer it.
func (s *Service) Invitation(ctx context.Context, userID, invitationID string) (MyInvitation, error) {
	var out MyInvitation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := liveSpaceInvitation(tx, invitationID)
		if err != nil {
			return err
		}
		if err := addressedTo(tx, row, userID); err != nil {
			return err
		}
		if !row.ExpiresAt.After(time.Now().UTC()) {
			return ErrInvitationExpired
		}
		if !s.stillEntitled(tx, row) {
			return ErrInvitationNotFound
		}
		out, err = s.describe(tx, row)
		return err
	})
	return out, err
}

// liveSpaceInvitation reads an invitation whose space still exists.
func liveSpaceInvitation(tx *gorm.DB, invitationID string) (schemas.SpaceInvitation, error) {
	var row schemas.SpaceInvitation
	err := tx.Raw(`SELECT i.* FROM space_invitations i JOIN spaces s ON s.id = i.space_id AND s.deleted_at IS NULL
		WHERE i.id = ?`, invitationID).Scan(&row).Error
	if err != nil {
		return row, err
	}
	if row.ID == "" {
		return row, ErrInvitationNotFound
	}
	return row, nil
}

// addressedTo refuses an invitation that is not for userID's address.
func addressedTo(tx *gorm.DB, row schemas.SpaceInvitation, userID string) error {
	var mine int64
	err := tx.Model(&schemas.User{}).
		Where("id = ? AND disabled_at IS NULL AND lower(email) = lower(?)", userID, row.Email).Count(&mine).Error
	if err != nil {
		return err
	}
	if mine == 0 {
		return ErrInvitationWrongAccount
	}
	return nil
}

// describe is row as its addressee reads it.
func (s *Service) describe(tx *gorm.DB, row schemas.SpaceInvitation) (MyInvitation, error) {
	var space schemas.Space
	if err := tx.Where("id = ?", row.SpaceID).Take(&space).Error; err != nil {
		return MyInvitation{}, err
	}
	var inviter schemas.User
	if err := tx.Where("id = ?", row.InvitedBy).Take(&inviter).Error; err != nil {
		return MyInvitation{}, err
	}
	return MyInvitation{
		ID: row.ID, SpaceID: row.SpaceID, SpaceName: space.Name, Role: row.Role,
		InvitedBy: displayName(inviter), ExpiresAt: row.ExpiresAt,
	}, nil
}

// InvitationURL is the page where the invitee reads and answers invitationID.
func (s *Service) InvitationURL(invitationID string) string {
	return s.invitations.Issuer + "/invitations/" + url.PathEscape(invitationID)
}

// displayName is how a mail names a person: their name, else their address.
func displayName(user schemas.User) string {
	if name := strings.TrimSpace(user.Name); name != "" {
		return name
	}
	return user.Email
}

// PurgeInvitations deletes invitations that expired more than
// ExpiredRetention before cutoff and the mail log older than a day before it.
func (s *Service) PurgeInvitations(ctx context.Context, cutoff time.Time) error {
	expired := cutoff.Add(-ExpiredRetention)
	if err := s.db.WithContext(ctx).Where("expires_at < ?", expired).Delete(&schemas.SpaceInvitation{}).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec("DELETE FROM invitation_mails WHERE sent_at < ?", cutoff.Add(-24*time.Hour)).Error
}
