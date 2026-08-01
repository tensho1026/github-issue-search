package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestDatabaseHealthRouteIsSeparateFromProcessHealth(t *testing.T) {
	health := &routerDatabaseHealthStub{}
	router := newTestRouterWithDatabase(t, health, true)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/api/health/database",
			nil,
		),
	)

	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"status":"ready"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if health.calls.Load() != 1 {
		t.Fatalf("Ping() calls = %d", health.calls.Load())
	}
}

func TestAnonymousCoreRoutesNeverProbeDatabase(t *testing.T) {
	health := &routerDatabaseHealthStub{
		err: errors.New("simulated database outage"),
	}
	router := newTestRouterWithDatabase(t, health, true)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/health", nil),
		httptest.NewRequest(http.MethodGet, "/api/github/users/octocat", nil),
		httptest.NewRequest(
			http.MethodGet,
			"/api/github/users/octocat/profile-analysis",
			nil,
		),
		httptest.NewRequest(
			http.MethodPost,
			"/api/issues/search",
			strings.NewReader(`{"username":"octocat"}`),
		),
		httptest.NewRequest(
			http.MethodPost,
			"/api/repositories/search",
			strings.NewReader(`{}`),
		),
		httptest.NewRequest(
			http.MethodGet,
			"/api/issues/acme/rocket/42",
			nil,
		),
	}
	requests[3].Header.Set("Content-Type", "application/json")
	requests[4].Header.Set("Content-Type", "application/json")
	for _, request := range requests {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code >= http.StatusInternalServerError {
			t.Fatalf(
				"%s %s response = %d %s",
				request.Method,
				request.URL.Path,
				recorder.Code,
				recorder.Body.String(),
			)
		}
	}
	if health.calls.Load() != 0 {
		t.Fatalf("anonymous routes made %d database calls", health.calls.Load())
	}
}

func TestDisabledAuthenticationIsReportedWithoutBlockingPublicApp(
	t *testing.T,
) {
	router := newTestRouter(t)
	sessionRecorder := httptest.NewRecorder()
	router.ServeHTTP(
		sessionRecorder,
		httptest.NewRequest(
			http.MethodGet,
			"/api/auth/session",
			nil,
		),
	)
	if sessionRecorder.Code != http.StatusOK ||
		!strings.Contains(
			sessionRecorder.Body.String(),
			`"configured":false`,
		) ||
		!strings.Contains(
			sessionRecorder.Body.String(),
			`"authenticated":false`,
		) {
		t.Fatalf(
			"session response = %d %s",
			sessionRecorder.Code,
			sessionRecorder.Body.String(),
		)
	}

	startRecorder := httptest.NewRecorder()
	router.ServeHTTP(
		startRecorder,
		httptest.NewRequest(
			http.MethodGet,
			"/api/auth/github/start",
			nil,
		),
	)
	if startRecorder.Code != http.StatusServiceUnavailable ||
		!strings.Contains(startRecorder.Body.String(), "AUTH_UNAVAILABLE") {
		t.Fatalf(
			"start response = %d %s",
			startRecorder.Code,
			startRecorder.Body.String(),
		)
	}
}

func TestAccountOnlyRouteRequiresConfiguredAuthenticationWithoutDatabaseCall(
	t *testing.T,
) {
	health := &routerDatabaseHealthStub{
		err: errors.New("simulated database outage"),
	}
	router := newTestRouterWithDatabase(t, health, true)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		httptest.NewRequest(
			http.MethodGet,
			"/api/account/bookmarks",
			nil,
		),
	)

	if recorder.Code != http.StatusServiceUnavailable ||
		!strings.Contains(recorder.Body.String(), "AUTH_UNAVAILABLE") {
		t.Fatalf(
			"account response = %d %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
	if health.calls.Load() != 0 {
		t.Fatalf("account middleware probed health %d times", health.calls.Load())
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

func TestDocumentationRoutesAreInjectedAndDisableable(t *testing.T) {
	router := newTestRouter(t)
	for _, path := range []string{
		"/docs",
		"/docs/",
		"/docs/swagger-ui.css",
		"/openapi.yaml",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Request-ID", "req_docs")
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK ||
			recorder.Body.String() != "injected documentation" {
			t.Errorf(
				"%s response = %d %q",
				path,
				recorder.Code,
				recorder.Body.String(),
			)
		}
		if recorder.Header().Get("X-Request-ID") != "req_docs" {
			t.Errorf("%s did not preserve request correlation", path)
		}
	}

	cfg := testConfig(t)
	cfg.APIDocumentationEnabled = false
	disabled, err := newTestRouterFromDependencies(Dependencies{
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("New() disabled documentation error = %v", err)
	}
	recorder := httptest.NewRecorder()
	disabled.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/docs/", nil),
	)
	if recorder.Code != http.StatusNotFound ||
		!strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf(
			"disabled response = %d %s",
			recorder.Code,
			recorder.Body.String(),
		)
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

func TestRepositoryDiscoveryRouteUsesStandardEnvelope(t *testing.T) {
	router := newTestRouter(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/repositories/search?page=1&perPage=20",
		strings.NewReader(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "req_repositories")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, fragment := range []string{
		`"items":[]`,
		`"page":1`,
		`"perPage":20`,
		`"candidatesChecked":0`,
		`"warnings":[]`,
		`"rateLimitRemaining":38`,
		`"requestId":"req_repositories"`,
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

func TestNewRequiresEnabledDocumentationHandler(t *testing.T) {
	cfg := testConfig(t)
	_, err := newTestRouterFromDependencies(Dependencies{Config: cfg})
	if err == nil ||
		!strings.Contains(err.Error(), "documentation handler is required") {
		t.Fatalf("New() error = %v", err)
	}
}

func newTestRouter(t *testing.T) http.Handler {
	return newTestRouterWithDatabase(t, nil, false)
}

func newTestRouterWithDatabase(
	t *testing.T,
	databaseHealth port.DatabaseHealth,
	databaseConfigured bool,
) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router, err := New(Dependencies{
		Config:    testConfig(t),
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		Responder: response.NewResponder(),
		Documentation: http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			_, _ = writer.Write([]byte("injected documentation"))
		}),
		GetGitHubUser:        routerGetGitHubUserStub{},
		AnalyzeGitHubProfile: routerAnalyzeGitHubProfileStub{},
		SearchIssues:         routerSearchIssuesStub{},
		SearchRepositories:   routerSearchRepositoriesStub{},
		RecommendIssue:       routerRecommendIssueStub{},
		DatabaseHealth:       databaseHealth,
		DatabaseConfigured:   databaseConfigured,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return router
}

func newTestRouterFromDependencies(
	dependencies Dependencies,
) (http.Handler, error) {
	var logs bytes.Buffer
	dependencies.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	dependencies.Responder = response.NewResponder()
	dependencies.GetGitHubUser = routerGetGitHubUserStub{}
	dependencies.AnalyzeGitHubProfile = routerAnalyzeGitHubProfileStub{}
	dependencies.SearchIssues = routerSearchIssuesStub{}
	dependencies.SearchRepositories = routerSearchRepositoriesStub{}
	dependencies.RecommendIssue = routerRecommendIssueStub{}

	return New(dependencies)
}

type routerDatabaseHealthStub struct {
	calls atomic.Int64
	err   error
}

func (health *routerDatabaseHealthStub) Ping(context.Context) error {
	health.calls.Add(1)
	return health.err
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

type routerSearchRepositoriesStub struct{}

func (routerSearchRepositoriesStub) Execute(
	context.Context,
	usecase.SearchRepositoriesInput,
) (usecase.SearchRepositoriesOutput, error) {
	return usecase.SearchRepositoriesOutput{
		Pagination: usecase.SearchRepositoriesPagination{
			Page:    1,
			PerPage: 20,
		},
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 38,
		},
	}, nil
}

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
