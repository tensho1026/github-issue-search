package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/bootstrap"
	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/router"
	"github.com/tensho1026/github-issue-search/apps/api/internal/server"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

var (
	buildCommit  = "unknown"
	buildVersion = "development"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid API configuration", "error", err)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	gitHubClient := bootstrap.NewGitHubReader(cfg, logger)
	getGitHubUser := usecase.NewGetGitHubUser(
		gitHubClient,
		cfg.ProfileRepositoryLimit,
	)
	profileAnalysisCache, err := memory.NewProfileAnalysis(
		cfg.ProfileAnalysisCacheCapacity,
		cfg.ProfileAnalysisCacheTTL,
	)
	if err != nil {
		logger.Error("compose profile analysis cache", "error", err)
		os.Exit(1)
	}
	analyzeGitHubProfile := usecase.NewAnalyzeGitHubProfile(
		gitHubClient,
		profileAnalysisCache,
		cfg.ProfileRepositoryLimit,
		cfg.ManifestFileLimit,
	)
	issueSearchCache, err := memory.NewIssueSearch(
		cfg.IssueSearchCacheCapacity,
		cfg.IssueSearchCacheTTL,
	)
	if err != nil {
		logger.Error("compose issue search cache", "error", err)
		os.Exit(1)
	}
	issueDetailCache, err := memory.NewIssueDetail(
		cfg.IssueDetailCacheCapacity,
		cfg.IssueDetailCacheTTL,
	)
	if err != nil {
		logger.Error("compose issue detail cache", "error", err)
		os.Exit(1)
	}
	recommendIssue, err := usecase.NewRecommendIssue(
		gitHubClient,
		issueDetailCache,
	)
	if err != nil {
		logger.Error("compose issue recommendation usecase", "error", err)
		os.Exit(1)
	}
	searchIssues, err := usecase.NewSearchIssues(
		gitHubClient,
		issueSearchCache,
		cfg.IssueSearchResultLimit,
		usecase.WithIssueRecommendationEnrichment(
			recommendIssue,
			cfg.IssueDetailAnalysisLimit,
			min(
				cfg.GitHubMaxConcurrency,
				cfg.IssueDetailAnalysisLimit,
			),
		),
	)
	if err != nil {
		logger.Error("compose issue search usecase", "error", err)
		os.Exit(1)
	}
	repositoryDiscoveryCache, err := memory.NewRepositoryDiscovery(
		cfg.RepositoryDiscoveryCacheCapacity,
		cfg.RepositoryDiscoveryCacheTTL,
	)
	if err != nil {
		logger.Error("compose repository discovery cache", "error", err)
		os.Exit(1)
	}
	searchRepositories, err := usecase.NewSearchRepositories(
		gitHubClient,
		gitHubClient,
		repositoryDiscoveryCache,
		cfg.RepositoryDiscoveryResultLimit,
		cfg.RepositoryDiscoveryEnrichmentLimit,
	)
	if err != nil {
		logger.Error("compose repository discovery usecase", "error", err)
		os.Exit(1)
	}
	httpHandler, err := router.New(router.Dependencies{
		Config:               cfg,
		Logger:               logger,
		Responder:            response.NewResponder(),
		GetGitHubUser:        getGitHubUser,
		AnalyzeGitHubProfile: analyzeGitHubProfile,
		SearchIssues:         searchIssues,
		SearchRepositories:   searchRepositories,
		RecommendIssue:       recommendIssue,
	})
	if err != nil {
		logger.Error("compose API router", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpHandler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(
		context.Background(),
		"tcp",
		httpServer.Addr,
	)
	if err != nil {
		logger.Error("listen for API traffic", "error", err)
		os.Exit(1)
	}
	logger.Info(
		"starting IssueScout API",
		"address",
		listener.Addr().String(),
		"commit",
		buildCommit,
		"version",
		buildVersion,
	)

	processContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := server.Run(
		processContext,
		httpServer,
		listener,
		cfg.ShutdownTimeout,
		logger,
	); err != nil {
		logger.Error("IssueScout API stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
