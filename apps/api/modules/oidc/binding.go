package oidc

import (
	"context"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/flows"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Binding is what ties an auth request to one browser.
//
// zitadel's AuthorizeCallback hands a code to whoever names a finished auth
// request, and nothing in the library ties that request to a browser. Left
// alone, the request id is a bearer: anyone who started a flow and got a
// signed-in victim to finish it could collect the victim's code. Binding closes
// that at the one place a finished request turns into a code. The callback only
// answers the browser holding both the flow cookie /authorize set when the flow
// started and the account session that finished it, and only once.
type Binding struct {
	Storage  *Storage
	Sessions *accounts.Service
	Cookies  accounts.Cookies
}

type flowKey struct{}

// flowName is the cookie name for this deployment; see internal/flows.
func (b Binding) flowName(digest string) string {
	return flows.CookieName(digest, b.Cookies.Secure())
}

// authorize sets a fresh flow cookie on the browser and hands its digest to
// CreateAuthRequest through the request context, which zitadel passes on.
func (b Binding) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret, err := ids.Secret()
		if err != nil {
			http.Error(w, "server_error", http.StatusInternalServerError)
			return
		}
		digest := flows.Digest(secret)
		http.SetCookie(w, b.flowCookie(b.flowName(digest), secret, int(authTTL.Seconds())))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), flowKey{}, digest)))
	})
}

// callback refuses to turn a finished auth request into a code for any browser
// but the one that started and finished it, and refuses a request that already
// has a code. The library re-reads the same row afterwards, which is safe: a
// finished request cannot be finished again by someone else, and SaveAuthCode
// refuses a second code atomically.
func (b Binding) callback(provider *op.Provider, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			op.AuthRequestError(w, r, nil, zoidc.ErrInvalidRequest().WithDescription("cannot parse form"), provider)
			return
		}
		found, err := b.Storage.AuthRequestByID(r.Context(), r.Form.Get("id"))
		if err != nil {
			op.AuthRequestError(w, r, nil, err, provider)
			return
		}
		request, ok := found.(authRequest)
		if !ok {
			op.AuthRequestError(w, r, nil, zoidc.ErrServerError(), provider)
			return
		}
		name, ok := b.owns(r, request)
		if !ok {
			op.AuthRequestError(w, r, request,
				zoidc.ErrAccessDenied().WithDescription("this sign-in was not finished in this browser"), provider)
			return
		}
		http.SetCookie(w, b.flowCookie(name, "", -1))
		next.ServeHTTP(w, r)
	})
}

// owns reports whether the browser behind r started and finished request, and
// returns the flow cookie that proved it so the caller can clear it.
func (b Binding) owns(r *http.Request, request authRequest) (string, bool) {
	row := request.row
	if !row.Done() || row.Code != "" || row.SessionID == "" || row.BrowserHash == "" {
		return "", false
	}
	_, session, err := b.Sessions.Resolve(r.Context(), b.Cookies.Read(r))
	if err != nil || session.ID != row.SessionID {
		return "", false
	}
	return flows.Holds(r, row.BrowserHash, b.Cookies.Secure())
}

// flowCookie is the flow cookie with the account cookie's attributes: HttpOnly,
// Lax and path "/". Lax is enough because every hop that reads it is a
// top-level navigation on registre's own origin.
func (b Binding) flowCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   b.Cookies.Secure(),
		SameSite: http.SameSiteLaxMode,
	}
}

// browserHash is the flow digest authorize stored in ctx, or empty when the
// auth request was not created through the bound /authorize route.
func browserHash(ctx context.Context) string {
	digest, _ := ctx.Value(flowKey{}).(string)
	return digest
}
