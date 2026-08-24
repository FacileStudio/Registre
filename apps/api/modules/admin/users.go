package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

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
	Roles         []string `json:"roles,omitempty"`
	EmailVerified bool     `json:"email_verified"`
	CreatedAt     string   `json:"created_at"`
	DisabledAt    *string  `json:"disabled_at,omitempty"`
}

func viewUser(row schemas.User) userView {
	view := userView{
		ID: row.ID, Email: row.Email, Name: row.Name, Admin: row.Admin,
		Roles: row.Roles, EmailVerified: row.EmailVerified,
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
	views := make([]userView, 0, len(rows))
	for _, row := range rows {
		views = append(views, viewUser(row))
	}
	encode(w, views, http.StatusOK)
}

// createUser mints a human account the way `registre user-create` does: the
// operator attests the address by creating it, so it is verified by
// construction, and the password is hashed under the ceiling.
func (s *Service) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string   `json:"email"`
		Name     string   `json:"name"`
		Password string   `json:"password"`
		Admin    bool     `json:"admin"`
		Roles    []string `json:"roles"`
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
		Admin: body.Admin, Roles: body.Roles,
	})
	if err != nil {
		serverError(w, err)
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	s.logger.Info("registre: admin minted a user",
		slog.String("email", row.Email), slog.String("operator", operator.Email))
	encode(w, viewUser(row), http.StatusCreated)
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
	s.logger.Info("registre: admin disabled a user",
		slog.String("user_id", id), slog.String("operator", operator.Email))
	encode(w, map[string]any{"disabled": id, "logout_targets": len(reached)}, http.StatusOK)
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
