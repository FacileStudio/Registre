package directory

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// invitationNotice is the mail an invitation sends, decided inside the
// transaction and sent after it commits.
type invitationNotice struct {
	to, locale, kind string
	vars             map[string]string
}

// noticeFor picks the mail: the invitation to a person with an account, the
// "join Facile" mail to an address without one while sign-up is open, and
// nothing otherwise. The caller answers the same in every case.
func (s *Service) noticeFor(tx *gorm.DB, row schemas.SpaceInvitation) (invitationNotice, error) {
	var space schemas.Space
	if err := tx.Where("id = ?", row.SpaceID).Take(&space).Error; err != nil {
		return invitationNotice{}, err
	}
	var inviter schemas.User
	if err := tx.Where("id = ?", row.InvitedBy).Take(&inviter).Error; err != nil {
		return invitationNotice{}, err
	}
	name := inviter.Name
	if name == "" {
		name = inviter.Email
	}
	vars := map[string]string{"inviter": name, "space": space.Name, "role": row.Role}
	var invitee schemas.User
	err := tx.Where("lower(email) = ? AND disabled_at IS NULL", row.Email).Take(&invitee).Error
	if err == nil {
		return invitationNotice{to: invitee.Email, locale: invitee.Locale, kind: i18n.MailSpaceInvite, vars: vars}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return invitationNotice{}, err
	}
	if !s.invitations.SignupOpen {
		return invitationNotice{}, nil
	}
	return invitationNotice{to: row.Email, locale: inviter.Locale, kind: i18n.MailSpaceJoin, vars: vars}, nil
}

// notify hands the notice to the mailer, which delivers asynchronously.
func (s *Service) notify(ctx context.Context, notice invitationNotice) {
	if notice.to == "" || s.invitations.Mailer == nil {
		return
	}
	link := s.invitations.Issuer + "/spaces"
	if notice.kind == i18n.MailSpaceJoin {
		link = s.invitations.Issuer + "/signup?lang=" + i18n.Normalize(notice.locale)
	}
	subject, body := i18n.Mail(notice.kind, notice.locale, link, notice.vars)
	if err := s.invitations.Mailer.Send(ctx, notice.to, subject, body); err != nil {
		s.logger.Warn("registre: space invitation mail failed", slog.Any("error", err))
	}
}

// Invited lists the live invitations addressed to userID's email.
func (s *Service) Invited(ctx context.Context, userID string) ([]MyInvitation, error) {
	out := []MyInvitation{}
	err := s.db.WithContext(ctx).Raw(`SELECT i.id, i.space_id, s.name AS space_name, i.role,
			COALESCE(NULLIF(inviter.name, ''), inviter.email) AS invited_by, i.expires_at
		FROM space_invitations i
		JOIN spaces s ON s.id = i.space_id AND s.deleted_at IS NULL
		JOIN users me ON me.id = ? AND lower(me.email) = lower(i.email)
		JOIN users inviter ON inviter.id = i.invited_by
		WHERE i.expires_at > ? ORDER BY i.created_at`, userID, time.Now().UTC()).Scan(&out).Error
	return out, err
}

// Accept turns an invitation addressed to userID into a membership at the
// offered role, bumping the directory version so the apps pick it up.
func (s *Service) Accept(ctx context.Context, userID, invitationID string) (Summary, error) {
	var joined Summary
	err := s.commit(ctx, func(tx *gorm.DB) error {
		row, err := s.claimInvitation(tx, userID, invitationID)
		if err != nil {
			return err
		}
		if _, _, err := s.membership(tx, userID, row.SpaceID); err == nil {
			return nil
		}
		joined = Summary{ID: row.SpaceID, Role: row.Role}
		return tx.Create(&schemas.SpaceMember{SpaceID: row.SpaceID, UserID: userID, Role: row.Role}).Error
	})
	return joined, err
}

// Decline drops an invitation addressed to userID.
func (s *Service) Decline(ctx context.Context, userID, invitationID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := s.claimInvitation(tx, userID, invitationID)
		return err
	})
}

// claimInvitation deletes and answers a live invitation whose address is
// userID's. Anyone else's invitation answers ErrInvitationNotFound.
func (s *Service) claimInvitation(tx *gorm.DB, userID, invitationID string) (schemas.SpaceInvitation, error) {
	var row schemas.SpaceInvitation
	err := tx.Raw(`DELETE FROM space_invitations i USING users u
		WHERE i.id = ? AND u.id = ? AND u.disabled_at IS NULL AND lower(u.email) = lower(i.email) AND i.expires_at > ?
		RETURNING i.*`, invitationID, userID, time.Now().UTC()).Scan(&row).Error
	if err != nil {
		return row, err
	}
	if row.ID == "" {
		return row, ErrInvitationNotFound
	}
	return row, nil
}

// PurgeInvitations deletes invitations that expired before cutoff.
func (s *Service) PurgeInvitations(ctx context.Context, cutoff time.Time) error {
	return s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.SpaceInvitation{}).Error
}
