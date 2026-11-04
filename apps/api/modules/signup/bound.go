package signup

import (
	"context"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/flows"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// bound answers id when it names a live, unfinished app login that this
// browser started, proven by its flow cookie, and "" otherwise. An auth
// request id alone is not a credential: carrying one the starter merely
// named would let anyone have somebody else's login finished.
func (s *Service) bound(r *http.Request, id string) string {
	if _, ok := s.liveAndHeld(r.Context(), r, id); !ok {
		return ""
	}
	return id
}

// liveAndHeld reads the auth request and reports whether it is live,
// unfinished and bound to the browser behind r.
func (s *Service) liveAndHeld(ctx context.Context, r *http.Request, id string) (schemas.AuthRequest, bool) {
	if id == "" || s.Users == nil {
		return schemas.AuthRequest{}, false
	}
	row, err := s.Users.AuthRequestByID(ctx, id)
	if err != nil || !row.ExpiresAt.After(time.Now().UTC()) || row.Done() {
		return schemas.AuthRequest{}, false
	}
	if _, held := flows.Holds(r, row.BrowserHash, s.Cookies.Secure()); !held {
		return schemas.AuthRequest{}, false
	}
	return row, true
}
