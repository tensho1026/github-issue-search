package router_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/client/githubmock"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/router"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func ExampleNew() {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	reader := githubmock.New(func() time.Time { return now })

	profileCache := must(memory.NewProfileAnalysis(8, time.Minute))
	issueSearchCache := must(memory.NewIssueSearch(8, time.Minute))
	issueDetailCache := must(memory.NewIssueDetail(8, time.Minute))
	repositoryCache := must(memory.NewRepositoryDiscovery(8, time.Minute))
	recommender := must(usecase.NewRecommendIssue(reader, issueDetailCache))
	issueSearch := must(usecase.NewSearchIssues(reader, issueSearchCache, 10))
	repositorySearch := must(usecase.NewSearchRepositories(
		reader,
		reader,
		repositoryCache,
		10,
		5,
	))

	handler, err := router.New(router.Dependencies{
		Config: config.Config{
			AllowedOrigins:                    []string{"http://127.0.0.1:5173"},
			NormalRequestTimeout:              time.Second,
			ProfileRequestTimeout:             time.Second,
			IssueSearchRequestTimeout:         time.Second,
			IssueDetailRequestTimeout:         time.Second,
			RepositoryDiscoveryRequestTimeout: time.Second,
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Responder: response.NewResponderWithClock(
			func() time.Time { return now },
		),
		GetGitHubUser: usecase.NewGetGitHubUser(reader, 10),
		AnalyzeGitHubProfile: usecase.NewAnalyzeGitHubProfile(
			reader,
			profileCache,
			10,
			3,
		),
		SearchIssues:       issueSearch,
		SearchRepositories: repositorySearch,
		RecommendIssue:     recommender,
	})
	if err != nil {
		panic(err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/health", nil),
	)
	fmt.Println("status:", recorder.Code)

	// Output:
	// status: 200
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
