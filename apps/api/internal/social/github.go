package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	githubAuthorize  = "https://github.com/login/oauth/authorize"
	githubToken      = "https://github.com/login/oauth/access_token"
	githubUser       = "https://api.github.com/user"
	githubEmails     = "https://api.github.com/user/emails"
	githubScopes     = "read:user user:email"
	githubUserAgent  = "Registre"
	githubAPIVersion = "2022-11-28"
)

// GitHub is the GitHub OAuth2 provider.
type GitHub struct {
	clientID     string
	clientSecret string
	client       *http.Client
	authorizeURL string
	tokenURL     string
	userURL      string
	emailsURL    string
}

type gitHubUser struct {
	ID        int64   `json:"id"`
	Login     string  `json:"login"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

type gitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// NewGitHub builds the provider for a GitHub OAuth application.
func NewGitHub(clientID, clientSecret string) *GitHub {
	return &GitHub{
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       &http.Client{Timeout: providerTimeout},
		authorizeURL: githubAuthorize,
		tokenURL:     githubToken,
		userURL:      githubUser,
		emailsURL:    githubEmails,
	}
}

// Name is the provider's key.
func (g *GitHub) Name() string { return "github" }

// AuthURL returns the GitHub authorization URL for state and PKCE challenge.
func (g *GitHub) AuthURL(state, challenge, redirectURI string) string {
	query := url.Values{
		"client_id":             {g.clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {githubScopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return g.authorizeURL + "?" + query.Encode()
}

// Exchange trades the code for a token and reads the user's profile and verified email.
func (g *GitHub) Exchange(ctx context.Context, code, verifier, redirectURI string) (Profile, error) {
	token, err := g.token(ctx, code, verifier, redirectURI)
	if err != nil {
		return Profile{}, err
	}
	return g.profile(ctx, token)
}

// token redeems the authorization code.
func (g *GitHub) token(ctx context.Context, code, verifier, redirectURI string) (string, error) {
	form := url.Values{
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("social: build the github token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", githubUserAgent)
	var granted struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := g.do(request, &granted); err != nil {
		return "", err
	}
	if granted.Error != "" {
		return "", fmt.Errorf("social: github returned error: %s", granted.Error)
	}
	if granted.AccessToken == "" || !strings.EqualFold(granted.TokenType, "bearer") {
		return "", errors.New("social: github returned no bearer token")
	}
	return granted.AccessToken, nil
}

// profile reads the person behind a token and their verified email.
func (g *GitHub) profile(ctx context.Context, token string) (Profile, error) {
	var u gitHubUser
	if err := g.get(ctx, g.userURL, token, &u); err != nil {
		return Profile{}, err
	}
	if u.ID <= 0 {
		return Profile{}, errors.New("social: github returned no usable user id")
	}
	var list []gitHubEmail
	if err := g.get(ctx, g.emailsURL, token, &list); err != nil {
		return Profile{}, err
	}
	email, err := chooseEmail(list)
	if err != nil {
		return Profile{}, err
	}
	name := u.Login
	if u.Name != nil && strings.TrimSpace(*u.Name) != "" {
		name = strings.TrimSpace(*u.Name)
	}
	var avatar string
	if u.AvatarURL != nil {
		avatar = *u.AvatarURL
	}
	return Profile{
		Subject:       strconv.FormatInt(u.ID, 10),
		Email:         email,
		EmailVerified: true,
		Name:          name,
		AvatarURL:     avatar,
	}, nil
}

// get executes an authenticated GET request against the GitHub API.
func (g *GitHub) get(ctx context.Context, endpoint, token string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("social: build the github request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("User-Agent", githubUserAgent)
	return g.do(request, out)
}

// do sends the request and decodes a capped JSON answer into out.
func (g *GitHub) do(request *http.Request, out any) error {
	response, err := g.client.Do(request)
	if err != nil {
		return fmt.Errorf("social: github request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxProviderBody))
		return fmt.Errorf("social: github answered %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxProviderBody)).Decode(out); err != nil {
		return fmt.Errorf("social: decode the github answer: %w", err)
	}
	return nil
}
