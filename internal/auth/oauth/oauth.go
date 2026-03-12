// Package oauth provides GitHub and GitLab OAuth authentication providers
// for the NexusOps platform.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthToken represents the token response from an OAuth provider.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// OAuthUser represents a user profile returned by an OAuth provider.
type OAuthUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
}

// Config holds the OAuth application credentials and settings.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// OAuthProvider defines the interface for OAuth authentication flows.
type OAuthProvider interface {
	// AuthURL returns the URL to redirect the user for authorization.
	AuthURL(state string) string
	// Exchange trades an authorization code for an access token.
	Exchange(ctx context.Context, code string) (*OAuthToken, error)
	// GetUser fetches the authenticated user's profile using an access token.
	GetUser(ctx context.Context, token string) (*OAuthUser, error)
}

// GitHubProvider implements OAuthProvider for GitHub OAuth.
type GitHubProvider struct {
	config     Config
	httpClient *http.Client
	authURL    string
	tokenURL   string
	apiURL     string
}

// NewGitHubProvider creates a new GitHub OAuth provider with the given configuration.
func NewGitHubProvider(config Config) *GitHubProvider {
	return &GitHubProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		authURL:  "https://github.com/login/oauth/authorize",
		tokenURL: "https://github.com/login/oauth/access_token",
		apiURL:   "https://api.github.com",
	}
}

// AuthURL returns the GitHub authorization URL for the given state parameter.
func (g *GitHubProvider) AuthURL(state string) string {
	params := url.Values{
		"client_id":    {g.config.ClientID},
		"redirect_uri": {g.config.RedirectURL},
		"scope":        {strings.Join(g.config.Scopes, " ")},
		"state":        {state},
	}
	return fmt.Sprintf("%s?%s", g.authURL, params.Encode())
}

// Exchange trades an authorization code for an access token with GitHub.
func (g *GitHubProvider) Exchange(ctx context.Context, code string) (*OAuthToken, error) {
	data := url.Values{
		"client_id":     {g.config.ClientID},
		"client_secret": {g.config.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.config.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth/github: failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth/github: token exchange returned status %d: %s", resp.StatusCode, string(body))
	}

	var token OAuthToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("oauth/github: failed to decode token response: %w", err)
	}

	if token.AccessToken == "" {
		// GitHub may return a 200 with an error payload instead of a token.
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error != "" {
			return nil, fmt.Errorf("oauth/github: authorization failed: %s — %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("oauth/github: received empty access token")
	}

	return &token, nil
}

// GetUser fetches the authenticated user's profile from the GitHub API.
func (g *GitHubProvider) GetUser(ctx context.Context, token string) (*OAuthUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiURL+"/user", nil)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: failed to create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth/github: failed to read user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth/github: user endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var user OAuthUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("oauth/github: failed to decode user response: %w", err)
	}
	user.Provider = "github"

	// If email is not public, attempt to fetch from the emails endpoint.
	if user.Email == "" {
		email, emailErr := g.fetchPrimaryEmail(ctx, token)
		if emailErr == nil {
			user.Email = email
		}
	}

	return &user, nil
}

// fetchPrimaryEmail retrieves the user's primary verified email from the GitHub emails API.
func (g *GitHubProvider) fetchPrimaryEmail(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiURL+"/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("oauth/github: failed to create emails request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth/github: emails request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth/github: emails endpoint returned status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("oauth/github: failed to decode emails response: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	// Fall back to the first verified email.
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("oauth/github: no verified email found")
}

// GitLabProvider implements OAuthProvider for GitLab OAuth.
type GitLabProvider struct {
	config     Config
	httpClient *http.Client
	baseURL    string
}

// NewGitLabProvider creates a new GitLab OAuth provider with the given configuration.
// It defaults to gitlab.com; for self-hosted instances, set the BaseURL on the returned provider.
func NewGitLabProvider(config Config) *GitLabProvider {
	return &GitLabProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		baseURL: "https://gitlab.com",
	}
}

// SetBaseURL overrides the GitLab base URL for self-hosted instances.
func (gl *GitLabProvider) SetBaseURL(baseURL string) {
	gl.baseURL = strings.TrimRight(baseURL, "/")
}

// AuthURL returns the GitLab authorization URL for the given state parameter.
func (gl *GitLabProvider) AuthURL(state string) string {
	params := url.Values{
		"client_id":     {gl.config.ClientID},
		"redirect_uri":  {gl.config.RedirectURL},
		"response_type": {"code"},
		"scope":         {strings.Join(gl.config.Scopes, " ")},
		"state":         {state},
	}
	return fmt.Sprintf("%s/oauth/authorize?%s", gl.baseURL, params.Encode())
}

// Exchange trades an authorization code for an access token with GitLab.
func (gl *GitLabProvider) Exchange(ctx context.Context, code string) (*OAuthToken, error) {
	data := url.Values{
		"client_id":     {gl.config.ClientID},
		"client_secret": {gl.config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {gl.config.RedirectURL},
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", gl.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth/gitlab: failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := gl.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth/gitlab: token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth/gitlab: failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth/gitlab: token exchange returned status %d: %s", resp.StatusCode, string(body))
	}

	var token OAuthToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("oauth/gitlab: failed to decode token response: %w", err)
	}

	if token.AccessToken == "" {
		return nil, fmt.Errorf("oauth/gitlab: received empty access token")
	}

	return &token, nil
}

// GetUser fetches the authenticated user's profile from the GitLab API.
func (gl *GitLabProvider) GetUser(ctx context.Context, token string) (*OAuthUser, error) {
	userURL := fmt.Sprintf("%s/api/v4/user", gl.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth/gitlab: failed to create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := gl.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth/gitlab: user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth/gitlab: failed to read user response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth/gitlab: user endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// GitLab returns a slightly different schema than GitHub.
	var glUser struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(body, &glUser); err != nil {
		return nil, fmt.Errorf("oauth/gitlab: failed to decode user response: %w", err)
	}

	if glUser.State != "active" {
		return nil, fmt.Errorf("oauth/gitlab: user account is not active (state: %s)", glUser.State)
	}

	return &OAuthUser{
		ID:        glUser.ID,
		Login:     glUser.Username,
		Name:      glUser.Name,
		Email:     glUser.Email,
		AvatarURL: glUser.AvatarURL,
		Provider:  "gitlab",
	}, nil
}
