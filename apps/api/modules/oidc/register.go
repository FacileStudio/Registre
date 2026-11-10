package oidc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/metadoc"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// RegistrationPath is the RFC 7591 dynamic client registration endpoint.
const RegistrationPath = "/register"

// DefaultRegisterLimit and DefaultRegisterBurst bound how fast one address
// may register clients: a burst of twenty, then one a minute. Registration is
// unauthenticated and every call writes a row; MCP clients that support CIMD
// never call it, and the rest call it once per connection.
const (
	DefaultRegisterLimit = 1.0 / 60
	DefaultRegisterBurst = 20
)

// registered is the RFC 7591 §3.2.1 answer. There is no secret: every
// dynamically registered client is public and proves itself with PKCE.
type registered struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ApplicationType         string   `json:"application_type"`
}

// register answers POST /register.
//
// DCR is kept for clients that do not speak CIMD yet (Cursor, older MCP
// SDKs). A client that asks for a secret-based auth method is refused rather
// than silently made public, so it learns at registration and not at its
// first token request. A missing application_type is native, which is what
// Cursor's registration needs.
func (s *Storage) register(w http.ResponseWriter, r *http.Request) {
	var asked metadoc.Metadata
	if err := json.NewDecoder(io.LimitReader(r.Body, metadoc.MaxDocument)).Decode(&asked); err != nil {
		registrationError(w, "invalid_client_metadata", "the body is not JSON client metadata")
		return
	}
	asked.ClientID = ""
	meta, err := metadoc.Validate(asked)
	if err != nil {
		registrationError(w, registrationCode(err), reason(err))
		return
	}
	row, err := s.saveDynamic(r, meta)
	if err != nil {
		registrationError(w, "server_error", "the client could not be registered")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(registered{
		ClientID: row.ClientID, ClientIDIssuedAt: row.CreatedAt.Unix(), ClientName: row.Name,
		ClientURI: row.ClientURI, RedirectURIs: row.RedirectURIs, GrantTypes: meta.GrantTypes,
		ResponseTypes: meta.ResponseTypes, TokenEndpointAuthMethod: meta.TokenEndpointAuthMethod,
		ApplicationType: meta.ApplicationType,
	})
}

// saveDynamic stores a dynamically registered client under a fresh id.
func (s *Storage) saveDynamic(r *http.Request, meta metadoc.Metadata) (schemas.Client, error) {
	id, err := ids.New()
	if err != nil {
		return schemas.Client{}, err
	}
	row := clients.ThirdPartyRow("dcr-"+id, schemas.RegistrationDynamic, meta, nil)
	saved, err := s.svc.Register.SaveThirdParty(r.Context(), row)
	if err == nil {
		s.logger.Info("registre: registered a dynamic client", "client_id", saved.ClientID,
			"redirect_uris", saved.RedirectURIs, "at", time.Now().UTC())
	}
	return saved, err
}

// registrationCode is invalid_redirect_uri for a refused redirect URI and
// invalid_client_metadata for anything else (RFC 7591 §3.2.2).
func registrationCode(err error) string {
	if errors.Is(err, metadoc.ErrRedirect) {
		return "invalid_redirect_uri"
	}
	return "invalid_client_metadata"
}

// reason is the last line of a joined metadata error: the part that says
// what to fix.
func reason(err error) string {
	message := err.Error()
	return message[strings.LastIndex(message, "\n")+1:]
}

// registrationError writes an RFC 7591 §3.2.2 error.
func registrationError(w http.ResponseWriter, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusBadRequest
	if code == "server_error" {
		status = http.StatusInternalServerError
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}
