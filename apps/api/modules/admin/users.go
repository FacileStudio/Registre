package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
)

// userView is what the dashboard sees of an account. PasswordHash never
// leaves the process, sealed or otherwise.
type userView struct {
	ID            string   `json:"id"`
	Email         string   `json:"email"`
	Name          string   `json:"name"`
	Admin         bool     `json:"admin"`
	EmailVerified bool     `json:"email_verified"`
	PictureURL    string   `json:"picture_url,omitempty"`
	LastSignInAt  *string  `json:"last_sign_in_at,omitempty"`
	Apps          []string `json:"apps"`
	TOTP          bool     `json:"totp"`
	CreatedAt     string   `json:"created_at"`
	DisabledAt    *string  `json:"disabled_at,omitempty"`
}

func viewUser(row schemas.User) userView {
	view := userView{
		ID: row.ID, Email: row.Email, Name: row.Name, Admin: row.Admin,
		EmailVerified: row.EmailVerified, Apps: []string{}, TOTP: row.TOTPConfirmedAt != nil,
		CreatedAt: row.CreatedAt.UTC().Format(timeFormat),
	}
	if row.DisabledAt != nil {
		at := row.DisabledAt.UTC().Format(timeFormat)
		view.DisabledAt = &at
	}
	return view
}

func (s *Service) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.users.List(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	views, err := s.userViews(r, rows)
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, views, http.StatusOK)
}

// createUser mints a human account the way `registre user-create` does: the
// operator attests the address by creating it, so it is verified by
// construction, and the password is hashed under the ceiling.
func (s *Service) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if body.Email == "" || body.Password == "" {
		encode(w, map[string]string{"error": "email and password are required"}, http.StatusBadRequest)
		return
	}
	row, err := s.users.Create(r.Context(), identity.NewUser{
		Email: body.Email, Password: body.Password, Name: body.Name,
		Admin: body.Admin,
	})
	if err != nil {
		serverError(w, err)
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	s.record(r, "user.create", userTarget(row.ID), map[string]any{"email": row.Email, "admin": row.Admin})
	s.logger.Info("registre: admin minted a user",
		slog.String("email", row.Email), slog.String("operator", operator.Email))
	encode(w, viewUser(row), http.StatusCreated)
}

func (s *Service) updateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	var body struct {
		Name  string `json:"name"`
		Admin *bool  `json:"admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	row, err := s.users.Update(r.Context(), id, body.Name, body.Admin)
	if err != nil {
		serverError(w, err)
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	s.record(r, "user.update", userTarget(id), map[string]any{"name": body.Name, "admin": body.Admin})
	s.logger.Info("registre: admin updated user",
		slog.String("user_id", id), slog.String("operator", operator.Email))
	encode(w, viewUser(row), http.StatusOK)
}

func (s *Service) setUserPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if len(body.Password) < 8 {
		encode(w, map[string]string{"error": "the new password is too short"}, http.StatusBadRequest)
		return
	}
	if err := s.users.SetPassword(r.Context(), id, body.Password); err != nil {
		serverError(w, err)
		return
	}
	reached, err := s.users.EndSessions(r.Context(), id, now())
	if err != nil {
		serverError(w, err)
		return
	}
	s.fireLogout(r, id, reached)
	operator := r.Context().Value(operatorKey).(schemas.User)
	s.record(r, "user.password", userTarget(id), nil)
	s.logger.Info("registre: admin updated password for user",
		slog.String("user_id", id), slog.String("operator", operator.Email))
	encode(w, map[string]any{"ok": true, "logout_targets": len(reached)}, http.StatusOK)
}

func (s *Service) disableUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	reached, err := s.users.Disable(r.Context(), id, now())
	if err != nil {
		serverError(w, err)
		return
	}
	s.fireLogout(r, id, reached)
	operator := r.Context().Value(operatorKey).(schemas.User)
	s.record(r, "user.disable", userTarget(id), nil)
	s.logger.Info("registre: admin disabled a user",
		slog.String("user_id", id), slog.String("operator", operator.Email))
	encode(w, map[string]any{"disabled": id, "logout_targets": len(reached)}, http.StatusOK)
}

func (s *Service) enableUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	if err := s.users.Enable(r.Context(), id); err != nil {
		serverError(w, err)
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	s.record(r, "user.enable", userTarget(id), nil)
	s.logger.Info("registre: admin enabled a user",
		slog.String("user_id", id), slog.String("operator", operator.Email))
	encode(w, map[string]any{"enabled": id}, http.StatusOK)
}

func (s *Service) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	operator := r.Context().Value(operatorKey).(schemas.User)
	if operator.ID == id {
		encode(w, map[string]string{"error": "cannot delete your own account"}, http.StatusBadRequest)
		return
	}
	if err := s.eraser.Erase(r.Context(), id); err != nil {
		serverError(w, err)
		return
	}
	s.record(r, "user.delete", userTarget(id), nil)
	s.logger.Info("registre: admin deleted user",
		slog.String("user_id", id), slog.String("operator", operator.Email))
	encode(w, map[string]any{"deleted": id}, http.StatusOK)
}

func (s *Service) fireLogout(r *http.Request, userID string, reached []string) {
	if s.logout == nil || len(reached) == 0 {
		return
	}
	if err := s.logout.Fire(r.Context(), userID, reached); err != nil {
		slog.Warn("registre: back-channel logout incomplete", slog.Any("error", err))
	}
}

// inviteUser records a single-use invitation and answers with its token,
// shown once — the same discipline as a client secret.
func (s *Service) inviteUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	body.Email = strings.TrimSpace(body.Email)
	if body.Email == "" {
		encode(w, map[string]string{"error": "email is required"}, http.StatusBadRequest)
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	token, err := s.invites.Issue(r.Context(), body.Email, operator.Email)
	if err != nil {
		encode(w, map[string]string{"error": "a valid email address is required"}, http.StatusBadRequest)
		return
	}
	s.record(r, "invite.create", audit.Target{Type: "invite", ID: body.Email}, nil)
	s.logger.Info("registre: admin invited a user",
		slog.String("email", body.Email), slog.String("operator", operator.Email))
	encode(w, map[string]any{"email": body.Email, "token": token}, http.StatusCreated)
}

func (s *Service) listInvites(w http.ResponseWriter, r *http.Request) {
	rows, err := s.invites.List(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	views := make([]inviteView, 0, len(rows))
	for _, row := range rows {
		views = append(views, inviteView{
			Email: row.Email, CreatedBy: row.CreatedBy,
			ExpiresAt: row.ExpiresAt.UTC().Format(timeFormat),
			CreatedAt: row.CreatedAt.UTC().Format(timeFormat),
			Redeemed:  row.RedeemedAt != nil,
		})
	}
	encode(w, views, http.StatusOK)
}

type inviteView struct {
	Email     string `json:"email"`
	CreatedBy string `json:"created_by"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
	Redeemed  bool   `json:"redeemed"`
}
