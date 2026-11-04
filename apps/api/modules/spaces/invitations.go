package spaces

import (
	"net/http"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/go-chi/chi/v5"
)

// InviteLimit and InviteBurst are the budget each inviter, and separately
// each space, gets for sending invitations: twenty at once, then one a
// minute. Every invitation can mail a stranger, so the budget is small.
const (
	InviteLimit = 1.0 / 60
	InviteBurst = 20
)

// inviteBudget limits invitations per inviter and per space.
func inviteBudget() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{
			Limit: InviteLimit, Burst: InviteBurst, KeyFunc: planKey,
		}),
		troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{
			Limit: InviteLimit, Burst: InviteBurst,
			KeyFunc: func(r *http.Request) string { return "space:" + chi.URLParam(r, "spaceID") },
		}),
	}
}

// invite offers a role in the space to an email. It answers 202 "invited"
// whether or not the address has an account; nobody joins until they accept.
func (s *Service) invite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &body) {
		return
	}
	err := s.spaces.Invite(r.Context(), holderID(r), chi.URLParam(r, "spaceID"), body.Email, body.Role)
	if err == nil {
		encode(w, map[string]string{"status": "invited"}, http.StatusAccepted)
		return
	}
	s.answer(w, err, nil)
}

// revokeInvitation withdraws a pending invitation.
func (s *Service) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	err := s.spaces.Revoke(r.Context(), holderID(r), chi.URLParam(r, "spaceID"), chi.URLParam(r, "invitationID"))
	s.answer(w, err, map[string]bool{"ok": true})
}

// myInvitations lists the invitations addressed to the caller.
func (s *Service) myInvitations(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.Invited(r.Context(), holderID(r))
	s.answer(w, err, map[string]any{"invitations": out})
}

// acceptInvitation joins the space at the offered role.
func (s *Service) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.Accept(r.Context(), holderID(r), chi.URLParam(r, "invitationID"))
	s.answer(w, err, out)
}

// declineInvitation drops the invitation.
func (s *Service) declineInvitation(w http.ResponseWriter, r *http.Request) {
	err := s.spaces.Decline(r.Context(), holderID(r), chi.URLParam(r, "invitationID"))
	s.answer(w, err, map[string]bool{"ok": true})
}
