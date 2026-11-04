package gdpr

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// Export is everything Registre holds about one person, as GDPR article 20
// asks. Secrets are left out on purpose: the password hash, the TOTP seed,
// recovery code digests and passkey public keys are not personal data the
// person can use, and handing them to whoever holds a session would weaken
// the account.
type Export struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Account     exportAccount   `json:"account"`
	Passkeys    []exportPasskey `json:"passkeys"`
	Consents    []exportConsent `json:"consents"`
	Sessions    []exportSession `json:"sessions"`
	Apps        []exportGrant   `json:"app_grants"`
	Spaces      []exportSpace   `json:"spaces"`
	Picture     bool            `json:"has_picture"`
	Entitlement schemas.JSONMap `json:"entitlements,omitempty"`
	Deletion    *time.Time      `json:"deletion_scheduled_for,omitempty"`
	Notice      string          `json:"notice"`
}

type exportAccount struct {
	Subject         string     `json:"sub"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Locale          string     `json:"locale"`
	Admin           bool       `json:"admin"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	TOTPEnrolled    bool       `json:"totp_enrolled"`
	HasPassword     bool       `json:"has_password"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type exportPasskey struct {
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type exportConsent struct {
	Document   string    `json:"document"`
	Version    string    `json:"version"`
	Locale     string    `json:"locale"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type exportSession struct {
	CreatedAt time.Time  `json:"created_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Apps      []string   `json:"apps"`
}

type exportGrant struct {
	App       string    `json:"app"`
	Roles     []string  `json:"roles"`
	GrantedAt time.Time `json:"granted_at"`
}

type exportSpace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// exportNotice tells the reader what the file does not cover.
const exportNotice = "This is the data Registre, the Facile identity provider, holds about you. " +
	"Each Facile app holds its own data and offers its own export."

// Export gathers user's data.
func (s *Service) Export(ctx context.Context, user schemas.User) (Export, error) {
	out := Export{GeneratedAt: s.now(), Notice: exportNotice, Account: exportAccount{
		Subject: user.ID, Email: user.Email, Name: user.Name, Locale: user.Locale, Admin: user.Admin,
		EmailVerifiedAt: user.EmailVerifiedAt, TOTPEnrolled: user.TOTPEnrolled(), HasPassword: user.PasswordHash != "",
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}}
	steps := []func(context.Context, string, *Export) error{
		s.exportPasskeys, s.exportConsents, s.exportSessions, s.exportGrants, s.exportRest,
	}
	for _, step := range steps {
		if err := step(ctx, user.ID, &out); err != nil {
			return Export{}, err
		}
	}
	return out, nil
}

func (s *Service) exportPasskeys(ctx context.Context, userID string, out *Export) error {
	var rows []schemas.Passkey
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at").Find(&rows).Error; err != nil {
		return fmt.Errorf("gdpr: export passkeys: %w", err)
	}
	out.Passkeys = make([]exportPasskey, 0, len(rows))
	for _, row := range rows {
		out.Passkeys = append(out.Passkeys, exportPasskey{Name: row.Name, CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt})
	}
	return nil
}

func (s *Service) exportConsents(ctx context.Context, userID string, out *Export) error {
	var rows []schemas.Consent
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Order("accepted_at, id").Find(&rows).Error; err != nil {
		return fmt.Errorf("gdpr: export consents: %w", err)
	}
	out.Consents = make([]exportConsent, 0, len(rows))
	for _, row := range rows {
		out.Consents = append(out.Consents, exportConsent{
			Document: row.Document, Version: row.Version, Locale: row.Locale, AcceptedAt: row.AcceptedAt,
		})
	}
	return nil
}

func (s *Service) exportSessions(ctx context.Context, userID string, out *Export) error {
	var rows []schemas.Session
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at").Find(&rows).Error; err != nil {
		return fmt.Errorf("gdpr: export sessions: %w", err)
	}
	out.Sessions = make([]exportSession, 0, len(rows))
	for _, row := range rows {
		out.Sessions = append(out.Sessions, exportSession{CreatedAt: row.CreatedAt, EndedAt: row.EndedAt, Apps: row.ClientIDs})
	}
	return nil
}

func (s *Service) exportGrants(ctx context.Context, userID string, out *Export) error {
	var rows []schemas.AppGrant
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return fmt.Errorf("gdpr: export grants: %w", err)
	}
	out.Apps = make([]exportGrant, 0, len(rows))
	for _, row := range rows {
		out.Apps = append(out.Apps, exportGrant{App: row.ClientID, Roles: row.Roles, GrantedAt: row.GrantedAt})
	}
	return nil
}

// exportRest reads the spaces, the picture flag, personal entitlements and
// the pending deletion.
func (s *Service) exportRest(ctx context.Context, userID string, out *Export) error {
	db := s.DB.WithContext(ctx)
	out.Spaces = []exportSpace{}
	err := db.Raw(`SELECT s.id, s.name, m.role FROM space_members m JOIN spaces s ON s.id = m.space_id
		WHERE m.user_id = ? AND s.deleted_at IS NULL ORDER BY s.name`, userID).Scan(&out.Spaces).Error
	if err != nil {
		return fmt.Errorf("gdpr: export spaces: %w", err)
	}
	var pictures int64
	if err := db.Model(&schemas.Avatar{}).Where("user_id = ?", userID).Count(&pictures).Error; err != nil {
		return fmt.Errorf("gdpr: export picture: %w", err)
	}
	out.Picture = pictures > 0
	var values []struct {
		Values schemas.JSONMap `gorm:"column:values"`
	}
	if err := db.Raw(`SELECT "values" FROM user_entitlements WHERE user_id = ?`, userID).Scan(&values).Error; err != nil {
		return fmt.Errorf("gdpr: export entitlements: %w", err)
	}
	if len(values) > 0 {
		out.Entitlement = values[0].Values
	}
	deletion, err := s.Scheduled(ctx, userID)
	out.Deletion = deletion
	return err
}
