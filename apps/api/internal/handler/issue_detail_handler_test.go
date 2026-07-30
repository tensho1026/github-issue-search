package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestIssueDetailHandlerReturnsCompleteNormalizedResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 30, 10, 0, 0, 0, time.UTC)
	recommendation := issue.Recommendation{
		Score: 92,
		Breakdown: issue.ScoreBreakdown{
			SkillMatch: issue.ScoreComponent{
				Name: "skill_match", Score: 30, Maximum: 30,
			},
		},
		SkillMatch: issue.SkillMatchAssessment{
			Percentage:  100,
			Matched:     1,
			Denominator: 1,
			Skills: []issue.SkillMatch{{
				Technology: "Go",
				Status:     issue.MatchMatched,
			}},
		},
		RepositorySignals: []issue.RepositorySignal{{
			Key:   issue.RepositoryREADME,
			State: issue.SignalPresent,
		}},
		Activity: issue.ActivityMetrics{
			LastMeaningfulUpdate: now,
			CI:                   issue.CIStateSuccess,
			Contributors:         issue.SummarizeCount(3, 5, 180, false),
			PullRequestMerge:     issue.SummarizeRatio(4, 5, 180, false),
			IssueResponse: issue.SummarizeDurations(
				[]time.Duration{2 * time.Hour},
				180,
				false,
			),
		},
		Claim: issue.ClaimEvidence{
			Claimed:    false,
			Confidence: issue.ConfidenceHigh,
		},
	}
	stub := &issueDetailRecommenderStub{
		output: usecase.RecommendIssueOutput{
			Item: issue.RankedIssue{
				Candidate: issue.Candidate{
					Repository: repository.Summary{
						ID:            7,
						Owner:         "acme",
						Name:          "rocket",
						FullName:      "acme/rocket",
						URL:           "https://github.com/acme/rocket",
						MainLanguage:  "Go",
						Stars:         100,
						DefaultBranch: "main",
						UpdatedAt:     now,
						PushedAt:      now,
					},
					Issue: issue.Summary{
						Number:      42,
						Title:       "Improve launch",
						Body:        "Detailed issue body",
						URL:         "https://github.com/acme/rocket/issues/42",
						State:       issue.StateOpen,
						Labels:      []string{"help wanted"},
						AuthorLogin: "maintainer",
						AuthorType:  issue.AuthorHuman,
						CreatedAt:   now.Add(-24 * time.Hour),
						UpdatedAt:   now,
					},
				},
				Analysis: issue.Analysis{
					Quality: issue.QualityAssessment{
						Score:      80,
						Confidence: issue.ConfidenceHigh,
					},
					Difficulty: issue.DifficultyAssessment{
						Level:      2,
						Label:      "Easy",
						Confidence: issue.ConfidenceHigh,
					},
					Effort: issue.EffortEstimate{
						Band:       issue.EffortTwoHours,
						Label:      "2 hours",
						Confidence: issue.ConfidenceMedium,
					},
					Confidence: issue.ConfidenceHigh,
				},
				Recommendation: recommendation,
			},
			RateLimit:  port.RateLimit{Known: true, Remaining: 37},
			Incomplete: false,
			CacheHit:   true,
		},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = []gin.Param{
		{Key: "owner", Value: "acme"},
		{Key: "repository", Value: "rocket"},
		{Key: "issueNumber", Value: "42"},
	}
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/issues/acme/rocket/42?skills=Go&skills=go",
		nil,
	)

	NewIssueDetailHandler(stub, response.NewResponder()).Get(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get(issueSearchCacheHeader) != issueSearchCacheHit {
		t.Fatalf("cache header = %q", recorder.Header().Get(issueSearchCacheHeader))
	}
	for _, fragment := range []string{
		`"fullName":"acme/rocket"`,
		`"body":"Detailed issue body"`,
		`"score":92`,
		`"percentage":100`,
		`"level":2`,
		`"band":"two_hours"`,
		`"key":"readme","state":"present"`,
		`"medianSeconds":7200`,
		`"incomplete":false`,
		`"rateLimitRemaining":37`,
	} {
		if !strings.Contains(recorder.Body.String(), fragment) {
			t.Errorf("body missing %s: %s", fragment, recorder.Body.String())
		}
	}
	if stub.input.Reference.CacheKey() !=
		"github:issue-detail:acme/rocket#42" ||
		len(stub.input.DesiredSkills) != 1 ||
		stub.input.DesiredSkills[0] != "Go" {
		t.Fatalf("usecase input = %+v", stub.input)
	}
}

func TestIssueDetailHandlerRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	tooManySkills := url.Values{}
	for index := 0; index <= maxIssueDetailSkills; index++ {
		tooManySkills.Add("skills", "Go")
	}
	tests := []struct {
		name       string
		owner      string
		repository string
		number     string
		query      string
	}{
		{name: "invalid owner", owner: "bad--owner", repository: "repo", number: "1"},
		{name: "invalid repository", owner: "owner", repository: "..", number: "1"},
		{name: "non-numeric issue", owner: "owner", repository: "repo", number: "one"},
		{name: "zero issue", owner: "owner", repository: "repo", number: "0"},
		{name: "unknown query", owner: "owner", repository: "repo", number: "1", query: "typo=1"},
		{name: "empty skill", owner: "owner", repository: "repo", number: "1", query: "skills="},
		{name: "too many skills", owner: "owner", repository: "repo", number: "1", query: tooManySkills.Encode()},
		{name: "control skill", owner: "owner", repository: "repo", number: "1", query: "skills=Go%0ATypeScript"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			stub := &issueDetailRecommenderStub{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Params = []gin.Param{
				{Key: "owner", Value: test.owner},
				{Key: "repository", Value: test.repository},
				{Key: "issueNumber", Value: test.number},
			}
			target := "/api/issues/detail"
			if test.query != "" {
				target += "?" + test.query
			}
			ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)

			NewIssueDetailHandler(stub, response.NewResponder()).Get(ctx)

			if recorder.Code != http.StatusBadRequest ||
				!strings.Contains(
					recorder.Body.String(),
					`"code":"INVALID_REQUEST"`,
				) {
				t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
			}
			if stub.called {
				t.Fatal("usecase was called for an invalid request")
			}
		})
	}
}

func TestIssueDetailHandlerWritesUsecaseError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	stub := &issueDetailRecommenderStub{err: apperror.New(
		apperror.CodeNotFound,
		"GitHub issue was not found",
		http.StatusNotFound,
	)}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = []gin.Param{
		{Key: "owner", Value: "acme"},
		{Key: "repository", Value: "rocket"},
		{Key: "issueNumber", Value: "42"},
	}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/issues/acme/rocket/42", nil)

	NewIssueDetailHandler(stub, response.NewResponder()).Get(ctx)

	if recorder.Code != http.StatusNotFound ||
		!strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

type issueDetailRecommenderStub struct {
	output usecase.RecommendIssueOutput
	err    error
	input  usecase.RecommendIssueInput
	called bool
}

func (stub *issueDetailRecommenderStub) Execute(
	_ context.Context,
	input usecase.RecommendIssueInput,
) (usecase.RecommendIssueOutput, error) {
	stub.called = true
	stub.input = input
	return stub.output, stub.err
}

func (*issueDetailRecommenderStub) EvaluateCandidate(
	candidate issue.Candidate,
	_ []string,
) issue.RankedIssue {
	return issue.RankedIssue{Candidate: candidate}
}

var _ usecase.IssueRecommender = (*issueDetailRecommenderStub)(nil)
