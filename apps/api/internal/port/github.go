package port

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
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

type GitHubIssueSearchResult struct {
	Candidates        []issue.Candidate
	TotalCount        int
	IncompleteResults bool
	RateLimit         RateLimit
}

// GitHubIssueDetailResult is one bounded, normalized repository and issue
// inspection. Incomplete indicates that optional GraphQL fields were omitted
// by GitHub and are represented as unknown rather than absent.
type GitHubIssueDetailResult struct {
	Candidate         issue.Candidate
	RepositorySignals []issue.RepositorySignal
	Activity          issue.ActivityMetrics
	Comments          []issue.CommentObservation
	CommentsTruncated bool
	RateLimit         RateLimit
	Incomplete        bool
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

// GitHubIssueSearcher finds one bounded candidate window. Pagination of
// eligible results is an application concern and never drives unbounded
// upstream GraphQL Search paging or repository-detail fan-out.
type GitHubIssueSearcher interface {
	SearchIssues(
		ctx context.Context,
		criteria issue.SearchCriteria,
		limit int,
	) (GitHubIssueSearchResult, error)
}

// GitHubIssueDetailReader retrieves one bounded public inspection without
// exposing GitHub response objects to the application layer.
type GitHubIssueDetailReader interface {
	GetIssueDetail(
		ctx context.Context,
		owner string,
		repositoryName string,
		issueNumber int,
	) (GitHubIssueDetailResult, error)
}

type ProfileAnalysisCacheEntry struct {
	Analysis  profile.Analysis
	RateLimit RateLimit
}

type ProfileAnalysisCache interface {
	Get(
		ctx context.Context,
		username user.Username,
	) (ProfileAnalysisCacheEntry, bool, error)
	Set(
		ctx context.Context,
		username user.Username,
		entry ProfileAnalysisCacheEntry,
	) error
}

type IssueSearchCacheEntry struct {
	Candidates        []issue.Candidate
	ExclusionCounts   map[issue.ExclusionReason]int
	CandidatesChecked int
	UpstreamTotal     int
	IncompleteResults bool
	RateLimit         RateLimit
}

type IssueSearchCache interface {
	Get(
		ctx context.Context,
		key string,
	) (IssueSearchCacheEntry, bool, error)
	Set(
		ctx context.Context,
		key string,
		entry IssueSearchCacheEntry,
	) error
}
