package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	maxIssueSearchQueryBytes    = 1024
	maxIssueSearchResponseBytes = 8 << 20

	graphQLIssueSearchDocument = `query IssueScoutIssueSearch($searchQuery: String!, $first: Int!) {
  search(query: $searchQuery, type: ISSUE, first: $first) {
    issueCount
    pageInfo {
      hasNextPage
    }
    nodes {
      __typename
      ... on Issue {
        number
        title
        body
        url
        state
        locked
        createdAt
        updatedAt
        comments {
          totalCount
        }
        author {
          login
          __typename
        }
        labels(first: 100) {
          nodes {
            name
          }
        }
        assignees(first: 10) {
          nodes {
            login
          }
        }
        repository {
          databaseId
          name
          nameWithOwner
          url
          description
          stargazerCount
          forkCount
          isFork
          isArchived
          updatedAt
          pushedAt
          primaryLanguage {
            name
          }
          defaultBranchRef {
            name
          }
          owner {
            login
          }
          issues(states: OPEN) {
            totalCount
          }
        }
      }
    }
  }
  rateLimit {
    limit
    remaining
    resetAt
  }
}`
)

// SearchIssues retrieves one bounded GraphQL search window and normalizes each
// issue together with its repository. The single query avoids an N+1
// repository-detail fan-out.
func (c *Client) SearchIssues(
	ctx context.Context,
	criteria issue.SearchCriteria,
	limit int,
) (port.GitHubIssueSearchResult, error) {
	if limit < 1 || limit > issue.MaximumCandidateResults {
		return port.GitHubIssueSearchResult{}, fmt.Errorf(
			"GitHub issue search limit must be between 1 and %d",
			issue.MaximumCandidateResults,
		)
	}

	searchQuery, err := buildIssueSearchQuery(criteria, c.now())
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}
	requestPayload, err := json.Marshal(graphQLIssueSearchRequest{
		Query: graphQLIssueSearchDocument,
		Variables: graphQLIssueSearchVariables{
			SearchQuery: searchQuery,
			First:       limit,
		},
	})
	if err != nil {
		return port.GitHubIssueSearchResult{}, upstreamDecodeError(
			"GitHub GraphQL issue search request",
			err,
		)
	}

	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "graphql")
	endpoint.RawQuery = ""
	response, err := c.doRequest(ctx, func() (*http.Request, error) {
		request, requestErr := c.newRequest(
			ctx,
			http.MethodPost,
			endpoint.String(),
			bytes.NewReader(requestPayload),
		)
		if requestErr != nil {
			return nil, requestErr
		}
		request.Header.Set("Content-Type", "application/json")
		return request, nil
	})
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}
	defer response.Body.Close()

	headerRateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, headerRateLimit); statusErr != nil {
		return port.GitHubIssueSearchResult{}, statusErr
	}

	payload, err := decodeGraphQLIssueSearchResponse(response.Body)
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}
	rateLimit, err := normalizeGraphQLRateLimit(payload.Data.RateLimit, headerRateLimit)
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}

	result, err := normalizeGraphQLIssueSearch(payload, limit, rateLimit)
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}

	c.logger.Debug(
		"GitHub GraphQL issue search response received",
		"status", response.StatusCode,
		"candidateCount", len(result.Candidates),
		"upstreamTotal", result.TotalCount,
		"incompleteResults", result.IncompleteResults,
		"rateLimitKnown", result.RateLimit.Known,
		"rateLimitRemaining", result.RateLimit.Remaining,
	)
	return result, nil
}

func decodeGraphQLIssueSearchResponse(
	body io.Reader,
) (graphQLIssueSearchEnvelope, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxIssueSearchResponseBytes+1))
	if err != nil {
		return graphQLIssueSearchEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL issue search response",
			err,
		)
	}
	if len(raw) > maxIssueSearchResponseBytes {
		return graphQLIssueSearchEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL issue search response",
			fmt.Errorf("exceeds %d bytes", maxIssueSearchResponseBytes),
		)
	}

	var payload graphQLIssueSearchEnvelope
	if err := json.Unmarshal(raw, &payload); err != nil {
		return graphQLIssueSearchEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL issue search response",
			err,
		)
	}
	return payload, nil
}

func normalizeGraphQLIssueSearch(
	payload graphQLIssueSearchEnvelope,
	limit int,
	rateLimit port.RateLimit,
) (port.GitHubIssueSearchResult, error) {
	if payload.Data.Search == nil {
		if len(payload.Errors) > 0 {
			return port.GitHubIssueSearchResult{}, graphQLIssueSearchError(
				payload.Errors,
				rateLimit,
			)
		}
		return port.GitHubIssueSearchResult{}, upstreamDecodeError(
			"GitHub GraphQL issue search response",
			errors.New("does not contain search data"),
		)
	}

	search := payload.Data.Search
	if search.IssueCount < 0 ||
		search.IssueCount < len(search.Nodes) ||
		len(search.Nodes) > limit {
		return port.GitHubIssueSearchResult{}, upstreamDecodeError(
			"GitHub GraphQL issue search response",
			errors.New("contains invalid result counts"),
		)
	}

	candidates := make([]issue.Candidate, 0, len(search.Nodes))
	for index, node := range search.Nodes {
		var candidate issue.Candidate
		var mappingErr error
		if node == nil {
			mappingErr = errors.New("contains a null issue node")
		} else {
			candidate, mappingErr = node.toDomain()
		}
		if mappingErr != nil {
			if len(payload.Errors) > 0 {
				continue
			}
			return port.GitHubIssueSearchResult{}, upstreamDecodeError(
				"GitHub GraphQL issue search response",
				fmt.Errorf("node %d: %w", index, mappingErr),
			)
		}
		candidates = append(candidates, candidate)
	}

	if len(payload.Errors) > 0 && len(candidates) == 0 {
		return port.GitHubIssueSearchResult{}, graphQLIssueSearchError(
			payload.Errors,
			rateLimit,
		)
	}
	return port.GitHubIssueSearchResult{
		Candidates:        candidates,
		TotalCount:        search.IssueCount,
		IncompleteResults: len(payload.Errors) > 0,
		RateLimit:         rateLimit,
	}, nil
}

func normalizeGraphQLRateLimit(
	graphQL *graphQLRateLimit,
	fallback port.RateLimit,
) (port.RateLimit, error) {
	if graphQL == nil {
		return fallback, nil
	}
	if graphQL.Limit == nil ||
		graphQL.Remaining == nil ||
		graphQL.ResetAt == nil ||
		*graphQL.Limit < 0 ||
		*graphQL.Remaining < 0 ||
		*graphQL.Remaining > *graphQL.Limit ||
		graphQL.ResetAt.IsZero() {
		return port.RateLimit{}, upstreamDecodeError(
			"GitHub GraphQL issue search response",
			errors.New("contains invalid rate-limit data"),
		)
	}
	return port.RateLimit{
		Known:     true,
		Limit:     *graphQL.Limit,
		Remaining: *graphQL.Remaining,
		Reset:     graphQL.ResetAt.UTC(),
	}, nil
}

func graphQLIssueSearchError(
	graphQLErrors []graphQLError,
	rateLimit port.RateLimit,
) error {
	return graphQLRequestError(
		"GitHub GraphQL issue search",
		graphQLErrors,
		rateLimit,
	)
}

func graphQLRequestError(
	operation string,
	graphQLErrors []graphQLError,
	rateLimit port.RateLimit,
) error {
	kind := port.GitHubErrorUpstream
	for _, graphQLError := range graphQLErrors {
		classification := strings.ToUpper(strings.Join([]string{
			graphQLError.Type,
			graphQLError.Extensions.Code,
			graphQLError.Message,
		}, " "))
		switch {
		case strings.Contains(classification, "RATE_LIMIT"),
			strings.Contains(classification, "RATE LIMIT"),
			strings.Contains(classification, "ABUSE"):
			kind = port.GitHubErrorRateLimited
		case kind != port.GitHubErrorRateLimited &&
			(strings.Contains(classification, "UNAUTHORIZED") ||
				strings.Contains(classification, "FORBIDDEN")):
			kind = port.GitHubErrorUnauthorized
		}
	}

	var reset time.Time
	if kind == port.GitHubErrorRateLimited {
		reset = rateLimit.Reset
	}
	return &port.GitHubError{
		Kind:  kind,
		Reset: reset,
		Cause: fmt.Errorf(
			"%s returned %d error(s)",
			operation,
			len(graphQLErrors),
		),
	}
}

func buildIssueSearchQuery(
	criteria issue.SearchCriteria,
	now time.Time,
) (string, error) {
	parts := []string{
		"is:issue",
		"is:open",
		"is:public",
		"no:assignee",
	}
	if criteria.ExcludesArchived() {
		parts = append(parts, "archived:false")
	}
	updatedAfter := now.UTC().
		AddDate(0, 0, -criteria.UpdatedWithinDays()).
		Format(time.DateOnly)
	parts = append(parts, "updated:>="+updatedAfter)

	if labels := criteria.Labels(); len(labels) > 0 {
		quotedLabels := make([]string, len(labels))
		for index, label := range labels {
			quotedLabels[index] = quoteSearchValue(label)
		}
		parts = append(parts, "label:"+strings.Join(quotedLabels, ","))
	}
	if languages := criteria.Languages(); len(languages) > 0 {
		qualifiers := make([]string, len(languages))
		for index, language := range languages {
			qualifiers[index] = "language:" + quoteSearchValue(language)
		}
		parts = append(parts, "("+strings.Join(qualifiers, " OR ")+")")
	}
	if frameworks := criteria.Frameworks(); len(frameworks) > 0 {
		terms := make([]string, len(frameworks))
		for index, framework := range frameworks {
			terms[index] = quoteSearchValue(framework)
		}
		parts = append(
			parts,
			"("+strings.Join(terms, " OR ")+")",
			"in:title,body",
		)
	}

	query := strings.Join(parts, " ")
	if len(query) > maxIssueSearchQueryBytes {
		return "", fmt.Errorf(
			"%w: the encoded GitHub search query exceeds %d bytes",
			issue.ErrInvalidSearchCriteria,
			maxIssueSearchQueryBytes,
		)
	}
	return query, nil
}

func quoteSearchValue(value issue.FilterValue) string {
	return `"` + value.String() + `"`
}

type graphQLIssueSearchRequest struct {
	Query     string                      `json:"query"`
	Variables graphQLIssueSearchVariables `json:"variables"`
}

type graphQLIssueSearchVariables struct {
	SearchQuery string `json:"searchQuery"`
	First       int    `json:"first"`
}

type graphQLIssueSearchEnvelope struct {
	Data struct {
		Search    *graphQLIssueSearchConnection `json:"search"`
		RateLimit *graphQLRateLimit             `json:"rateLimit"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLIssueSearchConnection struct {
	IssueCount int                 `json:"issueCount"`
	PageInfo   graphQLPageInfo     `json:"pageInfo"`
	Nodes      []*graphQLIssueNode `json:"nodes"`
}

type graphQLPageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
}

type graphQLIssueNode struct {
	TypeName   string                    `json:"__typename"`
	Number     int                       `json:"number"`
	Title      string                    `json:"title"`
	Body       *string                   `json:"body"`
	URL        string                    `json:"url"`
	State      string                    `json:"state"`
	Locked     bool                      `json:"locked"`
	CreatedAt  time.Time                 `json:"createdAt"`
	UpdatedAt  time.Time                 `json:"updatedAt"`
	Comments   graphQLTotalCount         `json:"comments"`
	Author     *graphQLActor             `json:"author"`
	Labels     graphQLLabelConnection    `json:"labels"`
	Assignees  graphQLAssigneeConnection `json:"assignees"`
	Repository graphQLIssueRepository    `json:"repository"`
}

type graphQLActor struct {
	Login    string `json:"login"`
	TypeName string `json:"__typename"`
}

type graphQLLabelConnection struct {
	Nodes []graphQLLabel `json:"nodes"`
}

type graphQLLabel struct {
	Name string `json:"name"`
}

type graphQLAssigneeConnection struct {
	Nodes []graphQLActor `json:"nodes"`
}

type graphQLTotalCount struct {
	TotalCount int `json:"totalCount"`
}

type graphQLIssueRepository struct {
	DatabaseID      *int64                 `json:"databaseId"`
	Owner           graphQLActor           `json:"owner"`
	Name            string                 `json:"name"`
	NameWithOwner   string                 `json:"nameWithOwner"`
	URL             string                 `json:"url"`
	Description     *string                `json:"description"`
	StargazerCount  int                    `json:"stargazerCount"`
	ForkCount       int                    `json:"forkCount"`
	OpenIssues      graphQLTotalCount      `json:"issues"`
	IsFork          bool                   `json:"isFork"`
	IsArchived      bool                   `json:"isArchived"`
	DefaultBranch   *graphQLRepositoryName `json:"defaultBranchRef"`
	PrimaryLanguage *graphQLRepositoryName `json:"primaryLanguage"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	PushedAt        *time.Time             `json:"pushedAt"`
}

type graphQLRepositoryName struct {
	Name string `json:"name"`
}

type graphQLRateLimit struct {
	Limit     *int       `json:"limit"`
	Remaining *int       `json:"remaining"`
	ResetAt   *time.Time `json:"resetAt"`
}

type graphQLError struct {
	Message    string `json:"message"`
	Type       string `json:"type"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

func (node graphQLIssueNode) toDomain() (issue.Candidate, error) {
	if node.TypeName != "Issue" {
		return issue.Candidate{}, fmt.Errorf(
			"contains unexpected node type %q",
			node.TypeName,
		)
	}
	if node.Number < 1 ||
		strings.TrimSpace(node.Title) == "" ||
		strings.TrimSpace(node.URL) == "" ||
		strings.TrimSpace(node.State) == "" ||
		node.Comments.TotalCount < 0 ||
		node.CreatedAt.IsZero() ||
		node.UpdatedAt.IsZero() {
		return issue.Candidate{}, errors.New("contains invalid issue fields")
	}
	if err := validateAbsoluteHTTPURL(node.URL); err != nil {
		return issue.Candidate{}, fmt.Errorf("invalid issue URL: %w", err)
	}

	authorLogin := "ghost"
	authorType := "Unknown"
	if node.Author != nil {
		authorLogin = strings.TrimSpace(node.Author.Login)
		authorType = strings.TrimSpace(node.Author.TypeName)
		if authorLogin == "" || authorType == "" {
			return issue.Candidate{}, errors.New("contains invalid author fields")
		}
	}

	repositorySummary := node.Repository.toDomain()
	if strings.TrimSpace(repositorySummary.Owner) == "" ||
		strings.TrimSpace(repositorySummary.Name) == "" ||
		strings.TrimSpace(repositorySummary.FullName) == "" ||
		strings.TrimSpace(repositorySummary.URL) == "" ||
		repositorySummary.Stars < 0 ||
		repositorySummary.Forks < 0 ||
		repositorySummary.OpenIssues < 0 ||
		repositorySummary.UpdatedAt.IsZero() {
		return issue.Candidate{}, errors.New("contains invalid repository fields")
	}
	if err := validateAbsoluteHTTPURL(repositorySummary.URL); err != nil {
		return issue.Candidate{}, fmt.Errorf("invalid repository URL: %w", err)
	}

	labels := make([]string, 0, len(node.Labels.Nodes))
	for _, label := range node.Labels.Nodes {
		name := strings.TrimSpace(label.Name)
		if name == "" {
			return issue.Candidate{}, errors.New("contains a blank label")
		}
		labels = append(labels, name)
	}
	assignees := make([]string, 0, len(node.Assignees.Nodes))
	for _, assignee := range node.Assignees.Nodes {
		login := strings.TrimSpace(assignee.Login)
		if login == "" {
			return issue.Candidate{}, errors.New("contains a blank assignee")
		}
		assignees = append(assignees, login)
	}

	return issue.Candidate{
		Repository: repositorySummary,
		Issue: issue.Summary{
			Number:        node.Number,
			Title:         strings.TrimSpace(node.Title),
			Body:          stringValue(node.Body),
			URL:           node.URL,
			State:         node.State,
			Labels:        labels,
			Assignees:     assignees,
			AuthorLogin:   authorLogin,
			AuthorType:    authorType,
			Comments:      node.Comments.TotalCount,
			IsPullRequest: false,
			Locked:        node.Locked,
			CreatedAt:     node.CreatedAt.UTC(),
			UpdatedAt:     node.UpdatedAt.UTC(),
		},
	}, nil
}

func (repositoryResponse graphQLIssueRepository) toDomain() repository.Summary {
	var id int64
	if repositoryResponse.DatabaseID != nil {
		id = *repositoryResponse.DatabaseID
	}
	var pushedAt time.Time
	if repositoryResponse.PushedAt != nil {
		pushedAt = repositoryResponse.PushedAt.UTC()
	}
	var mainLanguage string
	if repositoryResponse.PrimaryLanguage != nil {
		mainLanguage = repositoryResponse.PrimaryLanguage.Name
	}
	var defaultBranch string
	if repositoryResponse.DefaultBranch != nil {
		defaultBranch = repositoryResponse.DefaultBranch.Name
	}
	return repository.Summary{
		ID:            id,
		Owner:         repositoryResponse.Owner.Login,
		Name:          repositoryResponse.Name,
		FullName:      repositoryResponse.NameWithOwner,
		Description:   stringValue(repositoryResponse.Description),
		URL:           repositoryResponse.URL,
		MainLanguage:  mainLanguage,
		Stars:         repositoryResponse.StargazerCount,
		Forks:         repositoryResponse.ForkCount,
		OpenIssues:    repositoryResponse.OpenIssues.TotalCount,
		IsFork:        repositoryResponse.IsFork,
		IsArchived:    repositoryResponse.IsArchived,
		DefaultBranch: defaultBranch,
		UpdatedAt:     repositoryResponse.UpdatedAt.UTC(),
		PushedAt:      pushedAt,
	}
}

func validateAbsoluteHTTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil ||
		parsed.Scheme == "" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		(parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("must be an absolute HTTP(S) URL")
	}
	return nil
}
