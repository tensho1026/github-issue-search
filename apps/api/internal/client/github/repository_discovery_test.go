package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

const validRESTRepositoryItem = `{
	"id":123,
	"owner":{"login":"example"},
	"name":"typed-service",
	"full_name":"example/typed-service",
	"description":"A typed React and Go developer tool",
	"html_url":"https://github.com/example/typed-service",
	"stargazers_count":420,
	"forks_count":32,
	"watchers_count":18,
	"open_issues_count":14,
	"fork":false,
	"archived":false,
	"has_issues":true,
	"has_discussions":true,
	"language":"TypeScript",
	"license":{"name":"MIT License","spdx_id":"MIT"},
	"topics":["react","developer-tools"],
	"default_branch":"main",
	"updated_at":"2026-07-30T09:00:00Z",
	"pushed_at":"2026-07-30T08:00:00Z"
}`

func TestBuildRepositorySearchQueryUsesBoundedQualifiers(t *testing.T) {
	t.Parallel()

	minimumStars := 100
	minimumForks := 2
	updatedWithinDays := 30
	forkPolicy := "include"
	criteria := repositoryDiscoveryCriteria(t, repository.DiscoveryCriteriaOptions{
		Languages:         []string{"TypeScript", "Go"},
		Licenses:          []string{"MIT", "Apache-2.0"},
		MinimumStars:      &minimumStars,
		MinimumForks:      &minimumForks,
		UpdatedWithinDays: &updatedWithinDays,
		ForkPolicy:        &forkPolicy,
	})

	query, err := buildRepositorySearchQuery(
		criteria,
		time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("buildRepositorySearchQuery() error = %v", err)
	}
	want := `is:public archived:false fork:true stars:>=100 forks:>=2 ` +
		`pushed:>=2026-06-30 ` +
		`language:"Go" language:"TypeScript" ` +
		`license:apache-2.0 license:mit`
	if query != want {
		t.Fatalf("query =\n%s\nwant\n%s", query, want)
	}
	if strings.Contains(query, " OR ") {
		t.Fatalf("query contains unsupported qualifier OR syntax: %q", query)
	}
}

func TestSearchRepositoriesGetsRESTWindowAndNormalizesCandidate(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if request.Method != http.MethodGet ||
			request.URL.Path != "/search/repositories" {
			t.Errorf("request = %s %s", request.Method, request.URL.String())
		}
		query := request.URL.Query()
		if !strings.Contains(query.Get("q"), "stars:>=10") ||
			query.Get("per_page") != "50" ||
			query.Get("page") != "1" ||
			query.Get("sort") != "updated" ||
			query.Get("order") != "desc" {
			t.Errorf("search query = %q", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-RateLimit-Limit", "30")
		writer.Header().Set("X-RateLimit-Remaining", "29")
		writer.Header().Set("X-RateLimit-Reset", "1785384000")
		_, _ = io.WriteString(writer, `{
			"total_count":200,
			"incomplete_results":false,
			"items":[`+validRESTRepositoryItem+`]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	client.now = func() time.Time {
		return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	}
	result, err := client.SearchRepositories(
		context.Background(),
		repositoryDiscoveryCriteria(t, repository.DiscoveryCriteriaOptions{}),
		50,
	)
	if err != nil {
		t.Fatalf("SearchRepositories() error = %v", err)
	}
	if requests.Load() != 1 ||
		len(result.Candidates) != 1 ||
		result.TotalCount != 200 ||
		!result.IncompleteResults ||
		result.RateLimit.Remaining != 29 {
		t.Fatalf("SearchRepositories() result = %+v", result)
	}
	candidate := result.Candidates[0]
	if candidate.Repository.ID != 123 ||
		candidate.Repository.FullName != "example/typed-service" ||
		candidate.Repository.DefaultBranch != "main" ||
		candidate.Repository.MainLanguage != "TypeScript" ||
		candidate.License != "MIT" ||
		!candidate.LicenseKnown ||
		len(candidate.Topics) != 2 ||
		candidate.Topics[0] != "developer-tools" {
		t.Fatalf("candidate = %+v", candidate)
	}
}

func TestSearchRepositoriesReportsRESTIncompleteResults(t *testing.T) {
	t.Parallel()

	server := jsonServer(`{
		"total_count":2,
		"incomplete_results":true,
		"items":[` + validRESTRepositoryItem + `]
	}`)
	defer server.Close()

	result, err := newTestClient(t, server.URL, "token").SearchRepositories(
		context.Background(),
		repositoryDiscoveryCriteria(t, repository.DiscoveryCriteriaOptions{}),
		2,
	)
	if err != nil {
		t.Fatalf("SearchRepositories() error = %v", err)
	}
	if len(result.Candidates) != 1 || !result.IncompleteResults {
		t.Fatalf("result = %+v", result)
	}
}

func TestEnrichRepositoriesUsesOneBoundedBatchAndReportsPartialData(
	t *testing.T,
) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		for _, required := range []string{
			"repo0: repository",
			"repo1: repository",
			"goodFirstIssues: issues",
			"helpWantedIssues: issues",
			"codeOfConduct",
			"securityPolicyUrl",
			`"main:README.ja.md"`,
			`"main:.github/CONTRIBUTING.md"`,
			"rateLimit",
		} {
			if !strings.Contains(payload.Query, required) {
				t.Errorf("query does not contain %q:\n%s", required, payload.Query)
			}
		}
		japanese := strings.Repeat("日本語の説明", 20)
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": map[string]any{
				"repo0": map[string]any{
					"nameWithOwner": "example/typed-service",
					"goodFirstIssues": map[string]any{
						"totalCount": 4,
					},
					"helpWantedIssues": map[string]any{
						"totalCount": 6,
					},
					"codeOfConduct": map[string]any{
						"key": "contributor_covenant",
					},
					"securityPolicyUrl": "https://github.com/example/" +
						"typed-service/security/policy",
					"readmeMarkdown": map[string]any{
						"byteSize": 12,
						"isBinary": false,
						"text":     "English docs",
					},
					"readmeJapanese": map[string]any{
						"byteSize": len(japanese),
						"isBinary": false,
						"text":     japanese,
					},
					"contributingGitHub": map[string]any{
						"byteSize": 10,
						"isBinary": false,
					},
				},
				"repo1": nil,
				"rateLimit": map[string]any{
					"limit":     5000,
					"remaining": 58,
					"resetAt":   "2026-07-30T13:00:00Z",
				},
			},
			"errors": []map[string]any{{
				"type":    "NOT_FOUND",
				"message": "one repository disappeared",
			}},
		})
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "token").EnrichRepositories(
		context.Background(),
		[]repository.Summary{
			{
				Owner:         "example",
				Name:          "typed-service",
				FullName:      "example/typed-service",
				DefaultBranch: "main",
			},
			{
				Owner:         "example",
				Name:          "gone",
				FullName:      "example/gone",
				DefaultBranch: "main",
			},
		},
	)
	if err != nil {
		t.Fatalf("EnrichRepositories() error = %v", err)
	}
	if requests.Load() != 1 ||
		!result.IncompleteResults ||
		result.RateLimit.Remaining != 58 ||
		len(result.Items) != 2 {
		t.Fatalf("EnrichRepositories() result = %+v", result)
	}
	enrichment := result.Items["example/typed-service"]
	if !enrichment.Available ||
		!enrichment.READMEAvailable ||
		!enrichment.READMEContentAvailable ||
		!enrichment.ContributingAvailable ||
		enrichment.GoodFirstIssues != 4 ||
		enrichment.HelpWantedIssues != 6 ||
		!enrichment.HasCodeOfConduct ||
		!enrichment.HasSecurityPolicy ||
		!strings.Contains(enrichment.READMEText, "日本語") {
		t.Fatalf("enrichment = %+v", enrichment)
	}
	if result.Items["example/gone"].Available {
		t.Fatalf("missing repository = %+v", result.Items["example/gone"])
	}
}

func TestRepositoryDiscoveryRejectsInvalidBoundsBeforeRequest(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	client := newTestClient(t, "https://api.github.example", "")
	client.httpClient = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("must not be called")
	})
	criteria := repositoryDiscoveryCriteria(
		t,
		repository.DiscoveryCriteriaOptions{},
	)
	for _, limit := range []int{
		0,
		repository.MaximumDiscoveryCandidateResults + 1,
	} {
		if _, err := client.SearchRepositories(
			context.Background(),
			criteria,
			limit,
		); err == nil {
			t.Fatalf("SearchRepositories(limit=%d) error = nil", limit)
		}
	}
	oversized := make(
		[]repository.Summary,
		repository.MaximumDiscoveryEnrichmentResults+1,
	)
	if _, err := client.EnrichRepositories(
		context.Background(),
		oversized,
	); err == nil {
		t.Fatal("EnrichRepositories(oversized) error = nil")
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func repositoryDiscoveryCriteria(
	t *testing.T,
	options repository.DiscoveryCriteriaOptions,
) repository.DiscoveryCriteria {
	t.Helper()
	criteria, err := repository.NewDiscoveryCriteria(options)
	if err != nil {
		t.Fatalf("NewDiscoveryCriteria() error = %v", err)
	}
	return criteria
}
