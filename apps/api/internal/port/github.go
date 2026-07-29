package port

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	Limit     int
	Remaining int
	Reset     time.Time
}

type GitHubUserResult struct {
	Profile   user.Profile
	RateLimit RateLimit
}

// GitHubUserReader is the application-facing port for user profile reads.
type GitHubUserReader interface {
	GetUser(ctx context.Context, username user.Username) (GitHubUserResult, error)
}
