package directory

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// invitationNotice is the mail an invitation sends, decided inside the
// transaction and sent after it commits.
type invitationNotice struct {
	to, locale, kind, inviter string
	invitation, space, role   string
	inviterName               string
	expires                   time.Time
}

// noticeFor picks the mail: the invitation to a person with an account, the
// "join Facile" mail to an address without one while sign-up is open, and
// nothing otherwise. The caller answers the same in every case.
func (s *Service) noticeFor(tx *gorm.DB, row schemas.SpaceInvitation) (invitationNotice, error) {
	notice, err := s.pickNotice(tx, row)
	if err != nil || notice.to == "" {
		return notice, err
	}
	return logMail(tx, notice)
}

// pickNotice decides which mail, if any, an invitation would send, and
// gathers what it says: the inviter's display name (else their address),
// the space's name, the role and the expiry.
func (s *Service) pickNotice(tx *gorm.DB, row schemas.SpaceInvitation) (invitationNotice, error) {
	var inviter schemas.User
	if err := tx.Where("id = ?", row.InvitedBy).Take(&inviter).Error; err != nil {
		return invitationNotice{}, err
	}
	var space schemas.Space
	if err := tx.Where("id = ?", row.SpaceID).Take(&space).Error; err != nil {
		return invitationNotice{}, err
	}
	notice := invitationNotice{
		inviter: inviter.ID, invitation: row.ID, space: space.Name, role: row.Role,
		inviterName: displayName(inviter), expires: row.ExpiresAt,
	}
	var invitee schemas.User
	err := tx.Where("lower(email) = ? AND disabled_at IS NULL", row.Email).Take(&invitee).Error
	if err == nil {
		notice.to, notice.locale, notice.kind = invitee.Email, invitee.Locale, i18n.MailSpaceInvite
		return notice, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return invitationNotice{}, err
	}
	if !s.invitations.SignupOpen {
		return invitationNotice{}, nil
	}
	notice.to, notice.locale, notice.kind = row.Email, inviter.Locale, i18n.MailSpaceJoin
	return notice, nil
}

// logMail counts the invitation mails actually sent, to this address in the
// last day and by the instance in the last hour, from invitation_mails, and
// either logs this one or drops it when a cap is reached. Counting mails
// rather than invitation rows is the point: refreshing one invitation
// rewrites its row, and would otherwise mail without limit.
func logMail(tx *gorm.DB, notice invitationNotice) (invitationNotice, error) {
	now := time.Now().UTC()
	var toRecipient, byInviter, overall int64
	err := tx.Table("invitation_mails").
		Where("lower(email) = lower(?) AND sent_at > ?", notice.to, now.Add(-24*time.Hour)).Count(&toRecipient).Error
	if err != nil {
		return invitationNotice{}, err
	}
	err = tx.Table("invitation_mails").
		Where("inviter_id = ? AND sent_at > ?", notice.inviter, now.Add(-24*time.Hour)).Count(&byInviter).Error
	if err != nil {
		return invitationNotice{}, err
	}
	if err := tx.Table("invitation_mails").Where("sent_at > ?", now.Add(-time.Hour)).Count(&overall).Error; err != nil {
		return invitationNotice{}, err
	}
	if toRecipient >= MaxMailsPerRecipient || byInviter >= MaxMailsPerInviter || overall >= MaxMailsPerHour {
		return invitationNotice{}, nil
	}
	err = tx.Exec("INSERT INTO invitation_mails (email, inviter_id, sent_at) VALUES (lower(?), ?, ?)", notice.to, notice.inviter, now).Error
	if err != nil {
		return invitationNotice{}, err
	}
	return notice, nil
}

// notify hands the notice to the mailer, which delivers asynchronously.
// The invitation mail links to the invitation's own page; the join mail to
// sign-up with the invitation attached, so the new account lands on it.
func (s *Service) notify(ctx context.Context, notice invitationNotice) {
	if notice.to == "" || s.invitations.Mailer == nil {
		return
	}
	link := s.InvitationURL(notice.invitation)
	if notice.kind == i18n.MailSpaceJoin {
		query := url.Values{"lang": {i18n.Normalize(notice.locale)}, "invitation": {notice.invitation}}
		link = s.invitations.Issuer + "/signup?" + query.Encode()
	}
	subject, body := i18n.Mail(notice.kind, notice.locale, link, map[string]string{
		"inviter": notice.inviterName, "space": notice.space,
		"role": i18n.RoleName(notice.role, notice.locale), "date": i18n.Date(notice.expires, notice.locale),
	})
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
		JOIN users me ON me.id = ? AND me.disabled_at IS NULL AND lower(me.email) = lower(i.email)
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
		if !s.stillEntitled(tx, row) {
			return ErrInvitationNotFound
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
	err := tx.Raw(`DELETE FROM space_invitations i USING users u, spaces s
		WHERE i.id = ? AND u.id = ? AND u.disabled_at IS NULL AND lower(u.email) = lower(i.email) AND i.expires_at > ?
		  AND s.id = i.space_id AND s.deleted_at IS NULL
		RETURNING i.*`, invitationID, userID, time.Now().UTC()).Scan(&row).Error
	if err != nil {
		return row, err
	}
	if row.ID == "" {
		return row, ErrInvitationNotFound
	}
	return row, nil
}
