package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"github.com/FacileStudio/Registre/apps/api/internal/directory"
	"github.com/FacileStudio/Registre/apps/api/schemas"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// appAccess is one application as seen from one user: whether they may sign
// in to it, through which rule, and the roles their grant carries there.
type appAccess struct {
	ClientID     string   `json:"client_id"`
	Name         string   `json:"name"`
	Policy       string   `json:"access_policy"`
	VisibleRoles []string `json:"visible_roles"`
	Granted      bool     `json:"granted"`
	Allowed      bool     `json:"allowed"`
	Roles        []string `json:"roles"`
	Disabled     bool     `json:"disabled"`
}

// grantView is one person holding access to an application.
type grantView struct {
	User      userView `json:"user"`
	Roles     []string `json:"roles"`
	GrantedAt string   `json:"granted_at"`
	GrantedBy string   `json:"granted_by"`
}

// getUser answers one account with its access to every application and the
// spaces it belongs to.
func (s *Service) getUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.ByID(r.Context(), chi.URLParam(r, "userID"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		encode(w, map[string]string{"error": "no such user"}, http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	apps, err := s.appsFor(r, user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	spaces := []directory.Summary{}
	if s.directory != nil {
		if spaces, err = s.directory.Mine(r.Context(), user.ID); err != nil {
			serverError(w, err)
			return
		}
	}
	views, err := s.userViews(r, []schemas.User{user})
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]any{"user": views[0], "apps": apps, "spaces": spaces}, http.StatusOK)
}

func (s *Service) appsFor(r *http.Request, userID string) ([]appAccess, error) {
	clients, err := s.register.List(r.Context())
	if err != nil {
		return nil, err
	}
	grants, err := s.access.UserGrants(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	held := make(map[string]schemas.AppGrant, len(grants))
	for _, grant := range grants {
		held[grant.ClientID] = grant
	}
	apps := []appAccess{}
	for _, client := range clients {
		if client.Kind != schemas.KindApplication {
			continue
		}
		grant, granted := held[client.ClientID]
		apps = append(apps, appAccess{
			ClientID: client.ClientID, Name: client.Name, Policy: client.AccessPolicy,
			VisibleRoles: nonNil(client.VisibleRoles), Granted: granted,
			Allowed: client.AccessPolicy != schemas.AccessAssigned || granted,
			Roles:   nonNil(grant.Roles), Disabled: client.DisabledAt != nil,
		})
	}
	return apps, nil
}

// clientAccess answers an application's policy and everyone granted to it.
func (s *Service) clientAccess(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	client, err := s.register.ByClientID(r.Context(), clientID)
	if err != nil {
		encode(w, map[string]string{"error": "no such client"}, http.StatusNotFound)
		return
	}
	views, err := s.grantViews(r, clientID)
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]any{
		"client_id": client.ClientID, "access_policy": client.AccessPolicy,
		"visible_roles": nonNil(client.VisibleRoles), "self": client.ClientID == s.self, "grants": views,
	}, http.StatusOK)
}

func (s *Service) grantViews(r *http.Request, clientID string) ([]grantView, error) {
	grants, err := s.access.Grants(r.Context(), clientID)
	if err != nil {
		return nil, err
	}
	views := make([]grantView, 0, len(grants))
	for _, grant := range grants {
		user, err := s.users.ByID(r.Context(), grant.UserID)
		if err != nil {
			continue
		}
		users, err := s.userViews(r, []schemas.User{user})
		if err != nil {
			return nil, err
		}
		views = append(views, grantView{
			User: users[0], Roles: nonNil(grant.Roles),
			GrantedAt: grant.GrantedAt.UTC().Format(timeFormat), GrantedBy: grant.GrantedBy,
		})
	}
	return views, nil
}

// setPolicy switches an application between everyone and assigned.
func (s *Service) setPolicy(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	var body struct {
		Policy string `json:"access_policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	err := s.access.SetPolicy(r.Context(), clientID, body.Policy)
	if !s.accessError(w, err) {
		return
	}
	s.record(r, "client.access_policy", clientTarget(clientID), map[string]any{"access_policy": body.Policy})
	encode(w, map[string]string{"access_policy": body.Policy}, http.StatusOK)
}

// putGrant gives a user access to an application with roles, or changes
// their roles there.
func (s *Service) putGrant(w http.ResponseWriter, r *http.Request) {
	clientID, userID := chi.URLParam(r, "clientID"), chi.URLParam(r, "userID")
	var body struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if _, err := s.users.ByID(r.Context(), userID); err != nil {
		encode(w, map[string]string{"error": "no such user"}, http.StatusNotFound)
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	grant, err := s.access.Grant(r.Context(), clientID, userID, body.Roles, operator.Email)
	if !s.accessError(w, err) {
		return
	}
	s.record(r, "grant.set", userTarget(userID), map[string]any{"client_id": clientID, "roles": nonNil(grant.Roles)})
	encode(w, map[string]any{"client_id": clientID, "user_id": userID, "roles": nonNil(grant.Roles)}, http.StatusOK)
}

// deleteGrant takes a user's access to an application away.
func (s *Service) deleteGrant(w http.ResponseWriter, r *http.Request) {
	clientID, userID := chi.URLParam(r, "clientID"), chi.URLParam(r, "userID")
	if !s.accessError(w, s.access.Revoke(r.Context(), clientID, userID)) {
		return
	}
	s.record(r, "grant.revoke", userTarget(userID), map[string]any{"client_id": clientID})
	encode(w, map[string]bool{"ok": true}, http.StatusOK)
}

// accessError writes the response for an access failure and reports whether
// the caller may go on.
func (s *Service) accessError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, access.ErrSelfLocked), errors.Is(err, access.ErrUnknownRole), errors.Is(err, access.ErrUnknownPolicy):
		encode(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
	case errors.Is(err, gorm.ErrRecordNotFound):
		encode(w, map[string]string{"error": "no such application"}, http.StatusNotFound)
	default:
		serverError(w, err)
	}
	return false
}
