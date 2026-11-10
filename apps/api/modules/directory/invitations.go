package directory

import (
	"net/http"
	"time"
)

// maxSubjectLength bounds the sub parameter; Registre subjects are UUIDs.
const maxSubjectLength = 64

// pendingInvitation is one invitation as an app shows it: enough for a
// menu row, and the URL of the page where the person answers it.
type pendingInvitation struct {
	ID        string    `json:"id"`
	SpaceName string    `json:"space_name"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	ExpiresAt time.Time `json:"expires_at"`
	URL       string    `json:"url"`
}

// invitations answers the live invitations addressed to the person whose
// subject is ?sub=, for the app's user menu. The answer has the same shape
// for an unknown, disabled or uninvited subject: {count: 0}. It reads
// nothing an app could not already see of that person in the snapshot
// besides the invitations themselves, and changes nothing; accepting stays
// on Registre's own page.
func (s *Service) invitations(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.app(w, r); !ok {
		return
	}
	subject := r.URL.Query().Get("sub")
	if subject == "" || len(subject) > maxSubjectLength {
		encode(w, map[string]string{"error": "sub is required"}, http.StatusBadRequest)
		return
	}
	found, err := s.spaces.Invited(r.Context(), subject)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]pendingInvitation, 0, len(found))
	for _, inv := range found {
		out = append(out, pendingInvitation{
			ID: inv.ID, SpaceName: inv.SpaceName, Role: inv.Role, InvitedBy: inv.InvitedBy,
			ExpiresAt: inv.ExpiresAt, URL: s.spaces.InvitationURL(inv.ID),
		})
	}
	encode(w, map[string]any{"count": len(out), "invitations": out}, http.StatusOK)
}
