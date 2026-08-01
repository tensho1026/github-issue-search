package github

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	apiVersion       = "2022-11-28"
	maxAttempts      = 3
	maxResponseBytes = 2 << 20
	maxManifestBytes = 512 << 10

	operationGetUser              = "user.get"
	operationListRepositories     = "repository.list"
	operationSearchIssues         = "issue.search"
	operationGetIssueDetail       = "issue.detail"
	operationAnalyzeProfile       = "profile.analyze"
	operationSearchRepositories   = "repository.search"
	operationEnrichRepositories   = "repository.enrich"
	upstreamServiceGitHub         = "github"
	upstreamOutcomeSuccess        = "success"
	upstreamOutcomeNotFound       = "not_found"
	upstreamOutcomeRateLimited    = "rate_limited"
	upstreamOutcomeUnauthorized   = "unauthorized"
	upstreamOutcomeCancelled      = "cancelled"
	upstreamOutcomeDeadline       = "deadline_exceeded"
	upstreamOutcomeTransportError = "transport_error"
	upstreamOutcomeResponseError  = "response_error"
)

type httpDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

type sleepFunc func(context.Context, time.Duration) error
type backoffFunc func(retry int) time.Duration
type requestFactory func() (*http.Request, error)

// Client adapts bounded GitHub REST and GraphQL payloads to IssueScout ports.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient httpDoer
	logger     *slog.Logger
	sleep      sleepFunc
	backoff    backoffFunc
	now        func() time.Time
}

// NewClient constructs a GitHub adapter with a copied non-nil base URL,
// process-memory token, per-request timeout, bounded retries, and redacted
// structured logging. A nil logger uses slog.Default.
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
		now:     time.Now,
	}
}

// GetUser fetches one normalized public profile. It propagates cancellation,
// bounds the response body, retries only transient failures, and returns a
// classified port.GitHubError for upstream failures.
func (c *Client) GetUser(
	ctx context.Context,
	username user.Username,
) (port.GitHubUserResult, error) {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "users", url.PathEscape(username.String()))

	response, err := c.do(ctx, operationGetUser, endpoint.String())
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

	if statusErr := responseError(response.StatusCode, rateLimit); statusErr != nil {
		return port.GitHubUserResult{}, statusErr
	}

	var payload userResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		return port.GitHubUserResult{}, &port.GitHubError{
			Kind:  port.GitHubErrorUpstream,
			Cause: fmt.Errorf("decode GitHub user response: %w", decodeErr),
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

// ListRepositories retrieves at most limit owned public repositories using
// bounded GitHub pagination. A non-positive limit performs no I/O.
func (c *Client) ListRepositories(
	ctx context.Context,
	username user.Username,
	limit int,
) ([]repository.Summary, port.RateLimit, error) {
	if limit <= 0 {
		return []repository.Summary{}, port.RateLimit{}, nil
	}

	repositories := make([]repository.Summary, 0, limit)
	var rateLimit port.RateLimit
	pageSize := min(100, limit)

	for pageNumber := 1; len(repositories) < limit; pageNumber++ {
		endpoint := *c.baseURL
		endpoint.Path = path.Join(
			endpoint.Path,
			"users",
			url.PathEscape(username.String()),
			"repos",
		)
		query := endpoint.Query()
		query.Set("type", "owner")
		query.Set("sort", "updated")
		query.Set("direction", "desc")
		query.Set("per_page", strconv.Itoa(pageSize))
		query.Set("page", strconv.Itoa(pageNumber))
		endpoint.RawQuery = query.Encode()

		response, err := c.do(
			ctx,
			operationListRepositories,
			endpoint.String(),
		)
		if err != nil {
			return nil, rateLimit, err
		}

		rateLimit = parseRateLimit(response.Header)
		if statusErr := responseError(response.StatusCode, rateLimit); statusErr != nil {
			drainAndClose(response.Body)
			return nil, rateLimit, statusErr
		}

		var payload []repositoryResponse
		decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
		decodeErr := decoder.Decode(&payload)
		_ = response.Body.Close()
		if decodeErr != nil {
			return nil, rateLimit, &port.GitHubError{
				Kind:  port.GitHubErrorUpstream,
				Cause: fmt.Errorf("decode GitHub repository response: %w", decodeErr),
			}
		}

		for _, item := range payload {
			repositories = append(repositories, item.toDomain())
			if len(repositories) == limit {
				break
			}
		}

		if len(payload) == 0 || !hasNextPage(response.Header.Get("Link")) {
			break
		}
	}

	return repositories, rateLimit, nil
}

func upstreamDecodeError(description string, err error) error {
	return &port.GitHubError{
		Kind:  port.GitHubErrorUpstream,
		Cause: fmt.Errorf("decode %s: %w", description, err),
	}
}

func (c *Client) do(
	ctx context.Context,
	operation string,
	endpoint string,
) (*http.Response, error) {
	return c.doRequest(ctx, operation, func() (*http.Request, error) {
		return c.newRequest(ctx, http.MethodGet, endpoint, nil)
	})
}

func (c *Client) doRequest(
	ctx context.Context,
	operation string,
	createRequest requestFactory,
) (finalResponse *http.Response, finalErr error) {
	startedAt := time.Now()
	attempts := 0
	defer func() {
		c.logUpstreamRequest(
			ctx,
			operation,
			finalResponse,
			finalErr,
			attempts,
			time.Since(startedAt),
		)
	}()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		attempts = attempt + 1
		request, err := createRequest()
		if err != nil {
			return nil, &port.GitHubError{
				Kind:  port.GitHubErrorUpstream,
				Cause: fmt.Errorf("create GitHub request: %w", err),
			}
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

func (c *Client) logUpstreamRequest(
	ctx context.Context,
	operation string,
	response *http.Response,
	err error,
	attempts int,
	duration time.Duration,
) {
	attributes := []any{
		"upstreamService", upstreamServiceGitHub,
		"operation", operation,
		"outcome", upstreamOutcome(response, err),
		"attempts", attempts,
		"latencyMs", duration.Milliseconds(),
	}
	if requestID := requestcontext.RequestID(ctx); requestID != "" {
		attributes = append(attributes, "requestId", requestID)
	}
	if response != nil {
		attributes = append(attributes, "status", response.StatusCode)
	}
	c.logger.Info("upstream request completed", attributes...)
}

func upstreamOutcome(response *http.Response, err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return upstreamOutcomeCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return upstreamOutcomeDeadline
	case err != nil:
		return upstreamOutcomeTransportError
	case response == nil:
		return upstreamOutcomeTransportError
	case response.StatusCode >= http.StatusOK &&
		response.StatusCode < http.StatusMultipleChoices:
		return upstreamOutcomeSuccess
	case response.StatusCode == http.StatusNotFound:
		return upstreamOutcomeNotFound
	case response.StatusCode == http.StatusTooManyRequests ||
		(response.StatusCode == http.StatusForbidden &&
			parseRateLimit(response.Header).Known &&
			parseRateLimit(response.Header).Remaining == 0):
		return upstreamOutcomeRateLimited
	case response.StatusCode == http.StatusUnauthorized ||
		response.StatusCode == http.StatusForbidden:
		return upstreamOutcomeUnauthorized
	default:
		return upstreamOutcomeResponseError
	}
}

func (c *Client) newRequest(
	ctx context.Context,
	method string,
	endpoint string,
	body io.Reader,
) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", "IssueScout")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	return request, nil
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

func hasNextPage(linkHeader string) bool {
	for _, link := range strings.Split(linkHeader, ",") {
		if strings.Contains(link, `rel="next"`) {
			return true
		}
	}
	return false
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

type repositoryResponse struct {
	ID            int64      `json:"id"`
	Owner         owner      `json:"owner"`
	Name          string     `json:"name"`
	FullName      string     `json:"full_name"`
	Description   *string    `json:"description"`
	HTMLURL       string     `json:"html_url"`
	Language      *string    `json:"language"`
	Stars         int        `json:"stargazers_count"`
	Forks         int        `json:"forks_count"`
	OpenIssues    int        `json:"open_issues_count"`
	Fork          bool       `json:"fork"`
	Archived      bool       `json:"archived"`
	DefaultBranch string     `json:"default_branch"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PushedAt      *time.Time `json:"pushed_at"`
}

type owner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

func (r repositoryResponse) toDomain() repository.Summary {
	var pushedAt time.Time
	if r.PushedAt != nil {
		pushedAt = r.PushedAt.UTC()
	}
	return repository.Summary{
		ID:            r.ID,
		Owner:         r.Owner.Login,
		Name:          r.Name,
		FullName:      r.FullName,
		Description:   stringValue(r.Description),
		URL:           r.HTMLURL,
		MainLanguage:  stringValue(r.Language),
		Stars:         r.Stars,
		Forks:         r.Forks,
		OpenIssues:    r.OpenIssues,
		IsFork:        r.Fork,
		IsArchived:    r.Archived,
		DefaultBranch: r.DefaultBranch,
		UpdatedAt:     r.UpdatedAt.UTC(),
		PushedAt:      pushedAt,
	}
}

var _ port.GitHubUserReader = (*Client)(nil)
var _ port.GitHubRepositoryReader = (*Client)(nil)
var _ port.GitHubProfileAnalysisReader = (*Client)(nil)
var _ port.GitHubIssueSearcher = (*Client)(nil)
