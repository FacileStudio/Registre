package account

import (
	"context"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// selfView is the sign-up era's half of the account page: the language,
// which ways in exist, a pending deletion and whether the current terms are
// still owed.
type selfView struct {
	Locale      string     `json:"locale"`
	HasPassword bool       `json:"has_password"`
	Passkeys    int64      `json:"passkeys"`
	DeletionAt  *time.Time `json:"deletion_scheduled_for"`
	ConsentOwed []string   `json:"consent_owed"`
}

// selfFor gathers the selfView of user. A deployment built without the
// sign-up services answers the locale and the password flag only.
func (s *Service) selfFor(ctx context.Context, user schemas.User) (selfView, error) {
	view := selfView{Locale: user.Locale, HasPassword: user.PasswordHash != "", ConsentOwed: []string{}}
	if view.Locale == "" {
		view.Locale = "en"
	}
	var err error
	if s.self.Passkeys != nil {
		if view.Passkeys, err = s.self.Passkeys.Count(ctx, user.ID); err != nil {
			return view, err
		}
	}
	if s.self.Rights != nil {
		if view.DeletionAt, err = s.self.Rights.Scheduled(ctx, user.ID); err != nil {
			return view, err
		}
	}
	if s.self.Consents != nil {
		owed, err := s.self.Consents.Outdated(ctx, user.ID)
		if err != nil {
			return view, err
		}
		view.ConsentOwed = append(view.ConsentOwed, owed...)
	}
	return view, nil
}
