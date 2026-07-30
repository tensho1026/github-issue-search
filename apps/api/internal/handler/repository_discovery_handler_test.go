package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestRepositoryDiscoveryHandlerReturnsEvidenceOrientedResponse(
	t *testing.T,
) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	search := &searchRepositoriesStub{
		output: usecase.SearchRepositoriesOutput{
			Items: []repository.DiscoveryResult{{
				Repository: repository.Summary{
					Owner:        "example",
					Name:         "typed-service",
					FullName:     "example/typed-service",
					Description:  "A typed service",
					URL:          "https://github.com/example/typed-service",
					MainLanguage: "TypeScript",
					Stars:        420,
					Forks:        32,
					OpenIssues:   14,
					UpdatedAt:    now,
					PushedAt:     now.Add(-time.Hour),
				},
				Topics:           []string{"developer-tools", "react"},
				Technologies:     []string{"React"},
				License:          "MIT",
				LicenseName:      "MIT License",
				LicenseKnown:     true,
				Watchers:         18,
				GoodFirstIssues:  4,
				HelpWantedIssues: 6,
				HasIssuesEnabled: true,
				HasDiscussions:   true,
				Category:         repository.CategoryTooling,
				Documentation: repository.DocumentationSignals{
					READMEAvailable:       true,
					ContributingAvailable: true,
					CodeOfConduct:         true,
					SecurityPolicy:        true,
					Status:                repository.EvidenceSampled,
					JapaneseREADME: repository.JapaneseREADMEEvidence{
						Detected:      true,
						Status:        repository.EvidenceSampled,
						Confidence:    repository.ConfidenceMedium,
						JapaneseRunes: 80,
						LetterRunes:   200,
						SampledBytes:  4096,
					},
				},
				Difficulty: repository.PreliminaryDifficulty{
					Level:   1,
					Label:   "very_low",
					Reasons: []string{"contributing_guide_available"},
				},
				Readiness: repository.ContributionReadiness{
					Score:   88,
					Band:    repository.ReadinessReady,
					Reasons: []string{"readme_available"},
				},
				Warnings: []repository.DiscoveryWarning{
					repository.WarningREADMEContentSampled,
				},
			}},
			Pagination: usecase.SearchRepositoriesPagination{
				Page:       2,
				PerPage:    1,
				Total:      2,
				TotalPages: 2,
			},
			CandidatesChecked:    50,
			UpstreamTotal:        200,
			EnrichmentAttempted:  20,
			EnrichmentFailed:     1,
			GitHubIncomplete:     true,
			EnrichmentIncomplete: true,
			Warnings: []usecase.RepositoryDiscoveryWarning{
				usecase.RepositoryDiscoveryWarningGitHubIncomplete,
				usecase.RepositoryDiscoveryWarningEnrichmentIncomplete,
			},
			RateLimit: port.RateLimit{Known: true, Remaining: 58},
			CacheHit:  true,
		},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/repositories/search?page=2&perPage=1",
		strings.NewReader(`{
			"languages":["TypeScript"],
			"technologies":["React"],
			"licenses":["MIT"],
			"categories":["tooling"],
			"minimumStars":100,
			"minimumForks":2,
			"minimumOpenIssues":1,
			"maximumOpenIssues":100,
			"updatedWithinDays":30,
			"maximumDifficulty":3,
			"minimumReadiness":50,
			"hasJapaneseReadme":true,
			"forkPolicy":"include",
			"excludeArchived":true
		}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json; charset=utf-8")

	NewRepositoryDiscoveryHandler(
		search,
		response.NewResponder(),
	).Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get(issueSearchCacheHeader) != issueSearchCacheHit {
		t.Fatalf("cache header = %q", recorder.Header().Get(issueSearchCacheHeader))
	}
	for _, fragment := range []string{
		`"fullName":"example/typed-service"`,
		`"topics":["developer-tools","react"]`,
		`"technologies":["React"]`,
		`"category":"tooling"`,
		`"spdxId":"MIT"`,
		`"stars":420`,
		`"score":88`,
		`"band":"ready"`,
		`"japaneseReadme":{"detected":true,"status":"sampled","confidence":"medium"`,
		`"difficulty":{"level":1,"label":"very_low"`,
		`"code":"readme_content_sampled"`,
		`"code":"github_results_incomplete"`,
		`"code":"repository_enrichment_incomplete"`,
		`"candidatesChecked":50`,
		`"enrichmentAttempted":20`,
		`"rateLimitRemaining":58`,
	} {
		if !strings.Contains(recorder.Body.String(), fragment) {
			t.Errorf("body missing %s: %s", fragment, recorder.Body.String())
		}
	}
	if search.input.Pagination.Page != 2 ||
		search.input.Pagination.PerPage != 1 ||
		search.input.Criteria.MinimumStars() != 100 ||
		search.input.Criteria.ForkPolicy() != repository.ForkPolicyInclude {
		t.Fatalf("usecase input = %+v", search.input)
	}
	if japanese, configured := search.input.Criteria.HasJapaneseREADME(); !configured ||
		!japanese {
		t.Fatalf("Japanese README filter = %t, %t", japanese, configured)
	}
}

func TestRepositoryDiscoveryHandlerAppliesDefaults(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	search := &searchRepositoriesStub{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/repositories/search",
		strings.NewReader(`{}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewRepositoryDiscoveryHandler(
		search,
		response.NewResponder(),
	).Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if search.input.Pagination.Page != repository.DefaultDiscoveryPage ||
		search.input.Pagination.PerPage != repository.DefaultDiscoveryPerPage ||
		search.input.Criteria.MinimumStars() !=
			repository.DefaultDiscoveryMinimumStars ||
		search.input.Criteria.ForkPolicy() != repository.ForkPolicyExclude ||
		!search.input.Criteria.ExcludesArchived() ||
		!strings.Contains(recorder.Body.String(), `"items":[]`) ||
		!strings.Contains(recorder.Body.String(), `"warnings":[]`) {
		t.Fatalf(
			"input = %+v, body = %s",
			search.input,
			recorder.Body.String(),
		)
	}
}

func TestRepositoryDiscoveryHandlerRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		target      string
		contentType string
		body        string
	}{
		{
			name:        "unsupported SPDX",
			target:      "/api/repositories/search",
			contentType: "application/json",
			body:        `{"licenses":["WTFPL"]}`,
		},
		{
			name:        "inverted issue range",
			target:      "/api/repositories/search",
			contentType: "application/json",
			body:        `{"minimumOpenIssues":10,"maximumOpenIssues":2}`,
		},
		{
			name:        "unsafe technology",
			target:      "/api/repositories/search",
			contentType: "application/json",
			body:        `{"technologies":["React\" stars:>0"]}`,
		},
		{
			name:   "missing content type",
			target: "/api/repositories/search",
			body:   `{}`,
		},
		{
			name:        "unknown field",
			target:      "/api/repositories/search",
			contentType: "application/json",
			body:        `{"unexpected":true}`,
		},
		{
			name:        "trailing JSON",
			target:      "/api/repositories/search",
			contentType: "application/json",
			body:        `{} {}`,
		},
		{
			name:        "duplicate page",
			target:      "/api/repositories/search?page=1&page=2",
			contentType: "application/json",
			body:        `{}`,
		},
		{
			name:        "oversized page",
			target:      "/api/repositories/search?perPage=51",
			contentType: "application/json",
			body:        `{}`,
		},
		{
			name:        "unknown query",
			target:      "/api/repositories/search?sort=stars",
			contentType: "application/json",
			body:        `{}`,
		},
		{
			name:        "oversized body",
			target:      "/api/repositories/search",
			contentType: "application/json",
			body: `{"technologies":["` +
				strings.Repeat("a", maxRepositoryDiscoveryRequestBytes) +
				`"]}`,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			search := &searchRepositoriesStub{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(
				http.MethodPost,
				testCase.target,
				strings.NewReader(testCase.body),
			)
			if testCase.contentType != "" {
				ctx.Request.Header.Set("Content-Type", testCase.contentType)
			}

			NewRepositoryDiscoveryHandler(
				search,
				response.NewResponder(),
			).Search(ctx)

			if recorder.Code != http.StatusBadRequest ||
				!strings.Contains(
					recorder.Body.String(),
					`"code":"INVALID_REQUEST"`,
				) {
				t.Fatalf(
					"response = %d %s",
					recorder.Code,
					recorder.Body.String(),
				)
			}
			if search.called {
				t.Fatal("usecase was called for an invalid request")
			}
		})
	}
}

func TestRepositoryDiscoveryHandlerWritesUsecaseErrors(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	search := &searchRepositoriesStub{
		err: apperror.New(
			apperror.CodeRateLimit,
			"GitHub API rate limit was exceeded",
			http.StatusTooManyRequests,
		),
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/repositories/search",
		strings.NewReader(`{}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewRepositoryDiscoveryHandler(
		search,
		response.NewResponder(),
	).Search(ctx)

	if recorder.Code != http.StatusTooManyRequests ||
		!strings.Contains(
			recorder.Body.String(),
			"GITHUB_RATE_LIMIT_EXCEEDED",
		) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

type searchRepositoriesStub struct {
	output usecase.SearchRepositoriesOutput
	err    error
	input  usecase.SearchRepositoriesInput
	called bool
}

func (stub *searchRepositoriesStub) Execute(
	_ context.Context,
	input usecase.SearchRepositoriesInput,
) (usecase.SearchRepositoriesOutput, error) {
	stub.called = true
	stub.input = input
	return stub.output, stub.err
}
