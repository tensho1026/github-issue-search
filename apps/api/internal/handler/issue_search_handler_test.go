package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestIssueSearchHandlerReturnsNormalizedSearchResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 30, 10, 0, 0, 0, time.UTC)
	search := &searchIssuesStub{output: usecase.SearchIssuesOutput{
		Items: []issue.RankedIssue{{
			Candidate: issue.Candidate{
				Repository: repository.Summary{
					Owner:        "example",
					Name:         "api",
					FullName:     "example/api",
					Description:  "A Gin API",
					URL:          "https://github.com/example/api",
					Stars:        120,
					MainLanguage: "Go",
					UpdatedAt:    now,
				},
				Issue: issue.Summary{
					Number:    123,
					Title:     "Add validation",
					URL:       "https://github.com/example/api/issues/123",
					Labels:    []string{"good first issue"},
					Comments:  2,
					CreatedAt: now.Add(-time.Hour),
					UpdatedAt: now,
				},
			},
			Analysis: issue.Analysis{
				Difficulty: issue.DifficultyAssessment{
					Level:      1,
					Label:      "Very easy",
					Confidence: issue.ConfidenceHigh,
				},
				Effort: issue.EffortEstimate{
					Band:       issue.EffortTwoHours,
					Label:      "About two hours",
					Confidence: issue.ConfidenceMedium,
				},
			},
			Recommendation: issue.Recommendation{
				Score: 88,
				SkillMatch: issue.SkillMatchAssessment{
					Percentage:  100,
					Matched:     1,
					Denominator: 1,
				},
			},
		}},
		Pagination: usecase.SearchIssuesPagination{
			Page:       2,
			PerPage:    1,
			Total:      2,
			TotalPages: 2,
		},
		ExclusionCounts: map[issue.ExclusionReason]int{
			issue.ExclusionStale:           3,
			issue.ExclusionAlreadyAssigned: 1,
			issue.ExclusionBotGenerated:    2,
		},
		CandidatesChecked:    8,
		UpstreamTotal:        150,
		EnrichmentAttempted:  2,
		EnrichmentFailed:     1,
		GitHubIncomplete:     true,
		EnrichmentIncomplete: true,
		RateLimit:            port.RateLimit{Known: true, Remaining: 29},
		CacheHit:             true,
	}}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/issues/search?page=2&perPage=1",
		strings.NewReader(`{
			"username":"octocat",
			"languages":["Go"],
			"frameworks":["Gin"],
			"labels":["good first issue"],
			"minimumStars":0,
			"maximumDifficulty":3,
			"maximumEffort":"two_hours",
			"updatedWithinDays":30,
			"includeDocumentation":false,
			"includeEnglish":true,
			"excludeArchived":true
		}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json; charset=utf-8")

	NewIssueSearchHandler(search, response.NewResponder()).Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get(issueSearchCacheHeader) != issueSearchCacheHit {
		t.Fatalf("cache header = %q", recorder.Header().Get(issueSearchCacheHeader))
	}
	for _, fragment := range []string{
		`"fullName":"example/api"`,
		`"estimatedDifficulty":1`,
		`"difficulty":{"level":1,"label":"Very easy","confidence":"high"}`,
		`"effort":{"band":"two_hours","label":"About two hours","confidence":"medium"}`,
		`"score":88`,
		`"percentage":100`,
		`"page":2`,
		`"totalPages":2`,
		`"candidatesChecked":8`,
		`"upstreamTotal":150`,
		`"enrichmentAttempted":2`,
		`"enrichmentFailed":1`,
		`"reason":"already_assigned","count":1`,
		`"reason":"bot_generated","count":2`,
		`"reason":"stale","count":3`,
		`"code":"github_search_incomplete"`,
		`"code":"issue_enrichment_incomplete"`,
		`"rateLimitRemaining":29`,
	} {
		if !strings.Contains(recorder.Body.String(), fragment) {
			t.Errorf("body missing %s: %s", fragment, recorder.Body.String())
		}
	}
	if search.input.Pagination.Page != 2 ||
		search.input.Pagination.PerPage != 1 ||
		search.input.Criteria.Username() != "octocat" ||
		search.input.Criteria.MinimumStars() != 0 {
		t.Fatalf("usecase input = %+v", search.input)
	}
	if maximumEffort, configured := search.input.Criteria.MaximumEffort(); !configured || maximumEffort != issue.EffortTwoHours {
		t.Fatalf("maximum effort = %q, %t", maximumEffort, configured)
	}
}

func TestIssueSearchHandlerAppliesDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	search := &searchIssuesStub{output: usecase.SearchIssuesOutput{
		ExclusionCounts: make(map[issue.ExclusionReason]int),
	}}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/issues/search",
		strings.NewReader(`{"username":"octocat"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewIssueSearchHandler(search, response.NewResponder()).Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if search.input.Pagination.Page != issue.DefaultPage ||
		search.input.Pagination.PerPage != issue.DefaultPerPage ||
		search.input.Criteria.MinimumStars() != issue.DefaultMinimumStars ||
		search.input.Criteria.MaximumDifficulty().Int() !=
			issue.DefaultMaximumDifficulty ||
		recorder.Header().Get(issueSearchCacheHeader) != issueSearchCacheMiss ||
		!strings.Contains(recorder.Body.String(), `"items":[]`) ||
		!strings.Contains(recorder.Body.String(), `"excludedByReason":[]`) ||
		!strings.Contains(recorder.Body.String(), `"warnings":[]`) {
		t.Fatalf(
			"input = %+v, headers = %v, body = %s",
			search.input,
			recorder.Header(),
			recorder.Body.String(),
		)
	}
}

func TestIssueSearchHandlerRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		contentType string
		body        string
	}{
		{
			name:        "invalid maximum effort",
			target:      "/api/issues/search",
			contentType: "application/json",
			body:        `{"username":"octocat","maximumEffort":"weekend"}`,
		},
		{
			name:   "missing content type",
			target: "/api/issues/search",
			body:   `{"username":"octocat"}`,
		},
		{
			name:        "unknown JSON field",
			target:      "/api/issues/search",
			contentType: "application/json",
			body:        `{"username":"octocat","unexpected":true}`,
		},
		{
			name:        "trailing JSON",
			target:      "/api/issues/search",
			contentType: "application/json",
			body:        `{"username":"octocat"} {}`,
		},
		{
			name:        "invalid domain criteria",
			target:      "/api/issues/search",
			contentType: "application/json",
			body:        `{"username":"invalid--user"}`,
		},
		{
			name:        "duplicate page",
			target:      "/api/issues/search?page=1&page=2",
			contentType: "application/json",
			body:        `{"username":"octocat"}`,
		},
		{
			name:        "oversized per page",
			target:      "/api/issues/search?perPage=51",
			contentType: "application/json",
			body:        `{"username":"octocat"}`,
		},
		{
			name:        "unknown query",
			target:      "/api/issues/search?typo=1",
			contentType: "application/json",
			body:        `{"username":"octocat"}`,
		},
		{
			name:        "oversized body",
			target:      "/api/issues/search",
			contentType: "application/json",
			body: `{"username":"octocat","labels":["` +
				strings.Repeat("a", maxIssueSearchRequestBytes) + `"]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			search := &searchIssuesStub{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(
				http.MethodPost,
				test.target,
				strings.NewReader(test.body),
			)
			if test.contentType != "" {
				ctx.Request.Header.Set("Content-Type", test.contentType)
			}

			NewIssueSearchHandler(search, response.NewResponder()).Search(ctx)

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

func TestIssueSearchHandlerWritesUsecaseErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	search := &searchIssuesStub{err: apperror.New(
		apperror.CodeRateLimit,
		"GitHub API rate limit was exceeded",
		http.StatusTooManyRequests,
	)}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/issues/search",
		strings.NewReader(`{"username":"octocat"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewIssueSearchHandler(search, response.NewResponder()).Search(ctx)

	if recorder.Code != http.StatusTooManyRequests ||
		!strings.Contains(recorder.Body.String(), "GITHUB_RATE_LIMIT_EXCEEDED") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

type searchIssuesStub struct {
	output usecase.SearchIssuesOutput
	err    error
	input  usecase.SearchIssuesInput
	called bool
}

func (stub *searchIssuesStub) Execute(
	_ context.Context,
	input usecase.SearchIssuesInput,
) (usecase.SearchIssuesOutput, error) {
	stub.called = true
	stub.input = input
	return stub.output, stub.err
}

var _ usecase.SearchIssues = (*searchIssuesStub)(nil)
