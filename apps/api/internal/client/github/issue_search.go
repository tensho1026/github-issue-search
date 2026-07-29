package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	maxIssueSearchResults       = 50
	maxIssueSearchQueryBytes    = 1024
	maxIssueSearchResponseBytes = 8 << 20
)

func (c *Client) SearchIssues(
	ctx context.Context,
	criteria issue.SearchCriteria,
	limit int,
) (port.GitHubIssueSearchResult, error) {
	if limit < 1 || limit > maxIssueSearchResults {
		return port.GitHubIssueSearchResult{}, fmt.Errorf(
			"GitHub issue search limit must be between 1 and %d",
			maxIssueSearchResults,
		)
	}

	searchQuery, err := buildIssueSearchQuery(criteria, c.now())
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}

	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "search", "issues")
	query := endpoint.Query()
	query.Set("q", searchQuery)
	query.Set("sort", "updated")
	query.Set("order", "desc")
	query.Set("per_page", strconv.Itoa(limit))
	query.Set("page", "1")
	endpoint.RawQuery = query.Encode()

	response, err := c.do(ctx, endpoint.String())
	if err != nil {
		return port.GitHubIssueSearchResult{}, err
	}
	defer response.Body.Close()

	rateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, rateLimit); statusErr != nil {
		return port.GitHubIssueSearchResult{}, statusErr
	}

	var payload issueSearchResponse
	decoder := json.NewDecoder(
		io.LimitReader(response.Body, maxIssueSearchResponseBytes),
	)
	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		return port.GitHubIssueSearchResult{}, upstreamDecodeError(
			"GitHub issue search response",
			decodeErr,
		)
	}
	if payload.TotalCount < len(payload.Items) || len(payload.Items) > limit {
		return port.GitHubIssueSearchResult{}, upstreamDecodeError(
			"GitHub issue search response",
			fmt.Errorf("contains invalid result counts"),
		)
	}

	candidates := make([]issue.Candidate, 0, len(payload.Items))
	for index, item := range payload.Items {
		candidate, mappingErr := item.toDomain()
		if mappingErr != nil {
			return port.GitHubIssueSearchResult{}, upstreamDecodeError(
				"GitHub issue search response",
				fmt.Errorf("item %d: %w", index, mappingErr),
			)
		}
		candidates = append(candidates, candidate)
	}

	c.logger.Debug(
		"GitHub issue search response received",
		"status", response.StatusCode,
		"candidateCount", len(candidates),
		"upstreamTotal", payload.TotalCount,
		"incompleteResults", payload.IncompleteResults,
		"rateLimitKnown", rateLimit.Known,
		"rateLimitRemaining", rateLimit.Remaining,
	)

	return port.GitHubIssueSearchResult{
		Candidates:        candidates,
		TotalCount:        payload.TotalCount,
		IncompleteResults: payload.IncompleteResults,
		RateLimit:         rateLimit,
	}, nil
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

type issueSearchResponse struct {
	TotalCount        int               `json:"total_count"`
	IncompleteResults bool              `json:"incomplete_results"`
	Items             []issueSearchItem `json:"items"`
}

type issueSearchItem struct {
	Number      int                  `json:"number"`
	Title       string               `json:"title"`
	Body        *string              `json:"body"`
	HTMLURL     string               `json:"html_url"`
	State       string               `json:"state"`
	Labels      []issueLabelResponse `json:"labels"`
	Assignees   []owner              `json:"assignees"`
	User        owner                `json:"user"`
	Comments    int                  `json:"comments"`
	PullRequest json.RawMessage      `json:"pull_request"`
	Locked      bool                 `json:"locked"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	Repository  repositoryResponse   `json:"repository"`
}

type issueLabelResponse struct {
	Name string `json:"name"`
}

func (item issueSearchItem) toDomain() (issue.Candidate, error) {
	if item.Number < 1 ||
		strings.TrimSpace(item.Title) == "" ||
		strings.TrimSpace(item.HTMLURL) == "" ||
		strings.TrimSpace(item.State) == "" ||
		strings.TrimSpace(item.User.Login) == "" ||
		item.Comments < 0 ||
		item.CreatedAt.IsZero() ||
		item.UpdatedAt.IsZero() {
		return issue.Candidate{}, errors.New("contains invalid issue fields")
	}
	if err := validateAbsoluteHTTPURL(item.HTMLURL); err != nil {
		return issue.Candidate{}, fmt.Errorf("invalid issue URL: %w", err)
	}

	repository := item.Repository.toDomain()
	if repository.ID < 1 ||
		strings.TrimSpace(repository.Owner) == "" ||
		strings.TrimSpace(repository.Name) == "" ||
		strings.TrimSpace(repository.FullName) == "" ||
		strings.TrimSpace(repository.URL) == "" ||
		repository.Stars < 0 ||
		repository.Forks < 0 ||
		repository.OpenIssues < 0 ||
		repository.UpdatedAt.IsZero() {
		return issue.Candidate{}, errors.New("contains invalid repository fields")
	}
	if err := validateAbsoluteHTTPURL(repository.URL); err != nil {
		return issue.Candidate{}, fmt.Errorf("invalid repository URL: %w", err)
	}

	labels := make([]string, 0, len(item.Labels))
	for _, label := range item.Labels {
		name := strings.TrimSpace(label.Name)
		if name == "" {
			return issue.Candidate{}, errors.New("contains a blank label")
		}
		labels = append(labels, name)
	}
	assignees := make([]string, 0, len(item.Assignees))
	for _, assignee := range item.Assignees {
		login := strings.TrimSpace(assignee.Login)
		if login == "" {
			return issue.Candidate{}, errors.New("contains a blank assignee")
		}
		assignees = append(assignees, login)
	}

	return issue.Candidate{
		Repository: repository,
		Issue: issue.Summary{
			Number:      item.Number,
			Title:       strings.TrimSpace(item.Title),
			Body:        stringValue(item.Body),
			URL:         item.HTMLURL,
			State:       item.State,
			Labels:      labels,
			Assignees:   assignees,
			AuthorLogin: item.User.Login,
			AuthorType:  item.User.Type,
			Comments:    item.Comments,
			IsPullRequest: len(item.PullRequest) > 0 &&
				string(item.PullRequest) != "null",
			Locked:    item.Locked,
			CreatedAt: item.CreatedAt.UTC(),
			UpdatedAt: item.UpdatedAt.UTC(),
		},
	}, nil
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
