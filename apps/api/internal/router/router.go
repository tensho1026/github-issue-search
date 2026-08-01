package router

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/handler"
	"github.com/tensho1026/github-issue-search/apps/api/internal/middleware"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authcrypto"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

// Dependencies contains every already-constructed router collaborator.
// Anonymous dependencies are always required; account dependencies are
// required only when Config.AuthEnabled is true.
type Dependencies struct {
	Config               config.Config
	Logger               *slog.Logger
	Responder            response.Responder
	GetGitHubUser        usecase.GetGitHubUser
	AnalyzeGitHubProfile usecase.AnalyzeGitHubProfile
	SearchIssues         usecase.SearchIssues
	SearchRepositories   usecase.SearchRepositories
	RecommendIssue       usecase.IssueRecommender
	DatabaseHealth       port.DatabaseHealth
	DatabaseConfigured   bool
	Authentication       usecase.Authentication
	AuthFlowCodec        *authcrypto.FlowCodec
	AccountWorkspace     usecase.AccountWorkspace
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
	if dependencies.SearchRepositories == nil {
		return nil, fmt.Errorf(
			"compose router: search repositories usecase is required",
		)
	}
	if dependencies.RecommendIssue == nil {
		return nil, fmt.Errorf(
			"compose router: recommend issue usecase is required",
		)
	}
	if dependencies.DatabaseConfigured && dependencies.DatabaseHealth == nil {
		return nil, fmt.Errorf(
			"compose router: configured database health probe is required",
		)
	}
	if dependencies.Config.AuthEnabled &&
		(dependencies.Authentication == nil ||
			dependencies.AuthFlowCodec == nil ||
			dependencies.AccountWorkspace == nil) {
		return nil, fmt.Errorf(
			"compose router: enabled authentication dependencies are required",
		)
	}

	engine := gin.New()
	if err := engine.SetTrustedProxies(
		dependencies.Config.TrustedProxyCIDRs,
	); err != nil {
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
	databaseHealthHandler := handler.NewDatabaseHealthHandler(
		dependencies.DatabaseHealth,
		dependencies.DatabaseConfigured,
		dependencies.Responder,
	)
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
	issueDetailHandler := handler.NewIssueDetailHandler(
		dependencies.RecommendIssue,
		dependencies.Responder,
	)
	repositoryDiscoveryHandler := handler.NewRepositoryDiscoveryHandler(
		dependencies.SearchRepositories,
		dependencies.Responder,
	)
	authCookies := authhttp.NewPolicy(
		dependencies.Config.AuthCookieSecure,
	)
	authHandler, err := handler.NewAuthHandler(
		dependencies.Config.AuthEnabled,
		dependencies.Authentication,
		dependencies.AuthFlowCodec,
		authCookies,
		dependencies.Config.AuthFrontendURL,
		dependencies.Responder,
	)
	if err != nil {
		return nil, fmt.Errorf("compose authentication handler: %w", err)
	}
	accountHandler := handler.NewAccountHandler(
		dependencies.AccountWorkspace,
		authCookies,
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
		"/health/database",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		databaseHealthHandler.Check,
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
	api.POST(
		"/repositories/search",
		middleware.Timeout(
			dependencies.Config.RepositoryDiscoveryRequestTimeout,
			dependencies.Responder,
		),
		repositoryDiscoveryHandler.Search,
	)
	api.GET(
		"/issues/:owner/:repository/:issueNumber",
		middleware.Timeout(
			dependencies.Config.IssueDetailRequestTimeout,
			dependencies.Responder,
		),
		issueDetailHandler.Get,
	)
	api.GET(
		"/auth/session",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authHandler.Session,
	)
	api.GET(
		"/auth/github/start",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authHandler.Start,
	)
	api.GET(
		"/auth/github/callback",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authHandler.Callback,
	)
	authenticatedMutation := middleware.RequireAuthenticatedCSRF(
		dependencies.Authentication,
		authCookies,
		dependencies.Responder,
	)
	authenticatedRead := middleware.RequireAuthenticated(
		dependencies.Authentication,
		authCookies,
		dependencies.Responder,
	)
	api.POST(
		"/auth/session/refresh",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		authHandler.Refresh,
	)
	api.POST(
		"/auth/logout",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		authHandler.Logout,
	)
	api.GET(
		"/account/bookmarks",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedRead,
		accountHandler.ListBookmarks,
	)
	api.PUT(
		"/account/bookmarks",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.UpsertBookmark,
	)
	api.DELETE(
		"/account/bookmarks/:bookmarkID",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.DeleteBookmark,
	)
	api.GET(
		"/account/saved-searches",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedRead,
		accountHandler.ListSavedSearches,
	)
	api.POST(
		"/account/saved-searches",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.CreateSavedSearch,
	)
	api.PUT(
		"/account/saved-searches/:savedSearchID",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.UpdateSavedSearch,
	)
	api.DELETE(
		"/account/saved-searches/:savedSearchID",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.DeleteSavedSearch,
	)
	api.GET(
		"/account/preferences",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedRead,
		accountHandler.GetPreferences,
	)
	api.PUT(
		"/account/preferences",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.UpdatePreferences,
	)
	api.GET(
		"/account/export",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedRead,
		accountHandler.Export,
	)
	api.DELETE(
		"/account",
		middleware.Timeout(
			dependencies.Config.NormalRequestTimeout,
			dependencies.Responder,
		),
		authenticatedMutation,
		accountHandler.DeleteAccount,
	)

	engine.NoRoute(dependencies.Responder.NotFound)

	return engine, nil
}
