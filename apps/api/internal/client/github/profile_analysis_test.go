package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestGetProfileAnalysisUsesOnePublicBoundedGraphQLSnapshot(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if request.Method != http.MethodPost ||
			request.URL.Path != "/graphql" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer server-token" {
			t.Errorf("Authorization = %q", got)
		}
		var payload graphQLProfileAnalysisRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload.Variables.Login != "octocat" ||
			payload.Variables.RepositoryLimit != 2 ||
			!payload.Variables.WindowTo.Equal(now) ||
			!payload.Variables.WindowFrom.Equal(now.AddDate(0, 0, -365)) {
			t.Errorf("variables = %+v", payload.Variables)
		}
		for _, required := range []string{
			"privacy: PUBLIC",
			"isFork: false",
			"isFork: true",
			"includeUserRepositories: false",
			"visibility",
			"author:octocat",
			"created:>=2025-07-30T12:00:00+00:00",
			"created:<=2026-07-30T12:00:00+00:00",
		} {
			if !strings.Contains(
				payload.Query+payload.Variables.PullRequestQuery+
					payload.Variables.IssueQuery,
				required,
			) {
				t.Errorf("request does not contain %q", required)
			}
		}
		for _, forbidden := range []string{
			"restrictedContributionsCount",
			"totalCommitContributions",
			"totalPullRequestContributions",
		} {
			if strings.Contains(payload.Query, forbidden) {
				t.Errorf("query contains private-ambiguous field %q", forbidden)
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, completeProfileAnalysisFixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "server-token")
	client.now = func() time.Time { return now }
	result, err := client.GetProfileAnalysis(
		context.Background(),
		"octocat",
		2,
		3,
	)
	if err != nil {
		t.Fatalf("GetProfileAnalysis() error = %v", err)
	}

	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
	if result.Snapshot.Username != "octocat" ||
		!result.Snapshot.WindowFrom.Equal(now.AddDate(0, 0, -365)) ||
		!result.Snapshot.WindowTo.Equal(now) ||
		!result.RateLimit.Known ||
		result.RateLimit.Remaining != 4975 {
		t.Fatalf("result identity/rate = %+v", result)
	}
	if len(result.Snapshot.Owned.Repositories) != 1 ||
		result.Snapshot.Owned.Total != 2 ||
		!result.Snapshot.Owned.HasMore ||
		result.Snapshot.Owned.Repositories[0].Languages["Go"] != 800 ||
		result.Snapshot.Owned.Repositories[0].Languages["TypeScript"] != 200 ||
		len(result.Snapshot.Owned.Repositories[0].Manifests) != 1 ||
		result.Snapshot.Owned.Repositories[0].Manifests[0].Path != "go.mod" {
		t.Fatalf("owned repositories = %+v", result.Snapshot.Owned)
	}
	if len(result.Snapshot.Contributed.Repositories) != 1 ||
		result.Snapshot.Contributed.Total != 2 ||
		len(result.Snapshot.Forked.Repositories) != 1 ||
		result.Snapshot.Forked.Total != 1 {
		t.Fatalf(
			"contributed/forked = %+v / %+v",
			result.Snapshot.Contributed,
			result.Snapshot.Forked,
		)
	}
	if len(result.Snapshot.Starred.Repositories) != 1 ||
		result.Snapshot.Starred.TotalKnown ||
		!result.Snapshot.Starred.HasMore ||
		result.Snapshot.Starred.Repositories[0].Repository.FullName !=
			"community/public-star" {
		t.Fatalf("starred repositories = %+v", result.Snapshot.Starred)
	}
	contributions := result.Snapshot.Contributions
	if contributions.Commits.Value != 7 ||
		contributions.Commits.Complete ||
		contributions.PullRequestReviews.Value != 3 ||
		contributions.PullRequestsOpened.Value != 4 ||
		!contributions.PullRequestsOpened.Complete ||
		contributions.IssuesOpened.Value != 2 ||
		contributions.RepositoriesTouched.Value != 2 ||
		contributions.RepositoriesTouched.Complete {
		t.Fatalf("contributions = %+v", contributions)
	}
	if len(result.Snapshot.Warnings) != 1 ||
		result.Snapshot.Warnings[0].Code !=
			"private_starred_repositories_excluded" {
		t.Fatalf("warnings = %+v", result.Snapshot.Warnings)
	}
}

func TestGetProfileAnalysisPreservesTypedPartialSegments(t *testing.T) {
	server := jsonServer(`{
		"data":{
			"repositoryOwner":{
				"__typename":"User",
				"login":"octocat",
				"ownedRepositories":null,
				"contributedRepositories":{
					"totalCount":0,
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				},
				"starredRepositories":{
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				},
				"forkedRepositories":{
					"totalCount":0,
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				},
				"contributionsCollection":null
			},
			"authoredPullRequests":null,
			"authoredIssues":null,
			"rateLimit":{
				"limit":5000,
				"remaining":100,
				"resetAt":"2026-07-30T13:00:00Z"
			}
		},
		"errors":[{"type":"FORBIDDEN","message":"optional segment unavailable"}]
	}`)
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").GetProfileAnalysis(
		context.Background(),
		"octocat",
		20,
		3,
	)
	if err != nil {
		t.Fatalf("GetProfileAnalysis() error = %v", err)
	}
	if result.Snapshot.Owned.Available ||
		!result.Snapshot.Contributed.Available ||
		result.Snapshot.Contributions.Commits.Available ||
		result.Snapshot.Contributions.PullRequestsOpened.Available ||
		result.Snapshot.Contributions.RepositoriesTouched.Available {
		t.Fatalf("snapshot = %+v", result.Snapshot)
	}
	codes := make([]string, 0, len(result.Snapshot.Warnings))
	for _, warning := range result.Snapshot.Warnings {
		codes = append(codes, warning.Code)
	}
	for _, expected := range []string{
		"owned_repositories_unavailable",
		"contribution_activity_unavailable",
		"authored_pull_requests_unavailable",
		"authored_issues_unavailable",
		"github_partial_response",
	} {
		if !containsString(codes, expected) {
			t.Errorf("warnings %v do not contain %q", codes, expected)
		}
	}
}

func TestGetProfileAnalysisMapsMissingUser(t *testing.T) {
	server := jsonServer(`{
		"data":{
			"repositoryOwner":null,
			"authoredPullRequests":{"issueCount":0},
			"authoredIssues":{"issueCount":0}
		},
		"errors":[{
			"type":"NOT_FOUND",
			"message":"Could not resolve to a User with the login of 'missing'."
		}]
	}`)
	defer server.Close()

	_, err := newTestClient(t, server.URL, "").GetProfileAnalysis(
		context.Background(),
		"missing",
		20,
		3,
	)
	if !port.IsGitHubError(err, port.GitHubErrorNotFound) {
		t.Fatalf("GetProfileAnalysis() error = %v", err)
	}
}

func TestGetProfileAnalysisSupportsOrganizationRepositoryEvidence(
	t *testing.T,
) {
	server := jsonServer(`{
		"data":{
			"repositoryOwner":{
				"__typename":"Organization",
				"login":"github",
				"ownedRepositories":{
					"totalCount":1,
					"pageInfo":{"hasNextPage":false},
					"nodes":[{
						"databaseId":1,
						"owner":{"login":"github"},
						"name":"roadmap",
						"nameWithOwner":"github/roadmap",
						"url":"https://github.com/github/roadmap",
						"stargazerCount":1000,
						"forkCount":100,
						"issues":{"totalCount":10},
						"isFork":false,
						"isArchived":false,
						"primaryLanguage":{"name":"TypeScript"},
						"updatedAt":"2026-07-30T00:00:00Z",
						"visibility":"PUBLIC",
						"languages":{
							"totalSize":100,
							"edges":[{
								"size":100,
								"node":{"name":"TypeScript"}
							}]
						}
					}]
				},
				"forkedRepositories":{
					"totalCount":0,
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				}
			},
			"authoredPullRequests":{"issueCount":99},
			"authoredIssues":{"issueCount":99}
		}
	}`)
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").GetProfileAnalysis(
		context.Background(),
		"github",
		20,
		3,
	)
	if err != nil {
		t.Fatalf("GetProfileAnalysis() error = %v", err)
	}
	if len(result.Snapshot.Owned.Repositories) != 1 ||
		result.Snapshot.Owned.Repositories[0].Languages["TypeScript"] != 100 ||
		!result.Snapshot.Forked.Available ||
		result.Snapshot.Contributed.Available ||
		result.Snapshot.Starred.Available ||
		result.Snapshot.Contributions.Available {
		t.Fatalf("organization snapshot = %+v", result.Snapshot)
	}
	codes := make([]string, 0, len(result.Snapshot.Warnings))
	for _, warning := range result.Snapshot.Warnings {
		codes = append(codes, warning.Code)
	}
	if !containsString(codes, "organization_activity_unavailable") ||
		!containsString(codes, "contributed_repositories_unavailable") ||
		!containsString(codes, "starred_repositories_unavailable") {
		t.Fatalf("organization warnings = %v", codes)
	}
}

func TestGetProfileAnalysisRejectsNonPublicRequiredRepository(t *testing.T) {
	server := jsonServer(`{
		"data":{
			"repositoryOwner":{
				"__typename":"User",
				"login":"octocat",
				"ownedRepositories":{
					"totalCount":1,
					"pageInfo":{"hasNextPage":false},
					"nodes":[{
						"owner":{"login":"octocat"},
						"name":"private",
						"nameWithOwner":"octocat/private",
						"url":"https://github.com/octocat/private",
						"stargazerCount":0,
						"forkCount":0,
						"issues":{"totalCount":0},
						"isFork":false,
						"isArchived":false,
						"updatedAt":"2026-07-30T00:00:00Z",
						"visibility":"PRIVATE",
						"languages":{"totalSize":0,"edges":[]}
					}]
				},
				"contributedRepositories":{
					"totalCount":0,
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				},
				"starredRepositories":{
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				},
				"forkedRepositories":{
					"totalCount":0,
					"pageInfo":{"hasNextPage":false},
					"nodes":[]
				},
				"contributionsCollection":{
					"commitContributionsByRepository":[],
					"pullRequestReviewContributionsByRepository":[]
				}
			},
			"authoredPullRequests":{"issueCount":0},
			"authoredIssues":{"issueCount":0}
		}
	}`)
	defer server.Close()

	_, err := newTestClient(t, server.URL, "").GetProfileAnalysis(
		context.Background(),
		"octocat",
		20,
		3,
	)
	if !port.IsGitHubError(err, port.GitHubErrorUpstream) {
		t.Fatalf("GetProfileAnalysis() error = %v", err)
	}
}

func TestGetProfileAnalysisValidatesLimitsBeforeUpstream(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		requests.Add(1)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, "")

	for _, limits := range []struct {
		repositories int
		manifests    int
	}{
		{repositories: 0, manifests: 3},
		{repositories: 21, manifests: 3},
		{repositories: 20, manifests: 0},
		{repositories: 20, manifests: 11},
	} {
		if _, err := client.GetProfileAnalysis(
			context.Background(),
			"octocat",
			limits.repositories,
			limits.manifests,
		); err == nil {
			t.Fatalf("limits %+v unexpectedly succeeded", limits)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("upstream requests = %d, want 0", requests.Load())
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

const completeProfileAnalysisFixture = `{
	"data":{
		"repositoryOwner":{
			"__typename":"User",
			"login":"octocat",
			"ownedRepositories":{
				"totalCount":2,
				"pageInfo":{"hasNextPage":true},
				"nodes":[{
					"databaseId":1,
					"owner":{"login":"octocat"},
					"name":"api",
					"nameWithOwner":"octocat/api",
					"description":"Public API",
					"url":"https://github.com/octocat/api",
					"stargazerCount":20,
					"forkCount":2,
					"issues":{"totalCount":3},
					"isFork":false,
					"isArchived":false,
					"defaultBranchRef":{"name":"main"},
					"primaryLanguage":{"name":"Go"},
					"updatedAt":"2026-07-29T00:00:00Z",
					"pushedAt":"2026-07-29T00:00:00Z",
					"visibility":"PUBLIC",
					"languages":{
						"totalSize":1000,
						"edges":[
							{"size":800,"node":{"name":"Go"}},
							{"size":200,"node":{"name":"TypeScript"}}
						]
					},
					"goManifest":{
						"__typename":"Blob",
						"byteSize":49,
						"isBinary":false,
						"text":"require github.com/gin-gonic/gin v1.12.0"
					}
				}]
			},
			"forkedRepositories":{
				"totalCount":1,
				"pageInfo":{"hasNextPage":false},
				"nodes":[{
					"databaseId":2,
					"owner":{"login":"octocat"},
					"name":"fork",
					"nameWithOwner":"octocat/fork",
					"url":"https://github.com/octocat/fork",
					"stargazerCount":0,
					"forkCount":0,
					"issues":{"totalCount":0},
					"isFork":true,
					"isArchived":false,
					"primaryLanguage":{"name":"Python"},
					"updatedAt":"2026-07-28T00:00:00Z",
					"visibility":"PUBLIC"
				}]
			},
			"contributedRepositories":{
				"totalCount":2,
				"pageInfo":{"hasNextPage":true},
				"nodes":[{
					"databaseId":3,
					"owner":{"login":"community"},
					"name":"project",
					"nameWithOwner":"community/project",
					"url":"https://github.com/community/project",
					"stargazerCount":100,
					"forkCount":10,
					"issues":{"totalCount":5},
					"isFork":false,
					"isArchived":false,
					"primaryLanguage":{"name":"Go"},
					"updatedAt":"2026-07-27T00:00:00Z",
					"visibility":"PUBLIC"
				}]
			},
			"starredRepositories":{
				"pageInfo":{"hasNextPage":false},
				"nodes":[
					{
						"databaseId":4,
						"owner":{"login":"community"},
						"name":"public-star",
						"nameWithOwner":"community/public-star",
						"url":"https://github.com/community/public-star",
						"stargazerCount":200,
						"forkCount":20,
						"issues":{"totalCount":10},
						"isFork":false,
						"isArchived":false,
						"primaryLanguage":{"name":"Rust"},
						"updatedAt":"2026-07-26T00:00:00Z",
						"visibility":"PUBLIC"
					},
					{
						"owner":{"login":"private"},
						"name":"hidden",
						"nameWithOwner":"private/hidden",
						"url":"https://github.com/private/hidden",
						"stargazerCount":0,
						"forkCount":0,
						"issues":{"totalCount":0},
						"isFork":false,
						"isArchived":false,
						"updatedAt":"2026-07-25T00:00:00Z",
						"visibility":"PRIVATE"
					}
				]
			},
			"contributionsCollection":{
				"commitContributionsByRepository":[
					{
						"repository":{"id":"R1","visibility":"PUBLIC"},
						"contributions":{"totalCount":7}
					},
					{
						"repository":{"id":"PRIVATE1","visibility":"PRIVATE"},
						"contributions":{"totalCount":100}
					}
				],
				"issueContributionsByRepository":[{
					"repository":{"id":"R2","visibility":"PUBLIC"},
					"contributions":{"totalCount":2}
				}],
				"pullRequestContributionsByRepository":[{
					"repository":{"id":"R2","visibility":"PUBLIC"},
					"contributions":{"totalCount":4}
				}],
				"pullRequestReviewContributionsByRepository":[{
					"repository":{"id":"R1","visibility":"PUBLIC"},
					"contributions":{"totalCount":3}
				}]
			}
		},
		"authoredPullRequests":{"issueCount":4},
		"authoredIssues":{"issueCount":2},
		"rateLimit":{
			"limit":5000,
			"remaining":4975,
			"resetAt":"2026-07-30T13:00:00Z"
		}
	}
}`
