package login

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/identity"
)

// Service is the login UI: the pages a human sees between the app that sent
// them here and the code that sends them back.
type Service struct {
	identity *identity.Service
	logger   *slog.Logger
	now      func() time.Time
}

// New builds the login service over identity.
func New(identityService *identity.Service, logger *slog.Logger) *Service {
	return &Service{identity: identityService, logger: logger, now: func() time.Time { return time.Now().UTC() }}
}

var form = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in — registre</title>
<style>
  body { font-family: system-ui, sans-serif; display: grid; place-items: center;
         min-height: 100vh; margin: 0; background: #f6f6f4; color: #1a1a18; }
  main { background: #fff; padding: 2.5rem; border-radius: 12px; width: 20rem;
         box-shadow: 0 1px 3px rgba(0,0,0,.08); }
  h1 { font-size: 1.15rem; margin: 0 0 1.5rem; }
  label { display: block; font-size: .85rem; margin-bottom: .25rem; }
  input { width: 100%; box-sizing: border-box; padding: .55rem .7rem; margin-bottom: 1rem;
          border: 1px solid #ccc7bf; border-radius: 8px; font-size: 1rem; }
  button { width: 100%; padding: .6rem; border: 0; border-radius: 8px;
           background: #1a1a18; color: #fff; font-size: 1rem; cursor: pointer; }
  .error { color: #a32121; font-size: .85rem; margin-bottom: 1rem; }
</style>
</head>
<body>
<main>
<h1>Sign in to registre</h1>
{{if .Error}}<p class="error">Wrong email or password.</p>{{end}}
<form method="post" action="/login">
  <input type="hidden" name="auth_request" value="{{.AuthRequest}}">
  <label for="email">Email</label>
  <input id="email" name="email" type="email" autocomplete="username" required autofocus>
  <label for="password">Password</label>
  <input id="password" name="password" type="password" autocomplete="current-password" required>
  <button type="submit">Sign in</button>
</form>
</main>
</body>
</html>
`))

// home is what a human poking at the domain root sees. It says what this is
// instead of answering 404, and links nothing because there is nothing yet.
func (s *Service) home(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8">
<title>registre</title></head><body><p>registre is the Facile Suite's identity provider.
There is nothing to see here; sign-in starts from the app you are trying to reach.</p>
</body></html>`))
	if err != nil {
		s.logger.Error("registre: failed to write the home page", slog.Any("error", err))
	}
}

// form renders the sign-in page for a pending browser flow.
//
// An unknown or expired auth request gets the same page as any other failure
// state rather than an explanation: telling a visitor their login link died
// tells a phisher theirs did not.
func (s *Service) form(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("auth_request")
	if id == "" {
		http.Error(w, "missing auth request", http.StatusBadRequest)
		return
	}
	s.render(w, struct{ AuthRequest, Error string }{AuthRequest: id})
}

// submit checks the credentials and, on success, marks the flow
// authenticated and sends the browser back to zitadel's callback, which is
// what issues the code and redirects into the app.
//
// Every refusal re-renders the same form with the same generic message. The
// identity service already equalises timing between unknown address and wrong
// password, so nothing here can distinguish them either.
func (s *Service) submit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PostFormValue("auth_request")
	email := r.PostFormValue("email")
	password := r.PostFormValue("password")

	authReq, err := s.identity.AuthRequestByID(r.Context(), id)
	if err != nil || !authReq.ExpiresAt.After(s.now()) || authReq.Done() {
		s.render(w, struct{ AuthRequest, Error string }{AuthRequest: id, Error: "invalid"})
		return
	}
	user, err := s.identity.Verify(r.Context(), email, password)
	if errors.Is(err, identity.ErrInvalid) {
		s.render(w, struct{ AuthRequest, Error string }{AuthRequest: id, Error: "invalid"})
		return
	}
	if err != nil {
		s.logger.Error("registre: login verification failed", slog.Any("error", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if err := s.identity.Complete(r.Context(), id, user.ID, s.now()); err != nil {
		s.logger.Error("registre: failed to complete a login", slog.Any("error", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/authorize/callback?id="+id, http.StatusSeeOther)
}

func (s *Service) render(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := form.Execute(w, data); err != nil {
		s.logger.Error("registre: failed to render the login page", slog.Any("error", err))
	}
}
