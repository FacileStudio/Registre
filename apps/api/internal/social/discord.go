package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	discordAuthorize = "https://discord.com/oauth2/authorize"
	discordToken     = "https://discord.com/api/oauth2/token"
	discordUserinfo  = "https://discord.com/api/users/@me"
	discordScopes    = "identify email"
	discordCDN       = "https://cdn.discordapp.com"

	maxProviderBody = 1 << 20
	providerTimeout = 10 * time.Second
)

// Discord is the Discord OAuth2 provider: authorization code with PKCE, then
// the identify and email scopes.
type Discord struct {
	clientID     string
	clientSecret string
	client       *http.Client
	authorizeURL string
	tokenURL     string
	userinfoURL  string
}

// NewDiscord builds the provider for a Discord application.
func NewDiscord(clientID, clientSecret string) *Discord {
	return &Discord{
		clientID: clientID, clientSecret: clientSecret,
		client:       &http.Client{Timeout: providerTimeout},
		authorizeURL: discordAuthorize, tokenURL: discordToken, userinfoURL: discordUserinfo,
	}
}

// Name is the provider's key.
func (d *Discord) Name() string { return "discord" }

// AuthURL is Discord's consent page for this state and challenge.
func (d *Discord) AuthURL(state, challenge, redirectURI string) string {
	query := url.Values{
		"client_id":             {d.clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {discordScopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"none"},
	}
	return d.authorizeURL + "?" + query.Encode()
}

// Exchange trades the code for a token and reads the person's profile with
// it. Every response is size capped and the client has a timeout.
func (d *Discord) Exchange(ctx context.Context, code, verifier, redirectURI string) (Profile, error) {
	token, err := d.token(ctx, code, verifier, redirectURI)
	if err != nil {
		return Profile{}, err
	}
	return d.profile(ctx, token)
}

// token redeems the authorization code.
func (d *Discord) token(ctx context.Context, code, verifier, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {d.clientID},
		"client_secret": {d.clientSecret},
		"code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("social: build the discord token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var granted struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := d.do(request, &granted); err != nil {
		return "", err
	}
	if granted.AccessToken == "" || !strings.EqualFold(granted.TokenType, "bearer") {
		return "", errors.New("social: discord returned no bearer token")
	}
	return granted.AccessToken, nil
}

// profile reads the person behind a token.
func (d *Discord) profile(ctx context.Context, token string) (Profile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.userinfoURL, nil)
	if err != nil {
		return Profile{}, fmt.Errorf("social: build the discord profile request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	var user struct {
		ID         string  `json:"id"`
		Username   string  `json:"username"`
		GlobalName *string `json:"global_name"`
		Avatar     *string `json:"avatar"`
		Email      *string `json:"email"`
		Verified   bool    `json:"verified"`
	}
	if err := d.do(request, &user); err != nil {
		return Profile{}, err
	}
	if !snowflake(user.ID) {
		return Profile{}, errors.New("social: discord returned no usable user id")
	}
	out := Profile{Subject: user.ID, Name: user.Username, EmailVerified: user.Verified}
	if user.GlobalName != nil && strings.TrimSpace(*user.GlobalName) != "" {
		out.Name = strings.TrimSpace(*user.GlobalName)
	}
	if user.Email != nil {
		out.Email = strings.ToLower(strings.TrimSpace(*user.Email))
	}
	if user.Avatar != nil && *user.Avatar != "" {
		out.AvatarURL = fmt.Sprintf("%s/avatars/%s/%s.png", discordCDN, user.ID, url.PathEscape(*user.Avatar))
	}
	return out, nil
}

// do sends the request and decodes a capped JSON answer into out.
func (d *Discord) do(request *http.Request, out any) error {
	response, err := d.client.Do(request)
	if err != nil {
		return fmt.Errorf("social: discord request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderBody))
		return fmt.Errorf("social: discord answered %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxProviderBody)).Decode(out); err != nil {
		return fmt.Errorf("social: decode the discord answer: %w", err)
	}
	return nil
}

// snowflake reports whether id looks like a Discord user id: 1 to 20 digits.
func snowflake(id string) bool {
	if id == "" || len(id) > 20 {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
