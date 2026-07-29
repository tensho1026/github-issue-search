package github

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	apiVersion       = "2022-11-28"
	maxAttempts      = 3
	maxResponseBytes = 2 << 20
)

type httpDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

type sleepFunc func(context.Context, time.Duration) error
type backoffFunc func(retry int) time.Duration

// Client adapts GitHub REST payloads to IssueScout application ports.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient httpDoer
	logger     *slog.Logger
	sleep      sleepFunc
	backoff    backoffFunc
}

func NewClient(
	baseURL *url.URL,
	token string,
	timeout time.Duration,
	logger *slog.Logger,
) *Client {
	baseURLCopy := *baseURL
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL: &baseURLCopy,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:  logger,
		sleep:   sleepWithContext,
		backoff: exponentialBackoff,
	}
}

func (c *Client) GetUser(
	ctx context.Context,
	username user.Username,
) (port.GitHubUserResult, error) {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "users", url.PathEscape(username.String()))

	response, err := c.do(ctx, endpoint.String())
	if err != nil {
		return port.GitHubUserResult{}, err
	}
	defer response.Body.Close()

	rateLimit := parseRateLimit(response.Header)
	c.logger.Debug(
		"GitHub user response received",
		"status", response.StatusCode,
		"rateLimitKnown", rateLimit.Known,
		"rateLimitRemaining", rateLimit.Remaining,
		"rateLimitReset", rateLimit.Reset,
	)

	if err := responseError(response.StatusCode, rateLimit); err != nil {
		return port.GitHubUserResult{}, err
	}

	var payload userResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(&payload); err != nil {
		return port.GitHubUserResult{}, &port.GitHubError{
			Kind:  port.GitHubErrorUpstream,
			Cause: fmt.Errorf("decode GitHub user response: %w", err),
		}
	}

	login, err := user.ParseUsername(payload.Login)
	if err != nil {
		return port.GitHubUserResult{}, &port.GitHubError{
			Kind:  port.GitHubErrorUpstream,
			Cause: fmt.Errorf("validate GitHub response login: %w", err),
		}
	}

	return port.GitHubUserResult{
		Profile: user.Profile{
			Login:       login,
			Name:        stringValue(payload.Name),
			AvatarURL:   payload.AvatarURL,
			Bio:         stringValue(payload.Bio),
			PublicRepos: payload.PublicRepos,
			Followers:   payload.Followers,
			Following:   payload.Following,
		},
		RateLimit: rateLimit,
	}, nil
}

func (c *Client) do(ctx context.Context, endpoint string) (*http.Response, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, &port.GitHubError{
				Kind:  port.GitHubErrorUpstream,
				Cause: fmt.Errorf("create GitHub request: %w", err),
			}
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", apiVersion)
		request.Header.Set("User-Agent", "IssueScout")
		if c.token != "" {
			request.Header.Set("Authorization", "Bearer "+c.token)
		}

		response, requestErr := c.httpClient.Do(request)
		if requestErr == nil && !retryableStatus(response.StatusCode) {
			return response, nil
		}
		if requestErr != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt == maxAttempts-1 {
			if requestErr != nil {
				if response != nil {
					drainAndClose(response.Body)
				}
				return nil, &port.GitHubError{
					Kind:  port.GitHubErrorUpstream,
					Cause: fmt.Errorf("send GitHub request: %w", requestErr),
				}
			}
			return response, nil
		}

		if response != nil {
			drainAndClose(response.Body)
		}
		if err := c.sleep(ctx, c.backoff(attempt)); err != nil {
			return nil, err
		}
	}

	panic("unreachable GitHub retry loop")
}

func retryableStatus(status int) bool {
	return status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func responseError(status int, rateLimit port.RateLimit) error {
	switch {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		return nil
	case status == http.StatusNotFound:
		return &port.GitHubError{Kind: port.GitHubErrorNotFound}
	case status == http.StatusTooManyRequests ||
		(status == http.StatusForbidden &&
			rateLimit.Known &&
			rateLimit.Remaining == 0):
		return &port.GitHubError{
			Kind:  port.GitHubErrorRateLimited,
			Reset: rateLimit.Reset,
		}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &port.GitHubError{Kind: port.GitHubErrorUnauthorized}
	default:
		return &port.GitHubError{
			Kind:  port.GitHubErrorUpstream,
			Cause: fmt.Errorf("unexpected GitHub status: %d", status),
		}
	}
}

func parseRateLimit(header http.Header) port.RateLimit {
	limitRaw := header.Get("X-RateLimit-Limit")
	remainingRaw := header.Get("X-RateLimit-Remaining")
	resetRaw := header.Get("X-RateLimit-Reset")
	limit, _ := strconv.Atoi(limitRaw)
	remaining, _ := strconv.Atoi(remainingRaw)
	resetUnix, _ := strconv.ParseInt(resetRaw, 10, 64)

	var reset time.Time
	if resetUnix > 0 {
		reset = time.Unix(resetUnix, 0).UTC()
	}

	return port.RateLimit{
		Known:     limitRaw != "" || remainingRaw != "" || resetRaw != "",
		Limit:     limit,
		Remaining: remaining,
		Reset:     reset,
	}
}

func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
	_ = body.Close()
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func exponentialBackoff(retry int) time.Duration {
	base := 100 * time.Millisecond * time.Duration(1<<retry)
	var randomByte [1]byte
	if _, err := rand.Read(randomByte[:]); err != nil {
		return base
	}
	jitter := time.Duration(randomByte[0]) * base / 512
	return base + jitter
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type userResponse struct {
	Login       string  `json:"login"`
	Name        *string `json:"name"`
	AvatarURL   string  `json:"avatar_url"`
	Bio         *string `json:"bio"`
	PublicRepos int     `json:"public_repos"`
	Followers   int     `json:"followers"`
	Following   int     `json:"following"`
}

var _ port.GitHubUserReader = (*Client)(nil)
