// Package githuboauth adapts GitHub's OAuth web application flow to the
// authentication port without retaining upstream access tokens.
package githuboauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	apiVersion           = "2022-11-28"
	oauthScope           = "read:user"
	maximumResponseBytes = 64 << 10
	issueScoutUserAgent  = "IssueScout-OAuth"
)

var (
	// ErrInvalidConfiguration reports an incomplete adapter configuration
	// without including credentials or endpoint values.
	ErrInvalidConfiguration = errors.New("invalid GitHub OAuth client configuration")
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client performs a one-time PKCE exchange and fetches only GitHub's public
// authenticated-user identity.
type Client struct {
	authorizeURL *url.URL
	tokenURL     *url.URL
	apiBaseURL   *url.URL
	callbackURL  *url.URL
	clientID     string
	clientSecret string
	httpClient   httpDoer
}

var _ port.GitHubOAuth = (*Client)(nil)

// NewClient creates a bounded GitHub OAuth adapter.
func NewClient(
	authorizeURL *url.URL,
	tokenURL *url.URL,
	apiBaseURL *url.URL,
	callbackURL *url.URL,
	clientID string,
	clientSecret string,
	timeout time.Duration,
) (*Client, error) {
	if authorizeURL == nil || tokenURL == nil || apiBaseURL == nil ||
		callbackURL == nil || clientID == "" || clientSecret == "" ||
		timeout <= 0 {
		return nil, ErrInvalidConfiguration
	}
	authorizeCopy := *authorizeURL
	tokenCopy := *tokenURL
	apiCopy := *apiBaseURL
	callbackCopy := *callbackURL
	return &Client{
		authorizeURL: &authorizeCopy,
		tokenURL:     &tokenCopy,
		apiBaseURL:   &apiCopy,
		callbackURL:  &callbackCopy,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: timeout},
	}, nil
}

// AuthorizationURL returns GitHub's exact authorization endpoint with an
// explicit minimal scope, state, redirect URI, and S256 PKCE challenge.
func (client *Client) AuthorizationURL(
	state auth.Secret,
	codeChallenge string,
) string {
	endpoint := *client.authorizeURL
	query := endpoint.Query()
	query.Set("client_id", client.clientID)
	query.Set("redirect_uri", client.callbackURL.String())
	query.Set("scope", oauthScope)
	query.Set("state", state.Value())
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	endpoint.RawQuery = query.Encode()
	return endpoint.String()
}

// Exchange sends a one-time code and PKCE verifier directly to GitHub. The
// returned access token remains in memory only.
func (client *Client) Exchange(
	ctx context.Context,
	code auth.Secret,
	codeVerifier auth.Secret,
) (auth.Secret, error) {
	if !code.IsSet() || !codeVerifier.IsSet() {
		return auth.Secret{}, auth.ErrOAuthRejected
	}
	form := url.Values{
		"client_id":     {client.clientID},
		"client_secret": {client.clientSecret},
		"code":          {code.Value()},
		"redirect_uri":  {client.callbackURL.String()},
		"code_verifier": {codeVerifier.Value()},
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		client.tokenURL.String(),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return auth.Secret{}, auth.ErrOAuthUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", issueScoutUserAgent)
	response, err := client.httpClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return auth.Secret{}, ctx.Err()
		}
		return auth.Secret{}, auth.ErrOAuthUnavailable
	}
	defer response.Body.Close()
	var payload tokenResponse
	decoder := json.NewDecoder(io.LimitReader(
		response.Body,
		maximumResponseBytes,
	))
	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		return auth.Secret{}, auth.ErrOAuthUnavailable
	}
	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices ||
		payload.Error != "" ||
		payload.AccessToken == "" ||
		!strings.EqualFold(payload.TokenType, "bearer") {
		return auth.Secret{}, auth.ErrOAuthRejected
	}
	if !minimalScope(payload.Scope) {
		return auth.Secret{}, auth.ErrOAuthRejected
	}
	return auth.NewSecret(payload.AccessToken), nil
}

// FetchIdentity calls only GET /user, validates the public identity, and
// returns no upstream token data.
func (client *Client) FetchIdentity(
	ctx context.Context,
	accessToken auth.Secret,
) (auth.GitHubIdentity, error) {
	if !accessToken.IsSet() {
		return auth.GitHubIdentity{}, auth.ErrOAuthRejected
	}
	endpoint := *client.apiBaseURL
	endpoint.Path = path.Join(endpoint.Path, "user")
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint.String(),
		nil,
	)
	if err != nil {
		return auth.GitHubIdentity{}, auth.ErrOAuthUnavailable
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken.Value())
	request.Header.Set("User-Agent", issueScoutUserAgent)
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	response, err := client.httpClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return auth.GitHubIdentity{}, ctx.Err()
		}
		return auth.GitHubIdentity{}, auth.ErrOAuthUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized ||
		response.StatusCode == http.StatusForbidden {
		return auth.GitHubIdentity{}, auth.ErrOAuthRejected
	}
	if response.StatusCode != http.StatusOK {
		return auth.GitHubIdentity{}, auth.ErrOAuthUnavailable
	}
	var payload userResponse
	decoder := json.NewDecoder(io.LimitReader(
		response.Body,
		maximumResponseBytes,
	))
	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		return auth.GitHubIdentity{}, auth.ErrOAuthUnavailable
	}
	login, err := user.ParseUsername(payload.Login)
	if err != nil {
		return auth.GitHubIdentity{}, auth.ErrInvalidIdentity
	}
	identity := auth.GitHubIdentity{
		UserID:     payload.ID,
		Login:      login,
		AvatarURL:  payload.AvatarURL,
		ProfileURL: payload.ProfileURL,
	}
	if err := identity.Validate(); err != nil {
		return auth.GitHubIdentity{}, auth.ErrInvalidIdentity
	}
	return identity, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type userResponse struct {
	ID         int64  `json:"id"`
	Login      string `json:"login"`
	AvatarURL  string `json:"avatar_url"`
	ProfileURL string `json:"html_url"`
}

func minimalScope(raw string) bool {
	if raw == "" {
		return true
	}
	for _, scope := range strings.Split(raw, ",") {
		if strings.TrimSpace(scope) != oauthScope {
			return false
		}
	}
	return true
}
