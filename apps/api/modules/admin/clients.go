package admin

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/go-chi/chi/v5"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// timeFormat is RFC 3339 with a UTC offset, which is what every timestamp
// this API ships uses.
const timeFormat = time.RFC3339

// clientView is what the dashboard sees of a client row: everything but the
// secret hash, which is a credential even in its sealed form.
type clientView struct {
	ClientID   string   `json:"client_id"`
	Subject    string   `json:"subject"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	GrantTypes []string `json:"grant_types,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	Audiences  []string `json:"audiences,omitempty"`

	// RedirectURIs, VisibleRoles and BackchannelLogoutURI are shown because
	// they are the three settings whose absence breaks an app silently:
	// without the first it cannot finish a login, without the second its
	// authorization checks all deny, and without the third it never learns
	// a session was terminated. A dashboard that hides them is a dashboard
	// nobody can diagnose from.
	RedirectURIs         []string `json:"redirect_uris,omitempty"`
	VisibleRoles         []string `json:"visible_roles,omitempty"`
	BackchannelLogoutURI string   `json:"backchannel_logout_uri,omitempty"`

	Managed    bool    `json:"managed"`
	CreatedAt  string  `json:"created_at"`
	DisabledAt *string `json:"disabled_at,omitempty"`
}

func viewClient(row schemas.Client) clientView {
	view := clientView{
		ClientID: row.ClientID, Subject: row.Subject, Name: row.Name, Kind: row.Kind,
		GrantTypes: row.GrantTypes, Scopes: row.Scopes, Audiences: row.Audiences,
		RedirectURIs: row.RedirectURIs, VisibleRoles: row.VisibleRoles,
		BackchannelLogoutURI: row.BackchannelLogoutURI, Managed: row.Managed,
		CreatedAt: row.CreatedAt.UTC().Format(timeFormat),
	}
	if row.DisabledAt != nil {
		at := row.DisabledAt.UTC().Format(timeFormat)
		view.DisabledAt = &at
	}
	return view
}

func (s *Service) listClients(w http.ResponseWriter, r *http.Request) {
	rows, err := s.register.List(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	views := make([]clientView, 0, len(rows))
	for _, row := range rows {
		views = append(views, viewClient(row))
	}
	encode(w, views, http.StatusOK)
}

// createClient mints a service account or an application and answers with
// its secret exactly once. The response is the only copy; losing it means
// rotating.
func (s *Service) createClient(w http.ResponseWriter, r *http.Request) {
	var body newClient
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if reason, ok := body.invalid(); !ok {
		encode(w, map[string]string{"error": reason}, http.StatusBadRequest)
		return
	}
	row, secret, err := s.register.Create(r.Context(), body.declaration())
	if err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: admin minted a client",
		slog.String("client_id", row.ClientID), slog.String("kind", row.Kind))
	encode(w, map[string]any{"client": viewClient(row), "secret": secret}, http.StatusCreated)
}

func (s *Service) rotateSecret(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	_, secret, err := s.register.RotateSecret(r.Context(), clientID)
	if errors.Is(err, clients.ErrUnknownClient) {
		encode(w, map[string]string{"error": "unknown client"}, http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: admin rotated a client secret", slog.String("client_id", clientID))
	encode(w, map[string]string{"client_id": clientID, "secret": secret}, http.StatusOK)
}

// newClient is the body createClient accepts.
type newClient struct {
	ClientID             string   `json:"client_id"`
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	GrantTypes           []string `json:"grant_types"`
	Scopes               []string `json:"scopes"`
	Audiences            []string `json:"audiences"`
	RedirectURIs         []string `json:"redirect_uris"`
	VisibleRoles         []string `json:"visible_roles"`
	BackchannelLogoutURI string   `json:"backchannel_logout_uri"`
}

// invalid reports why the declaration cannot be registered, or that it can.
//
// The redirect rule is the one worth stating. An application with no redirect
// URI is registrable and completely unusable: /authorize refuses it with "the
// requested redirect_uri is missing in the client configuration", which reads
// as registre being broken rather than as the row being half-filled. A service
// account with one is the opposite mistake, a browser flow declared for a
// principal that never opens a browser, and refusing it here keeps the exact
// match list to rows that actually redirect.
func (c newClient) invalid() (string, bool) {
	if c.ClientID == "" || c.Name == "" {
		return "client_id and name are required", false
	}
	switch c.Kind {
	case schemas.KindApplication:
		if len(c.RedirectURIs) == 0 {
			return "an application needs at least one redirect_uri, or it can never finish a login", false
		}
	case schemas.KindServiceAccount:
		if len(c.RedirectURIs) > 0 {
			return "a service account has no browser flow, so it takes no redirect_uri", false
		}
	default:
		return "kind must be service_account or application", false
	}
	if bad, ok := firstBadURI(append(c.RedirectURIs, c.BackchannelLogoutURI)); !ok {
		return "not an absolute http or https URL: " + bad, false
	}
	return c.grantsFit()
}

// grantsFit reports whether the declared grants are ones this kind can use.
//
// Defaulting the grants was not enough on its own. A caller naming
// client_credentials on an application registers a row the seed could never
// produce, and it fails at /authorize with the same unnameable "missing
// response type" that defaulting was added to prevent. The register is the
// ceiling on what a client may ask for, so a grant its kind cannot perform
// belongs nowhere in it.
func (c newClient) grantsFit() (string, bool) {
	allowed := map[string]bool{}
	for _, grant := range defaultGrants(c.Kind) {
		allowed[grant] = true
	}
	for _, grant := range c.GrantTypes {
		if !allowed[grant] {
			return "a " + c.Kind + " cannot use the " + grant + " grant", false
		}
	}
	return "", true
}

// firstBadURI returns the first entry that is not an absolute http or https
// URL, ignoring empty ones.
//
// A malformed redirect URI is refused at /authorize, which is at least loud. A
// malformed backchannel_logout_uri is worse: SPEC §6 lists back-channel logout
// as one of the two things that break silently, so a typo there means
// deactivation quietly stops propagating and nothing anywhere errors.
func firstBadURI(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" ||
			(parsed.Scheme != "https" && parsed.Scheme != "http") {
			return candidate, false
		}
	}
	return "", true
}

// declaration turns the body into the row the register stores, filling the
// grants and scopes the kind implies when the caller names none.
//
// The defaults are the seed's, deliberately: internal/seed already decides
// what an application and a service account are allowed to do, and a
// dashboard that decided differently would mean the same app worked or did
// not depending on which of the two registered it. A client with no grants at
// all is the failure this prevents, and it surfaces as /authorize answering
// "the requested response type is missing in the client configuration", which
// names neither the row nor the field.
func (c newClient) declaration() schemas.Client {
	grants, scopes := c.GrantTypes, c.Scopes
	if len(grants) == 0 {
		grants = defaultGrants(c.Kind)
	}
	if len(scopes) == 0 && c.Kind == schemas.KindApplication {
		scopes = []string{"openid", "profile", "email"}
	}
	return schemas.Client{
		ClientID: c.ClientID, Name: c.Name, Kind: c.Kind,
		GrantTypes:   schemas.StringList(grants),
		Scopes:       schemas.StringList(scopes),
		Audiences:    schemas.StringList(c.Audiences),
		RedirectURIs: schemas.StringList(c.RedirectURIs),
		VisibleRoles: schemas.StringList(c.VisibleRoles),

		BackchannelLogoutURI: c.BackchannelLogoutURI,
	}
}

// defaultGrants is what a kind may do when the declaration says nothing.
func defaultGrants(kind string) []string {
	if kind == schemas.KindApplication {
		return []string{string(zoidc.GrantTypeCode), string(zoidc.GrantTypeRefreshToken)}
	}
	return []string{string(zoidc.GrantTypeClientCredentials)}
}

func badRequest(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "bad body"}, http.StatusBadRequest)
}

func serverError(w http.ResponseWriter, err error) {
	slog.Error("registre: admin request failed", slog.Any("error", err))
	encode(w, map[string]string{"error": "internal"}, http.StatusInternalServerError)
}
