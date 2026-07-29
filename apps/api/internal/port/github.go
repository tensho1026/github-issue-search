package port

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

type GitHubErrorKind string

const (
	GitHubErrorNotFound     GitHubErrorKind = "not_found"
	GitHubErrorRateLimited  GitHubErrorKind = "rate_limited"
	GitHubErrorUnauthorized GitHubErrorKind = "unauthorized"
	GitHubErrorUpstream     GitHubErrorKind = "upstream"
)

type GitHubError struct {
	Kind  GitHubErrorKind
	Reset time.Time
	Cause error
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("GitHub request failed: %s", e.Kind)
}

func (e *GitHubError) Unwrap() error {
	return e.Cause
}

func IsGitHubError(err error, kind GitHubErrorKind) bool {
	var gitHubError *GitHubError
	return errors.As(err, &gitHubError) && gitHubError.Kind == kind
}

type RateLimit struct {
	Known     bool
	Limit     int
	Remaining int
	Reset     time.Time
}

type GitHubUserResult struct {
	Profile   user.Profile
	RateLimit RateLimit
}

type GitHubLanguagesResult struct {
	Languages map[string]int64
	RateLimit RateLimit
}

type GitHubRepositoryFileResult struct {
	Content   []byte
	Exists    bool
	RateLimit RateLimit
}

// GitHubUserReader is the application-facing port for user profile reads.
type GitHubUserReader interface {
	GetUser(ctx context.Context, username user.Username) (GitHubUserResult, error)
}

type GitHubRepositoryReader interface {
	ListRepositories(
		ctx context.Context,
		username user.Username,
		limit int,
	) ([]repository.Summary, RateLimit, error)
}

type GitHubProfileReader interface {
	GitHubUserReader
	GitHubRepositoryReader
}

type GitHubProfileAnalysisReader interface {
	GitHubProfileReader
	GetRepositoryLanguages(
		ctx context.Context,
		owner string,
		name string,
	) (GitHubLanguagesResult, error)
	GetRepositoryFile(
		ctx context.Context,
		owner string,
		name string,
		filePath string,
	) (GitHubRepositoryFileResult, error)
}
