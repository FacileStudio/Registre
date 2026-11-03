package admin

import (
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// userViews decorates accounts with what the dashboard lists beside them:
// their picture, when they last signed in and the apps they hold a grant to.
func (s *Service) userViews(r *http.Request, rows []schemas.User) ([]userView, error) {
	last, err := s.users.LastSignIns(r.Context())
	if err != nil {
		return nil, err
	}
	views := make([]userView, 0, len(rows))
	for _, row := range rows {
		view, err := s.decorate(r, row, last)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) decorate(r *http.Request, row schemas.User, last map[string]time.Time) (userView, error) {
	view := viewUser(row)
	if at, ok := last[row.ID]; ok {
		formatted := at.UTC().Format(timeFormat)
		view.LastSignInAt = &formatted
	}
	var err error
	if s.avatars != nil {
		if view.PictureURL, err = s.avatars.Versioned(r.Context(), row.ID); err != nil {
			return view, err
		}
	}
	if s.access == nil {
		return view, nil
	}
	grants, err := s.access.UserGrants(r.Context(), row.ID)
	for _, grant := range grants {
		view.Apps = append(view.Apps, grant.ClientID)
	}
	return view, err
}

// nonNil answers an empty list in place of nil, so JSON carries [] not null.
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
