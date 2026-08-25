package accounts

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// Guard is the pair of middlewares every route reached with the account cookie
// sits behind.
//
// It lives beside the session rather than in the module that first needed it
// because there is now more than one: the account pages manage a human's own
// security, and the device verification page approves a CLI's login, and both
// are "the human holding this cookie, proving the request came from a page we
// served". A second copy of a session check is a second place for one of them
// to drift into accepting something the other refuses.
type Guard struct {
	sessions *Service
	cookies  Cookies
}

// NewGuard builds the guard over a session service and a cookie policy.
func NewGuard(sessions *Service, cookies Cookies) Guard {
	return Guard{sessions: sessions, cookies: cookies}
}

type ctxKey int

const (
	userKey    ctxKey = 0
	sessionKey ctxKey = 1
)

// CSRF refuses a mutating request that does not carry the header no
// cross-origin page can set without a preflight this API never answers.
//
// It runs ahead of Session so a forged write costs a header comparison rather
// than a database read. GET is exempt: it changes nothing, and requiring the
// header there would break the page's first load.
func (g Guard) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Header.Get(CSRFHeader) == "" {
			write(w, map[string]string{"error": "forbidden"}, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Session resolves the account cookie and refuses every request that does not
// carry a live one.
//
// Absent, unknown, expired and belonging-to-a-disabled-account all answer with
// the same 401. Saying which would tell whoever holds a stolen cookie what
// they are holding.
func (g Guard) Session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, session, err := g.sessions.Resolve(r.Context(), g.cookies.Read(r))
		if err != nil {
			write(w, map[string]string{"error": "unauthorized"}, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, sessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Holder returns the human behind the request and the session they are holding.
//
// The session comes back as well as the user because its CreatedAt is when
// that human actually authenticated, and a route that stamps an auth_time onto
// something — the device grant does — must not invent one. Every caller is
// behind Session, so the absent case is a routing mistake rather than a
// runtime one and answers with zero values.
func Holder(ctx context.Context) (schemas.User, schemas.Session) {
	user, _ := ctx.Value(userKey).(schemas.User)
	session, _ := ctx.Value(sessionKey).(schemas.Session)
	return user, session
}

// write emits one JSON object with the given status. A write failure after the
// status is sent can only be logged; the stream is already committed.
func write(w http.ResponseWriter, body any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("registre: refusing a request failed to write", slog.Any("error", err))
	}
}
