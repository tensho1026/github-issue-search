package router

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestHealthRouteUsesStandardEnvelopeAndHeaders(t *testing.T) {
	router := newTestRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("X-Request-ID", "req_health")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("X-Request-ID"); got != "req_health" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := recorder.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	var body struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Meta response.Meta `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Status != "ok" || body.Meta.RequestID != "req_health" {
		t.Fatalf("body = %+v", body)
	}
}

func TestUnknownRouteUsesSafeErrorEnvelope(t *testing.T) {
	router := newTestRouter(t)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/unknown", nil),
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestProfileAnalysisRouteUsesStandardEnvelope(t *testing.T) {
	router := newTestRouter(t)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/github/users/octocat/profile-analysis",
		nil,
	)
	request.Header.Set("X-Request-ID", "req_profile")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"username":"octocat"`) ||
		!strings.Contains(recorder.Body.String(), `"languages":[]`) ||
		!strings.Contains(recorder.Body.String(), `"frameworks":[]`) ||
		!strings.Contains(recorder.Body.String(), `"warnings":[]`) ||
		!strings.Contains(recorder.Body.String(), `"rateLimitRemaining":41`) ||
		!strings.Contains(recorder.Body.String(), `"requestId":"req_profile"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestIssueSearchRouteUsesStandardEnvelope(t *testing.T) {
	router := newTestRouter(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/issues/search?page=1&perPage=20",
		strings.NewReader(`{"username":"octocat"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "req_search")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, fragment := range []string{
		`"items":[]`,
		`"page":1`,
		`"perPage":20`,
		`"excludedByReason":[]`,
		`"warnings":[]`,
		`"rateLimitRemaining":40`,
		`"requestId":"req_search"`,
	} {
		if !strings.Contains(recorder.Body.String(), fragment) {
			t.Errorf("body missing %s: %s", fragment, recorder.Body.String())
		}
	}
}

func TestIssueDetailRouteUsesStandardEnvelope(t *testing.T) {
	router := newTestRouter(t)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/issues/acme/rocket/42?skills=Go",
		nil,
	)
	request.Header.Set("X-Request-ID", "req_detail")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, fragment := range []string{
		`"fullName":"acme/rocket"`,
		`"number":42`,
		`"score":80`,
		`"inspection":{"incomplete":false}`,
		`"rateLimitRemaining":39`,
		`"requestId":"req_detail"`,
	} {
		if !strings.Contains(recorder.Body.String(), fragment) {
			t.Errorf("body missing %s: %s", fragment, recorder.Body.String())
		}
	}
}

func TestNewRequiresLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig(t)

	_, err := New(Dependencies{
		Config:    cfg,
		Responder: response.NewResponder(),
	})

	if err == nil {
		t.Fatalf("New() error = nil")
	}
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router, err := New(Dependencies{
		Config:               testConfig(t),
		Logger:               slog.New(slog.NewJSONHandler(&logs, nil)),
		Responder:            response.NewResponder(),
		GetGitHubUser:        routerGetGitHubUserStub{},
		AnalyzeGitHubProfile: routerAnalyzeGitHubProfileStub{},
		SearchIssues:         routerSearchIssuesStub{},
		RecommendIssue:       routerRecommendIssueStub{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return router
}

type routerGetGitHubUserStub struct{}

func (routerGetGitHubUserStub) Execute(
	context.Context,
	user.Username,
) (usecase.GetGitHubUserOutput, error) {
	return usecase.GetGitHubUserOutput{
		Profile:   user.Profile{Login: "octocat"},
		RateLimit: port.RateLimit{Known: true, Remaining: 42},
	}, nil
}

type routerAnalyzeGitHubProfileStub struct{}

func (routerAnalyzeGitHubProfileStub) Execute(
	context.Context,
	user.Username,
) (usecase.AnalyzeGitHubProfileOutput, error) {
	return usecase.AnalyzeGitHubProfileOutput{
		Analysis: profile.Analysis{Username: "octocat"},
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 41,
		},
	}, nil
}

type routerSearchIssuesStub struct{}

func (routerSearchIssuesStub) Execute(
	context.Context,
	usecase.SearchIssuesInput,
) (usecase.SearchIssuesOutput, error) {
	return usecase.SearchIssuesOutput{
		Pagination: usecase.SearchIssuesPagination{
			Page:    1,
			PerPage: 20,
		},
		ExclusionCounts: make(map[issue.ExclusionReason]int),
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 40,
		},
	}, nil
}

type routerRecommendIssueStub struct{}

func (routerRecommendIssueStub) Execute(
	_ context.Context,
	input usecase.RecommendIssueInput,
) (usecase.RecommendIssueOutput, error) {
	return usecase.RecommendIssueOutput{
		Item: issue.RankedIssue{
			Candidate: issue.Candidate{
				Repository: repository.Summary{
					Owner: input.Reference.Owner(),
					Name:  input.Reference.RepositoryName(),
					FullName: input.Reference.Owner() + "/" +
						input.Reference.RepositoryName(),
					UpdatedAt: time.Now().UTC(),
				},
				Issue: issue.Summary{
					Number:    input.Reference.Number(),
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				},
			},
			Recommendation: issue.Recommendation{Score: 80},
		},
		RateLimit: port.RateLimit{Known: true, Remaining: 39},
	}, nil
}

func (routerRecommendIssueStub) EvaluateCandidate(
	candidate issue.Candidate,
	_ []string,
) issue.RankedIssue {
	return issue.RankedIssue{Candidate: candidate}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("ALLOWED_ORIGINS", "https://issuescout.example")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}
