package usecase

import (
	"context"
	"errors"
	"net/http"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// GetGitHubUserOutput combines a public profile, a bounded owned-repository
// list, and the most recent known GitHub quota snapshot.
type GetGitHubUserOutput struct {
	Profile      user.Profile
	Repositories []repository.Summary
	RateLimit    port.RateLimit
}

// GetGitHubUser retrieves a public profile and bounded repository list.
// Implementations must honor ctx and return only normalized domain values.
type GetGitHubUser interface {
	// Execute retrieves the profile before its bounded repository list, honors
	// ctx, and returns the last known quota snapshot.
	Execute(
		ctx context.Context,
		username user.Username,
	) (GetGitHubUserOutput, error)
}

type getGitHubUser struct {
	reader          port.GitHubProfileReader
	repositoryLimit int
}

// NewGetGitHubUser composes a profile reader with its repository result limit.
func NewGetGitHubUser(
	reader port.GitHubProfileReader,
	repositoryLimit int,
) GetGitHubUser {
	return &getGitHubUser{
		reader:          reader,
		repositoryLimit: repositoryLimit,
	}
}

func (u *getGitHubUser) Execute(
	ctx context.Context,
	username user.Username,
) (GetGitHubUserOutput, error) {
	result, err := u.reader.GetUser(ctx, username)
	if err != nil {
		return GetGitHubUserOutput{}, mapGitHubUserError(err)
	}

	repositories, repositoryRateLimit, err := u.reader.ListRepositories(
		ctx,
		username,
		u.repositoryLimit,
	)
	if err != nil {
		return GetGitHubUserOutput{}, mapGitHubUserError(err)
	}
	rateLimit := result.RateLimit
	if repositoryRateLimit.Known {
		rateLimit = repositoryRateLimit
	}

	return GetGitHubUserOutput{
		Profile:      result.Profile,
		Repositories: repositories,
		RateLimit:    rateLimit,
	}, nil
}

func mapGitHubUserError(err error) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return apperror.Wrap(
			apperror.CodeRequestTimeout,
			"The request was cancelled or timed out",
			http.StatusGatewayTimeout,
			err,
		)
	case port.IsGitHubError(err, port.GitHubErrorNotFound):
		return apperror.Wrap(
			apperror.CodeGitHubUserNotFound,
			"GitHub user was not found",
			http.StatusNotFound,
			err,
		)
	case port.IsGitHubError(err, port.GitHubErrorRateLimited):
		return apperror.Wrap(
			apperror.CodeRateLimit,
			"GitHub API rate limit was exceeded",
			http.StatusTooManyRequests,
			err,
		)
	default:
		return apperror.Wrap(
			apperror.CodeGitHubAPI,
			"Unable to retrieve data from GitHub",
			http.StatusBadGateway,
			err,
		)
	}
}

var _ GetGitHubUser = (*getGitHubUser)(nil)
