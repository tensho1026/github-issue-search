package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestGitHubProfileAnalysisHandlerGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	analyze := &analyzeGitHubProfileStub{
		output: usecase.AnalyzeGitHubProfileOutput{
			Analysis: profile.Analysis{
				Username: "octocat",
				Languages: []profile.LanguageShare{
					{Name: "Go", Percentage: 60},
					{Name: "TypeScript", Percentage: 40},
				},
				Frameworks: []string{"Gin", "React"},
				RecentTechnologies: []profile.RecentTechnology{{
					Name:            "Go",
					Kind:            profile.TechnologyLanguage,
					LastUsedAt:      now.Add(-time.Hour),
					RepositoryCount: 2,
					RepositorySources: []profile.RepositorySource{
						profile.RepositoryContributed,
						profile.RepositoryOwned,
					},
					Confidence: profile.ConfidenceMedium,
				}},
				Contributions: profile.ContributionAnalysis{
					WindowDays: profile.AnalysisWindowDays,
					Commits: profile.CountMetric{
						Value:  10,
						Status: profile.EvidenceSampled,
					},
					IssuesOpened: profile.CountMetric{
						Value:  2,
						Status: profile.EvidenceExact,
					},
					PullRequestsOpened: profile.CountMetric{
						Value:  5,
						Status: profile.EvidenceExact,
					},
					PullRequestReviews: profile.CountMetric{
						Value:  3,
						Status: profile.EvidenceSampled,
					},
					RepositoriesTouched: profile.CountMetric{
						Value:  2,
						Status: profile.EvidenceExact,
					},
				},
				OSSExperience: profile.OSSExperience{
					Level:      "active",
					Confidence: profile.ConfidenceHigh,
					PublicOnly: true,
					Evidence: []profile.TechnologyEvidence{{
						Kind:   "authored_pull_requests",
						Value:  5,
						Status: profile.EvidenceExact,
					}},
				},
				RepositoryEvidence: profile.RepositoryEvidence{
					Owned: profile.RepositorySample{
						Status:         profile.EvidenceSampled,
						Observed:       2,
						Total:          profileTotal(5),
						Limit:          20,
						ActiveInWindow: 2,
						PrimaryTechnologies: []profile.LanguageShare{{
							Name:       "Go",
							Percentage: 60,
						}},
					},
					Contributed: profile.RepositorySample{
						Status:         profile.EvidenceExact,
						Observed:       2,
						Total:          profileTotal(2),
						Limit:          20,
						ActiveInWindow: 2,
					},
					Starred: profile.RepositorySample{
						Status:   profile.EvidenceSampled,
						Observed: 1,
						Limit:    20,
					},
					Forked: profile.RepositorySample{
						Status:   profile.EvidenceExact,
						Observed: 0,
						Total:    profileTotal(0),
						Limit:    20,
					},
				},
				Proficiency: []profile.TechnologyProficiency{{
					Name:       "Go",
					Kind:       profile.TechnologyLanguage,
					Level:      4,
					Label:      "advanced",
					Score:      65,
					Confidence: profile.ConfidenceMedium,
					Evidence: []profile.TechnologyEvidence{{
						Kind:   "owned_repositories",
						Value:  2,
						Status: profile.EvidenceSampled,
					}},
				}},
				Window: profile.AnalysisWindow{
					From:       now.AddDate(0, 0, -profile.AnalysisWindowDays),
					To:         now,
					Days:       profile.AnalysisWindowDays,
					PublicOnly: true,
				},
				RepositoriesAnalyzed: 2,
				Warnings: []profile.Warning{{
					Code:       "manifest_data_unavailable",
					Message:    "A framework manifest could not be retrieved",
					Repository: "octocat/private",
				}},
			},
			RateLimit: port.RateLimit{Known: true, Remaining: 37},
		},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "username", Value: "octocat"}}
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/github/users/octocat/profile-analysis",
		nil,
	)

	NewGitHubProfileAnalysisHandler(
		analyze,
		response.NewResponder(),
	).Get(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	for _, value := range []string{
		`"username":"octocat"`,
		`"name":"Go","percentage":60`,
		`"frameworks":["Gin","React"]`,
		`"recentTechnologies":[{"name":"Go","kind":"language"`,
		`"repositorySources":["contributed","owned"]`,
		`"commits":{"value":10,"status":"sampled"}`,
		`"pullRequestsOpened":{"value":5,"status":"exact"}`,
		`"ossExperience":{"level":"active","confidence":"high","publicOnly":true`,
		`"owned":{"status":"sampled","observed":2,"total":5`,
		`"starred":{"status":"sampled","observed":1,"total":null`,
		`"proficiency":[{"name":"Go","kind":"language","level":4,"label":"advanced"`,
		`"analysisWindow":{"from":"2025-07-30T12:00:00Z","to":"2026-07-30T12:00:00Z","days":365,"publicOnly":true}`,
		`"repositoriesAnalyzed":2`,
		`"code":"manifest_data_unavailable"`,
		`"repository":"octocat/private"`,
		`"rateLimitRemaining":37`,
	} {
		if !strings.Contains(recorder.Body.String(), value) {
			t.Errorf("body missing %s: %s", value, recorder.Body.String())
		}
	}
	if analyze.username != "octocat" {
		t.Fatalf("usecase username = %q", analyze.username)
	}
}

func TestGitHubProfileAnalysisHandlerRejectsInvalidUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	analyze := &analyzeGitHubProfileStub{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "username", Value: "invalid--username"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewGitHubProfileAnalysisHandler(
		analyze,
		response.NewResponder(),
	).Get(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if analyze.called {
		t.Fatal("usecase was called for invalid input")
	}
	if !strings.Contains(recorder.Body.String(), `"code":"INVALID_REQUEST"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestGitHubProfileAnalysisHandlerWritesUsecaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	analyze := &analyzeGitHubProfileStub{
		err: apperror.New(
			apperror.CodeRateLimit,
			"GitHub API rate limit was exceeded",
			http.StatusTooManyRequests,
		),
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "username", Value: "octocat"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewGitHubProfileAnalysisHandler(
		analyze,
		response.NewResponder(),
	).Get(ctx)

	if recorder.Code != http.StatusTooManyRequests ||
		!strings.Contains(recorder.Body.String(), "GITHUB_RATE_LIMIT_EXCEEDED") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

type analyzeGitHubProfileStub struct {
	output   usecase.AnalyzeGitHubProfileOutput
	err      error
	username user.Username
	called   bool
}

func (stub *analyzeGitHubProfileStub) Execute(
	_ context.Context,
	username user.Username,
) (usecase.AnalyzeGitHubProfileOutput, error) {
	stub.called = true
	stub.username = username
	return stub.output, stub.err
}

var _ usecase.AnalyzeGitHubProfile = (*analyzeGitHubProfileStub)(nil)

func profileTotal(value int) *int {
	return &value
}
