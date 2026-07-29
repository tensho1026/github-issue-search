package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	apiVersion       = "2022-11-28"
	maxAttempts      = 3
	maxResponseBytes = 2 << 20
	maxManifestBytes = 512 << 10
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
	now        func() time.Time
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
		now:     time.Now,
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

		response, err := c.do(ctx, endpoint.String())
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

func (c *Client) GetRepositoryLanguages(
	ctx context.Context,
	owner string,
	name string,
) (port.GitHubLanguagesResult, error) {
	endpoint := c.repositoryEndpoint(owner, name, "languages")
	response, err := c.do(ctx, endpoint.String())
	if err != nil {
		return port.GitHubLanguagesResult{}, err
	}
	defer response.Body.Close()

	rateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, rateLimit); statusErr != nil {
		return port.GitHubLanguagesResult{}, statusErr
	}

	languages := make(map[string]int64)
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if decodeErr := decoder.Decode(&languages); decodeErr != nil {
		return port.GitHubLanguagesResult{}, upstreamDecodeError(
			"GitHub language response",
			decodeErr,
		)
	}
	for language, count := range languages {
		if strings.TrimSpace(language) == "" || count < 0 {
			return port.GitHubLanguagesResult{}, upstreamDecodeError(
				"GitHub language response",
				fmt.Errorf("contains invalid language byte count"),
			)
		}
	}

	return port.GitHubLanguagesResult{
		Languages: languages,
		RateLimit: rateLimit,
	}, nil
}

func (c *Client) GetRepositoryFile(
	ctx context.Context,
	owner string,
	name string,
	filePath string,
) (port.GitHubRepositoryFileResult, error) {
	endpoint := c.repositoryEndpoint(owner, name, "contents", filePath)
	response, err := c.do(ctx, endpoint.String())
	if err != nil {
		return port.GitHubRepositoryFileResult{}, err
	}
	defer response.Body.Close()

	rateLimit := parseRateLimit(response.Header)
	if response.StatusCode == http.StatusNotFound {
		return port.GitHubRepositoryFileResult{
			Exists:    false,
			RateLimit: rateLimit,
		}, nil
	}
	if statusErr := responseError(response.StatusCode, rateLimit); statusErr != nil {
		return port.GitHubRepositoryFileResult{}, statusErr
	}

	var payload repositoryFileResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		return port.GitHubRepositoryFileResult{}, upstreamDecodeError(
			"GitHub repository file response",
			decodeErr,
		)
	}
	if payload.Encoding != "base64" {
		return port.GitHubRepositoryFileResult{}, upstreamDecodeError(
			"GitHub repository file response",
			fmt.Errorf("unsupported content encoding %q", payload.Encoding),
		)
	}

	content, decodeErr := base64.StdEncoding.DecodeString(payload.Content)
	if decodeErr != nil {
		return port.GitHubRepositoryFileResult{}, upstreamDecodeError(
			"GitHub repository file content",
			decodeErr,
		)
	}
	if len(content) > maxManifestBytes {
		return port.GitHubRepositoryFileResult{}, upstreamDecodeError(
			"GitHub repository file response",
			fmt.Errorf("decoded content exceeds %d bytes", maxManifestBytes),
		)
	}

	return port.GitHubRepositoryFileResult{
		Content:   content,
		Exists:    true,
		RateLimit: rateLimit,
	}, nil
}

func (c *Client) repositoryEndpoint(
	owner string,
	name string,
	segments ...string,
) url.URL {
	endpoint := *c.baseURL
	parts := []string{
		endpoint.Path,
		"repos",
		url.PathEscape(owner),
		url.PathEscape(name),
	}
	parts = append(parts, segments...)
	endpoint.Path = path.Join(parts...)
	return endpoint
}

func upstreamDecodeError(description string, err error) error {
	return &port.GitHubError{
		Kind:  port.GitHubErrorUpstream,
		Cause: fmt.Errorf("decode %s: %w", description, err),
	}
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

type repositoryFileResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
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
