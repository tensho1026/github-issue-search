package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const validGraphQLIssueNode = `{
	"__typename":"Issue",
	"number":123,
	"title":"Add request validation",
	"body":"The Gin handler needs validation, clear errors, regression tests, and acceptance criteria.",
	"url":"https://github.com/example/example-api/issues/123",
	"state":"OPEN",
	"locked":false,
	"createdAt":"2026-07-28T10:00:00Z",
	"updatedAt":"2026-07-30T10:00:00Z",
	"comments":{"totalCount":4},
	"author":{"login":"contributor","__typename":"User"},
	"labels":{"nodes":[{"name":"good first issue"}]},
	"assignees":{"nodes":[]},
	"repository":{
		"databaseId":99,
		"owner":{"login":"example","__typename":"Organization"},
		"name":"example-api",
		"nameWithOwner":"example/example-api",
		"description":"A production Gin service",
		"url":"https://github.com/example/example-api",
		"primaryLanguage":{"name":"Go"},
		"stargazerCount":120,
		"forkCount":3,
		"issues":{"totalCount":7},
		"isFork":false,
		"isArchived":false,
		"defaultBranchRef":{"name":"main"},
		"updatedAt":"2026-07-30T09:00:00Z",
		"pushedAt":"2026-07-30T09:00:00Z"
	}
}`

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

func TestSearchIssuesPostsGraphQLAndNormalizesPayload(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if request.Method != http.MethodPost {
			t.Errorf("method = %q", request.Method)
		}
		if request.URL.Path != "/graphql" || request.URL.RawQuery != "" {
			t.Errorf("URL = %q", request.URL.String())
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}

		var payload graphQLIssueSearchRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload.Query != graphQLIssueSearchDocument {
			t.Error("request did not use the pinned GraphQL document")
		}
		if payload.Variables.First != 50 {
			t.Errorf("first = %d", payload.Variables.First)
		}
		if strings.Contains(payload.Variables.SearchQuery, `language:`) {
			t.Errorf("unexpected language filter = %q", payload.Variables.SearchQuery)
		}
		if !strings.Contains(
			payload.Variables.SearchQuery,
			`label:"good first issue","help wanted"`,
		) {
			t.Errorf("searchQuery = %q", payload.Variables.SearchQuery)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, graphQLIssueSearchResponse(
			1234,
			validGraphQLIssueNode,
		))
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
		result.IncompleteResults ||
		result.RateLimit.Limit != 5000 ||
		result.RateLimit.Remaining != 29 ||
		len(result.Candidates) != 1 {
		t.Fatalf("SearchIssues() result = %+v, requests = %d", result, requests.Load())
	}
	candidate := result.Candidates[0]
	if candidate.Repository.ID != 99 ||
		candidate.Repository.FullName != "example/example-api" ||
		candidate.Repository.MainLanguage != "Go" ||
		candidate.Repository.DefaultBranch != "main" ||
		candidate.Repository.Stars != 120 ||
		candidate.Issue.Number != 123 ||
		candidate.Issue.AuthorType != "User" ||
		candidate.Issue.State != "OPEN" ||
		candidate.Issue.IsPullRequest ||
		len(candidate.Issue.Labels) != 1 {
		t.Fatalf("candidate = %+v", candidate)
	}
}

func TestSearchIssuesAllowsDeletedAuthorAndMissingOptionalRepositoryFields(
	t *testing.T,
) {
	node := strings.NewReplacer(
		`"author":{"login":"contributor","__typename":"User"}`,
		`"author":null`,
		`"databaseId":99,`,
		`"databaseId":null,`,
		`"primaryLanguage":{"name":"Go"}`,
		`"primaryLanguage":null`,
		`"defaultBranchRef":{"name":"main"}`,
		`"defaultBranchRef":null`,
		`"pushedAt":"2026-07-30T09:00:00Z"`,
		`"pushedAt":null`,
	).Replace(validGraphQLIssueNode)
	server := jsonServer(graphQLIssueSearchResponse(1, node))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "token").SearchIssues(
		context.Background(),
		issueSearchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"}),
		1,
	)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	candidate := result.Candidates[0]
	if candidate.Issue.AuthorLogin != "ghost" ||
		candidate.Issue.AuthorType != "Unknown" ||
		candidate.Repository.ID != 0 ||
		candidate.Repository.MainLanguage != "" ||
		candidate.Repository.DefaultBranch != "" ||
		!candidate.Repository.PushedAt.IsZero() {
		t.Fatalf("candidate = %+v", candidate)
	}
}

func TestSearchIssuesReturnsValidNodesWithPartialError(t *testing.T) {
	server := jsonServer(`{
		"data":{
			"search":{
				"issueCount":2,
				"pageInfo":{"hasNextPage":false},
				"nodes":[` + validGraphQLIssueNode + `,null]
			},
			"rateLimit":{
				"limit":5000,
				"remaining":20,
				"resetAt":"2026-07-30T13:00:00Z"
			}
		},
		"errors":[{"type":"INTERNAL","message":"one node could not be resolved"}]
	}`)
	defer server.Close()

	result, err := newTestClient(t, server.URL, "token").SearchIssues(
		context.Background(),
		issueSearchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"}),
		2,
	)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if !result.IncompleteResults ||
		result.TotalCount != 2 ||
		len(result.Candidates) != 1 {
		t.Fatalf("result = %+v", result)
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

	for _, limit := range []int{0, issue.MaximumCandidateResults + 1} {
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
		{name: "malformed JSON", body: `{"data":`},
		{
			name: "negative total",
			body: `{"data":{"search":{"issueCount":-1,"nodes":[]}}}`,
		},
		{
			name: "invalid node",
			body: `{"data":{"search":{"issueCount":1,"nodes":[
				{"__typename":"Issue","number":0}
			]}}}`,
		},
		{
			name: "unexpected node type",
			body: `{"data":{"search":{"issueCount":1,"nodes":[
				{"__typename":"PullRequest"}
			]}}}`,
		},
		{
			name: "invalid rate limit",
			body: `{"data":{
				"search":{"issueCount":0,"nodes":[]},
				"rateLimit":{
					"limit":10,
					"remaining":11,
					"resetAt":"2026-07-30T13:00:00Z"
				}
			}}`,
		},
		{
			name: "missing search data",
			body: `{"data":{"rateLimit":{
				"limit":10,
				"remaining":9,
				"resetAt":"2026-07-30T13:00:00Z"
			}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := jsonServer(test.body)
			defer server.Close()

			_, err := newTestClient(t, server.URL, "token").SearchIssues(
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

func TestSearchIssuesMapsGraphQLErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		kind port.GitHubErrorKind
	}{
		{
			name: "rate limited",
			body: `{"data":{
				"search":null,
				"rateLimit":{
					"limit":5000,
					"remaining":0,
					"resetAt":"2026-07-30T13:00:00Z"
				}
			},"errors":[{"type":"RATE_LIMITED","message":"rate limit exceeded"}]}`,
			kind: port.GitHubErrorRateLimited,
		},
		{
			name: "forbidden",
			body: `{"data":{"search":null},"errors":[
				{"extensions":{"code":"FORBIDDEN"},"message":"denied"}
			]}`,
			kind: port.GitHubErrorUnauthorized,
		},
		{
			name: "upstream",
			body: `{"data":{"search":null},"errors":[
				{"type":"INTERNAL","message":"unavailable"}
			]}`,
			kind: port.GitHubErrorUpstream,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := jsonServer(test.body)
			defer server.Close()

			_, err := newTestClient(t, server.URL, "token").SearchIssues(
				context.Background(),
				issueSearchCriteria(
					t,
					issue.SearchCriteriaOptions{Username: "octocat"},
				),
				50,
			)
			if !port.IsGitHubError(err, test.kind) {
				t.Fatalf("SearchIssues() error = %v", err)
			}
			if test.kind == port.GitHubErrorRateLimited {
				var gitHubError *port.GitHubError
				if !errors.As(err, &gitHubError) ||
					gitHubError.Reset.IsZero() {
					t.Fatalf("rate-limit error = %+v", gitHubError)
				}
			}
		})
	}
}

func TestSearchIssuesRetriesTransientGraphQLRequestWithCompleteBody(
	t *testing.T,
) {
	var requests atomic.Int32
	var bodiesMu sync.Mutex
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		bodiesMu.Lock()
		bodies = append(bodies, body)
		bodiesMu.Unlock()

		if requests.Add(1) == 1 {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, graphQLIssueSearchResponse(1, validGraphQLIssueNode))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "token")
	client.sleep = func(context.Context, time.Duration) error { return nil }
	result, err := client.SearchIssues(
		context.Background(),
		issueSearchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"}),
		1,
	)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if requests.Load() != 2 || len(result.Candidates) != 1 {
		t.Fatalf("requests = %d, result = %+v", requests.Load(), result)
	}
	bodiesMu.Lock()
	defer bodiesMu.Unlock()
	if len(bodies) != 2 ||
		len(bodies[0]) == 0 ||
		string(bodies[0]) != string(bodies[1]) {
		t.Fatalf("request bodies were not recreated: lengths = %d, %d", len(bodies[0]), len(bodies[1]))
	}
}

func TestSearchIssuesUsesHeaderRateLimitWhenGraphQLMetadataIsMissing(
	t *testing.T,
) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Set("X-RateLimit-Limit", "30")
		writer.Header().Set("X-RateLimit-Remaining", "29")
		writer.Header().Set("X-RateLimit-Reset", "1785416400")
		_, _ = io.WriteString(writer, `{"data":{
			"search":{"issueCount":0,"nodes":[]}
		}}`)
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "token").SearchIssues(
		context.Background(),
		issueSearchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"}),
		1,
	)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if !result.RateLimit.Known ||
		result.RateLimit.Limit != 30 ||
		result.RateLimit.Remaining != 29 ||
		result.RateLimit.Reset.IsZero() {
		t.Fatalf("rate limit = %+v", result.RateLimit)
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

func graphQLIssueSearchResponse(total int, nodes ...string) string {
	return `{
		"data":{
			"search":{
				"issueCount":` + strconv.Itoa(total) + `,
				"pageInfo":{"hasNextPage":true},
				"nodes":[` + strings.Join(nodes, ",") + `]
			},
			"rateLimit":{
				"limit":5000,
				"remaining":29,
				"resetAt":"2026-07-30T13:00:00Z"
			}
		}
	}`
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
