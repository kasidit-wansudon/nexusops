package oauth

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGitHubConfig() Config {
	return Config{
		ClientID:     "gh-client-id-123",
		ClientSecret: "gh-client-secret-456",
		RedirectURL:  "https://nexusops.example.com/auth/github/callback",
		Scopes:       []string{"user:email", "read:org"},
	}
}

func testGitLabConfig() Config {
	return Config{
		ClientID:     "gl-client-id-789",
		ClientSecret: "gl-client-secret-abc",
		RedirectURL:  "https://nexusops.example.com/auth/gitlab/callback",
		Scopes:       []string{"read_user", "openid"},
	}
}

func TestGitHubProviderAuthURL(t *testing.T) {
	cfg := testGitHubConfig()
	provider := NewGitHubProvider(cfg)

	state := "random-state-token"
	authURL := provider.AuthURL(state)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "github.com", parsed.Host)
	assert.Equal(t, "/login/oauth/authorize", parsed.Path)

	params := parsed.Query()
	assert.Equal(t, cfg.ClientID, params.Get("client_id"))
	assert.Equal(t, cfg.RedirectURL, params.Get("redirect_uri"))
	assert.Equal(t, state, params.Get("state"))
	assert.Equal(t, "user:email read:org", params.Get("scope"))
}

func TestGitHubProviderAuthURLEmptyState(t *testing.T) {
	cfg := testGitHubConfig()
	provider := NewGitHubProvider(cfg)

	authURL := provider.AuthURL("")

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	params := parsed.Query()
	assert.Equal(t, "", params.Get("state"))
	assert.Equal(t, cfg.ClientID, params.Get("client_id"))
}

func TestGitLabProviderAuthURL(t *testing.T) {
	cfg := testGitLabConfig()
	provider := NewGitLabProvider(cfg)

	state := "gl-state-xyz"
	authURL := provider.AuthURL(state)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "gitlab.com", parsed.Host)
	assert.Equal(t, "/oauth/authorize", parsed.Path)

	params := parsed.Query()
	assert.Equal(t, cfg.ClientID, params.Get("client_id"))
	assert.Equal(t, cfg.RedirectURL, params.Get("redirect_uri"))
	assert.Equal(t, state, params.Get("state"))
	assert.Equal(t, "code", params.Get("response_type"))
	assert.Equal(t, "read_user openid", params.Get("scope"))
}

func TestGitLabProviderAuthURLCustomBaseURL(t *testing.T) {
	cfg := testGitLabConfig()
	provider := NewGitLabProvider(cfg)
	provider.SetBaseURL("https://gitlab.mycompany.com/")

	authURL := provider.AuthURL("state-123")

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	assert.Equal(t, "gitlab.mycompany.com", parsed.Host)
	assert.Equal(t, "/oauth/authorize", parsed.Path)
}

func TestGitHubProviderConfig(t *testing.T) {
	cfg := testGitHubConfig()
	provider := NewGitHubProvider(cfg)

	assert.Equal(t, "gh-client-id-123", provider.config.ClientID)
	assert.Equal(t, "gh-client-secret-456", provider.config.ClientSecret)
	assert.Equal(t, "https://nexusops.example.com/auth/github/callback", provider.config.RedirectURL)
	assert.Equal(t, []string{"user:email", "read:org"}, provider.config.Scopes)
	assert.NotNil(t, provider.httpClient)
	assert.Equal(t, "https://github.com/login/oauth/authorize", provider.authURL)
	assert.Equal(t, "https://github.com/login/oauth/access_token", provider.tokenURL)
	assert.Equal(t, "https://api.github.com", provider.apiURL)
}

func TestGitLabProviderConfig(t *testing.T) {
	cfg := testGitLabConfig()
	provider := NewGitLabProvider(cfg)

	assert.Equal(t, "gl-client-id-789", provider.config.ClientID)
	assert.Equal(t, "gl-client-secret-abc", provider.config.ClientSecret)
	assert.Equal(t, "https://nexusops.example.com/auth/gitlab/callback", provider.config.RedirectURL)
	assert.Equal(t, []string{"read_user", "openid"}, provider.config.Scopes)
	assert.NotNil(t, provider.httpClient)
	assert.Equal(t, "https://gitlab.com", provider.baseURL)
}

func TestGitHubProviderImplementsInterface(t *testing.T) {
	cfg := testGitHubConfig()
	var _ OAuthProvider = NewGitHubProvider(cfg)
}

func TestGitLabProviderImplementsInterface(t *testing.T) {
	cfg := testGitLabConfig()
	var _ OAuthProvider = NewGitLabProvider(cfg)
}

func TestGitLabSetBaseURLTrimsTrailingSlash(t *testing.T) {
	cfg := testGitLabConfig()
	provider := NewGitLabProvider(cfg)

	provider.SetBaseURL("https://gitlab.self-hosted.io///")
	assert.Equal(t, "https://gitlab.self-hosted.io", provider.baseURL)
}
