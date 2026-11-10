package connect

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/metadoc"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// errGone is every flow this session may not answer: unknown, expired, not
// finished, already turned into a code, or signed in to by another session.
// They answer alike so an auth request id tells its holder nothing.
var errGone = errors.New("connect: no such pending connection")

// pendingView is what the consent screen shows.
//
// client_name comes from the client's own metadata and is untrusted, so the
// screen leads with redirect_host, which registre checked. loopback_only is
// the MCP authorization spec's warning case: every redirect is on this
// machine, where any program can listen.
type pendingView struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	ClientURI    string   `json:"client_uri,omitempty"`
	Registration string   `json:"registration"`
	RedirectHost string   `json:"redirect_host"`
	LoopbackOnly bool     `json:"loopback_only"`
	Resource     string   `json:"resource"`
	AppName      string   `json:"app_name"`
	Scopes       []string `json:"scopes"`
}

// pending answers GET /api/connect?auth_request=ID.
func (s *Service) pending(w http.ResponseWriter, r *http.Request) {
	row, client, err := s.flow(r.Context(), r.URL.Query().Get("auth_request"))
	if err != nil {
		s.refuse(w, err)
		return
	}
	view := pendingView{
		ClientID: client.ClientID, ClientName: client.Name, ClientURI: client.ClientURI,
		Registration: client.Registration, Resource: row.Resource, Scopes: row.Scopes,
		LoopbackOnly: loopbackOnly(client.RedirectURIs),
	}
	if target, err := url.Parse(row.RedirectURI); err == nil {
		view.RedirectHost = target.Host
		if view.RedirectHost == "" {
			view.RedirectHost = target.Scheme + ":"
		}
	}
	if owner, err := s.register.ByResource(r.Context(), row.Resource); err == nil {
		view.AppName = owner.Name
	}
	encode(w, view, http.StatusOK)
}

// answerBody is the person's decision.
type answerBody struct {
	AuthRequest string `json:"auth_request"`
	Allow       bool   `json:"allow"`
}

// answer answers POST /api/connect. Allowing records the grant and sends the
// browser back to the callback, which now issues the code. Refusing ends the
// flow and sends the browser to the client with access_denied, as RFC 6749
// §4.1.2.1 asks, carrying state and RFC 9207 iss.
func (s *Service) answer(w http.ResponseWriter, r *http.Request) {
	var body answerBody
	if err := decode(r, &body); err != nil {
		encode(w, map[string]string{"error": "invalid_request"}, http.StatusBadRequest)
		return
	}
	row, client, err := s.flow(r.Context(), body.AuthRequest)
	if err != nil {
		s.refuse(w, err)
		return
	}
	if !body.Allow {
		s.deny(w, r, row)
		return
	}
	if err := s.grants.Record(r.Context(), *row.UserID, client.ClientID, row.Resource, row.Scopes); err != nil {
		s.refuse(w, err)
		return
	}
	s.logger.Info("registre: a person connected a third-party client",
		"client_id", client.ClientID, "resource", row.Resource, "scopes", row.Scopes)
	encode(w, map[string]string{"redirect": "/authorize/callback?id=" + url.QueryEscape(row.ID)}, http.StatusOK)
}

// deny ends the flow and answers the client's access_denied redirect.
func (s *Service) deny(w http.ResponseWriter, r *http.Request, row schemas.AuthRequest) {
	if err := s.db.WithContext(r.Context()).Delete(&schemas.AuthRequest{}, "id = ?", row.ID).Error; err != nil {
		s.refuse(w, err)
		return
	}
	target, err := url.Parse(row.RedirectURI)
	if err != nil {
		s.refuse(w, err)
		return
	}
	query := target.Query()
	query.Set("error", "access_denied")
	query.Set("iss", s.issuer)
	if row.State != "" {
		query.Set("state", row.State)
	}
	target.RawQuery = query.Encode()
	encode(w, map[string]string{"redirect": target.String()}, http.StatusOK)
}

// flow reads the pending third-party flow this session signed in to.
func (s *Service) flow(ctx context.Context, id string) (schemas.AuthRequest, schemas.Client, error) {
	_, session := accounts.Holder(ctx)
	var row schemas.AuthRequest
	if id == "" || s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error != nil {
		return schemas.AuthRequest{}, schemas.Client{}, errGone
	}
	if !row.Done() || row.Code != "" || row.SessionID != session.ID || !row.ExpiresAt.After(s.now()) {
		return schemas.AuthRequest{}, schemas.Client{}, errGone
	}
	client, err := s.register.ByClientID(ctx, row.ClientID)
	if err != nil || !client.ThirdParty() {
		return schemas.AuthRequest{}, schemas.Client{}, errGone
	}
	return row, client, nil
}

// loopbackOnly reports whether every redirect URI is on the person's own
// machine.
func loopbackOnly(uris []string) bool {
	for _, raw := range uris {
		parsed, err := url.Parse(raw)
		if err != nil || !metadoc.IsLoopback(parsed.Hostname()) {
			return false
		}
	}
	return len(uris) > 0
}
