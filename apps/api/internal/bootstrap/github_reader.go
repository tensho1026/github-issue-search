// Package bootstrap contains infrastructure selection at the composition
// boundary. Domain and usecase packages remain unaware of runtime adapters.
package bootstrap

import (
	"log/slog"
	"time"

	githubclient "github.com/tensho1026/github-issue-search/apps/api/internal/client/github"
	"github.com/tensho1026/github-issue-search/apps/api/internal/client/githubmock"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// NewGitHubReader selects exactly one GitHub adapter from validated process
// configuration. Config rejects mock mode outside APP_ENV=test.
func NewGitHubReader(cfg config.Config, logger *slog.Logger) port.GitHubReader {
	if cfg.UseGitHubAPIMock {
		logger.Warn("deterministic GitHub API mock enabled")
		return githubmock.New(time.Now)
	}
	return githubclient.NewClient(
		cfg.GitHubAPIBaseURL,
		cfg.GitHubToken,
		cfg.GitHubRequestTimeout,
		logger,
	)
}
