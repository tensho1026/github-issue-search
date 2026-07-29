package router

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/handler"
	"github.com/tensho1026/github-issue-search/apps/api/internal/middleware"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

type Dependencies struct {
	Config               config.Config
	Logger               *slog.Logger
	Responder            response.Responder
	GetGitHubUser        usecase.GetGitHubUser
	AnalyzeGitHubProfile usecase.AnalyzeGitHubProfile
	SearchIssues         usecase.SearchIssues
}

// New composes concrete HTTP dependencies. Feature handlers are constructed by
// the application composition root and registered here.
func New(dependencies Dependencies) (http.Handler, error) {
	if dependencies.Logger == nil {
		return nil, fmt.Errorf("compose router: logger is required")
	}
	if dependencies.GetGitHubUser == nil {
		return nil, fmt.Errorf("compose router: get GitHub user usecase is required")
	}
	if dependencies.AnalyzeGitHubProfile == nil {
		return nil, fmt.Errorf(
			"compose router: analyze GitHub profile usecase is required",
		)
	}
	if dependencies.SearchIssues == nil {
		return nil, fmt.Errorf("compose router: search issues usecase is required")
	}

	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}

	engine.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(),
		middleware.CORS(dependencies.Config.AllowedOrigins, dependencies.Responder),
		middleware.RequestLogger(dependencies.Logger),
		middleware.Recovery(dependencies.Logger, dependencies.Responder),
	)

	healthHandler := handler.NewHealthHandler(dependencies.Responder)
	gitHubUserHandler := handler.NewGitHubUserHandler(
		dependencies.GetGitHubUser,
		dependencies.Responder,
	)
	gitHubProfileAnalysisHandler := handler.NewGitHubProfileAnalysisHandler(
		dependencies.AnalyzeGitHubProfile,
		dependencies.Responder,
	)
	issueSearchHandler := handler.NewIssueSearchHandler(
		dependencies.SearchIssues,
		dependencies.Responder,
	)
	api := engine.Group("/api")
	api.GET(
		"/health",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		healthHandler.Check,
	)
	api.GET(
		"/github/users/:username",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		gitHubUserHandler.Get,
	)
	api.GET(
		"/github/users/:username/profile-analysis",
		middleware.Timeout(
			dependencies.Config.ProfileRequestTimeout,
			dependencies.Responder,
		),
		gitHubProfileAnalysisHandler.Get,
	)
	api.POST(
		"/issues/search",
		middleware.Timeout(
			dependencies.Config.IssueSearchRequestTimeout,
			dependencies.Responder,
		),
		issueSearchHandler.Search,
	)

	engine.NoRoute(dependencies.Responder.NotFound)

	return engine, nil
}
