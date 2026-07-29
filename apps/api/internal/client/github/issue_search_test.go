package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestBuildIssueSearchQueryUsesCanonicalSafeQualifiers(t *testing.T) {
	criteria := issueSearchCriteria(t, issue.SearchCriteriaOptions{
		Username:   "octocat",
		Languages:  []string{"TypeScript", "Go"},
		Frameworks: []string{"React", "Gin"},
		Labels:     []string{"help wanted", "good first issue"},
	})
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)

	query, err := buildIssueSearchQuery(criteria, now)
	if err != nil {
		t.Fatalf("buildIssueSearchQuery() error = %v", err)
	}
	want := `is:issue is:open is:public no:assignee archived:false ` +
		`updated:>=2026-01-31 ` +
		`label:"good first issue","help wanted" ` +
		`(language:"Go" OR language:"TypeScript") ` +
		`("Gin" OR "React") in:title,body`
	if query != want {
		t.Fatalf("query =\n%s\nwant\n%s", query, want)
	}
}

func TestSearchIssuesEncodesRequestAndNormalizesPayload(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if request.URL.Path != "/search/issues" {
			t.Errorf("path = %q", request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("sort") != "updated" ||
			query.Get("order") != "desc" ||
			query.Get("per_page") != "50" ||
			query.Get("page") != "1" {
			t.Errorf("query parameters = %s", request.URL.RawQuery)
		}
		if strings.Contains(request.URL.RawQuery, "good first issue") {
			t.Errorf("raw query was not URL encoded: %s", request.URL.RawQuery)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-RateLimit-Limit", "30")
		writer.Header().Set("X-RateLimit-Remaining", "29")
		_, _ = io.WriteString(writer, `{
			"total_count":1234,
			"incomplete_results":true,
			"items":[{
				"number":123,
				"title":"Add request validation",
				"body":"The Gin handler needs validation, clear errors, regression tests, and acceptance criteria.",
				"html_url":"https://github.com/example/example-api/issues/123",
				"state":"open",
				"labels":[{"name":"good first issue"}],
				"assignees":[],
				"user":{"login":"contributor","type":"User"},
				"comments":4,
				"locked":false,
				"created_at":"2026-07-28T10:00:00Z",
				"updated_at":"2026-07-30T10:00:00Z",
				"repository":{
					"id":99,
					"owner":{"login":"example"},
					"name":"example-api",
					"full_name":"example/example-api",
					"description":"A production Gin service",
					"html_url":"https://github.com/example/example-api",
					"language":"Go",
					"stargazers_count":120,
					"forks_count":3,
					"open_issues_count":7,
					"fork":false,
					"archived":false,
					"default_branch":"main",
					"updated_at":"2026-07-30T09:00:00Z",
					"pushed_at":"2026-07-30T09:00:00Z"
				}
			}]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	client.now = func() time.Time { return now }
	result, err := client.SearchIssues(
		context.Background(),
		issueSearchCriteria(t, issue.SearchCriteriaOptions{
			Username: "octocat",
		}),
		50,
	)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}

	if requests.Load() != 1 ||
		result.TotalCount != 1234 ||
		!result.IncompleteResults ||
		result.RateLimit.Remaining != 29 ||
		len(result.Candidates) != 1 {
		t.Fatalf("SearchIssues() result = %+v, requests = %d", result, requests.Load())
	}
	candidate := result.Candidates[0]
	if candidate.Repository.FullName != "example/example-api" ||
		candidate.Repository.MainLanguage != "Go" ||
		candidate.Repository.Stars != 120 ||
		candidate.Issue.Number != 123 ||
		candidate.Issue.AuthorType != "User" ||
		candidate.Issue.IsPullRequest ||
		len(candidate.Issue.Labels) != 1 {
		t.Fatalf("candidate = %+v", candidate)
	}
}

func TestSearchIssuesMarksPullRequestsWithoutLeakingPayload(t *testing.T) {
	server := jsonServer(`{
		"total_count":1,
		"incomplete_results":false,
		"items":[{
			"number":1,
			"title":"A valid pull request title",
			"body":"This body contains enough meaningful information to pass normalization checks.",
			"html_url":"https://github.com/example/repo/pull/1",
			"state":"open",
			"labels":[],
			"assignees":[],
			"user":{"login":"octocat","type":"User"},
			"comments":0,
			"pull_request":{"url":"https://api.github.com/repos/example/repo/pulls/1"},
			"created_at":"2026-07-28T10:00:00Z",
			"updated_at":"2026-07-30T10:00:00Z",
			"repository":{
				"id":1,
				"owner":{"login":"example"},
				"name":"repo",
				"full_name":"example/repo",
				"html_url":"https://github.com/example/repo",
				"stargazers_count":10,
				"forks_count":0,
				"open_issues_count":1,
				"updated_at":"2026-07-30T09:00:00Z"
			}
		}]
	}`)
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").SearchIssues(
		context.Background(),
		issueSearchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"}),
		1,
	)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if !result.Candidates[0].Issue.IsPullRequest {
		t.Fatal("pull request marker was not normalized")
	}
}

func TestSearchIssuesRejectsInvalidLimitsBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	client := newTestClient(t, "https://api.github.example", "")
	client.httpClient = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("must not be called")
	})
	criteria := issueSearchCriteria(t, issue.SearchCriteriaOptions{
		Username: "octocat",
	})

	for _, limit := range []int{0, maxIssueSearchResults + 1} {
		if _, err := client.SearchIssues(
			context.Background(),
			criteria,
			limit,
		); err == nil {
			t.Fatalf("SearchIssues(limit=%d) error = nil", limit)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func TestSearchIssuesRejectsOversizedQueryBeforeRequest(t *testing.T) {
	values := make([]string, issue.MaximumFilterValues)
	for index := range values {
		values[index] = strings.Repeat(string(rune('a'+index)), 64)
	}
	criteria := issueSearchCriteria(t, issue.SearchCriteriaOptions{
		Username:   "octocat",
		Languages:  values,
		Frameworks: values,
		Labels:     values,
	})
	var requests atomic.Int32
	client := newTestClient(t, "https://api.github.example", "")
	client.httpClient = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("must not be called")
	})

	_, err := client.SearchIssues(context.Background(), criteria, 50)
	if !errors.Is(err, issue.ErrInvalidSearchCriteria) {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func TestSearchIssuesRejectsMalformedUpstreamResults(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"items":`},
		{
			name: "negative total",
			body: `{"total_count":-1,"items":[]}`,
		},
		{
			name: "invalid item",
			body: `{"total_count":1,"items":[{"number":0}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := jsonServer(test.body)
			defer server.Close()

			_, err := newTestClient(t, server.URL, "").SearchIssues(
				context.Background(),
				issueSearchCriteria(
					t,
					issue.SearchCriteriaOptions{Username: "octocat"},
				),
				50,
			)
			if !port.IsGitHubError(err, port.GitHubErrorUpstream) {
				t.Fatalf("SearchIssues() error = %v", err)
			}
		})
	}
}

func TestSearchIssuesPropagatesCancellation(t *testing.T) {
	var requests atomic.Int32
	client := newTestClient(t, "https://api.github.example", "")
	client.httpClient = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.SearchIssues(
		ctx,
		issueSearchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"}),
		50,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
}

func issueSearchCriteria(
	t *testing.T,
	options issue.SearchCriteriaOptions,
) issue.SearchCriteria {
	t.Helper()
	criteria, err := issue.NewSearchCriteria(options)
	if err != nil {
		t.Fatalf("NewSearchCriteria() error = %v", err)
	}
	return criteria
}
