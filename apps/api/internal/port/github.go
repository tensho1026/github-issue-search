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

// GitHubErrorKind classifies upstream failures without exposing response
// bodies or credentials.
type GitHubErrorKind string

// GitHubErrorKind values drive stable application error mapping.
const (
	GitHubErrorNotFound     GitHubErrorKind = "not_found"
	GitHubErrorRateLimited  GitHubErrorKind = "rate_limited"
	GitHubErrorUnauthorized GitHubErrorKind = "unauthorized"
	GitHubErrorUpstream     GitHubErrorKind = "upstream"
)

// GitHubError carries a stable kind, optional rate-limit reset, and an
// internal cause. The cause must never cross the HTTP boundary.
type GitHubError struct {
	Kind  GitHubErrorKind
	Reset time.Time
	Cause error
}

// Error returns a credential-free classification string.
func (e *GitHubError) Error() string {
	return fmt.Sprintf("GitHub request failed: %s", e.Kind)
}

// Unwrap exposes the internal cause to errors.Is and errors.As callers.
func (e *GitHubError) Unwrap() error {
	return e.Cause
}

// IsGitHubError reports whether err wraps a GitHubError of kind.
func IsGitHubError(err error, kind GitHubErrorKind) bool {
	var gitHubError *GitHubError
	return errors.As(err, &gitHubError) && gitHubError.Kind == kind
}

// RateLimit is a normalized GitHub quota snapshot. Numeric fields are
// meaningful only when Known is true.
type RateLimit struct {
	Known     bool
	Limit     int
	Remaining int
	Reset     time.Time
}

// GitHubUserResult pairs one normalized public profile with quota metadata.
type GitHubUserResult struct {
	Profile   user.Profile
	RateLimit RateLimit
}

// GitHubProfileAnalysisResult contains the bounded public evidence snapshot
// required by profile analysis.
type GitHubProfileAnalysisResult struct {
	Snapshot  profile.ProfileSnapshot
	RateLimit RateLimit
}

// GitHubIssueSearchResult is a bounded candidate window and its completeness
// and quota metadata.
type GitHubIssueSearchResult struct {
	Candidates        []issue.Candidate
	TotalCount        int
	IncompleteResults bool
	RateLimit         RateLimit
}

// GitHubRepositoryDiscoveryResult is one bounded public repository candidate
// window. IncompleteResults indicates either upstream partial GraphQL data or
// a larger upstream result beyond the configured window.
type GitHubRepositoryDiscoveryResult struct {
	Candidates        []repository.DiscoveryCandidate
	TotalCount        int
	IncompleteResults bool
	RateLimit         RateLimit
}

// GitHubRepositoryEnrichmentResult contains documentation evidence for a
// bounded shortlist keyed by canonical lower-case owner/name.
type GitHubRepositoryEnrichmentResult struct {
	Items             map[string]repository.DiscoveryEnrichment
	IncompleteResults bool
	RateLimit         RateLimit
}

// GitHubIssueDetailResult is one bounded, normalized repository and issue
// inspection. Incomplete indicates that optional GraphQL fields were omitted
// by GitHub and are represented as unknown rather than absent.
type GitHubIssueDetailResult struct {
	Candidate         issue.Candidate
	Dependencies      []string
	RepositorySignals []issue.RepositorySignal
	Activity          issue.ActivityMetrics
	Comments          []issue.CommentObservation
	CommentsTruncated bool
	RateLimit         RateLimit
	Incomplete        bool
}

// GitHubUserReader is the application-facing port for user profile reads.
type GitHubUserReader interface {
	// GetUser retrieves one normalized public profile, honors ctx, and returns a
	// classified GitHubError on upstream failure.
	GetUser(ctx context.Context, username user.Username) (GitHubUserResult, error)
}

// GitHubRepositoryReader retrieves at most limit owned public repositories.
// Implementations must honor ctx cancellation and return caller-owned slices.
type GitHubRepositoryReader interface {
	// ListRepositories returns at most limit caller-owned public repositories;
	// a non-positive limit performs no I/O.
	ListRepositories(
		ctx context.Context,
		username user.Username,
		limit int,
	) ([]repository.Summary, RateLimit, error)
}

// GitHubProfileReader combines the public profile and repository operations
// needed by the basic profile use case.
type GitHubProfileReader interface {
	GitHubUserReader
	GitHubRepositoryReader
}

// GitHubProfileAnalysisReader retrieves a bounded public evidence snapshot.
// Implementations must cap repository and manifest work and honor ctx.
type GitHubProfileAnalysisReader interface {
	// GetProfileAnalysis returns a bounded public snapshot with explicit
	// sampling status and no private contribution data.
	GetProfileAnalysis(
		ctx context.Context,
		username user.Username,
		repositoryLimit int,
		manifestLimit int,
	) (GitHubProfileAnalysisResult, error)
}

// GitHubIssueSearcher finds one bounded candidate window. Pagination of
// eligible results is an application concern and never drives unbounded
// upstream GraphQL Search paging or repository-detail fan-out.
type GitHubIssueSearcher interface {
	// SearchIssues returns at most limit normalized candidates and preserves
	// upstream partial-result and rate-limit metadata.
	SearchIssues(
		ctx context.Context,
		criteria issue.SearchCriteria,
		limit int,
	) (GitHubIssueSearchResult, error)
}

// GitHubRepositoryDiscoverySearcher performs the cheap bounded candidate
// search without README content fan-out.
type GitHubRepositoryDiscoverySearcher interface {
	// SearchRepositories returns at most limit cheap normalized candidates and
	// does not perform per-repository documentation fan-out.
	SearchRepositories(
		ctx context.Context,
		criteria repository.DiscoveryCriteria,
		limit int,
	) (GitHubRepositoryDiscoveryResult, error)
}

// GitHubRepositoryDiscoveryEnricher inspects public documentation for only the
// preselected shortlist.
type GitHubRepositoryDiscoveryEnricher interface {
	// EnrichRepositories returns bounded documentation evidence keyed by
	// canonical repository name; partial failures remain explicit.
	EnrichRepositories(
		ctx context.Context,
		repositories []repository.Summary,
	) (GitHubRepositoryEnrichmentResult, error)
}

// GitHubIssueDetailReader retrieves one bounded public inspection without
// exposing GitHub response objects to the application layer.
type GitHubIssueDetailReader interface {
	// GetIssueDetail validates and retrieves one bounded issue, repository,
	// activity, and comment snapshot while honoring ctx.
	GetIssueDetail(
		ctx context.Context,
		owner string,
		repositoryName string,
		issueNumber int,
	) (GitHubIssueDetailResult, error)
}

// GitHubReader is the complete public-data boundary required by the current
// application. Production and deterministic test adapters implement the same
// port so usecases never branch on runtime infrastructure.
type GitHubReader interface {
	GitHubProfileReader
	GitHubProfileAnalysisReader
	GitHubIssueSearcher
	GitHubIssueDetailReader
	GitHubRepositoryDiscoverySearcher
	GitHubRepositoryDiscoveryEnricher
}

// ProfileAnalysisCacheEntry owns a normalized profile result and the quota
// snapshot observed when it was produced.
type ProfileAnalysisCacheEntry struct {
	Analysis  profile.Analysis
	RateLimit RateLimit
}

// ProfileAnalysisCache stores profile analysis by validated username.
// Implementations must be concurrency-safe, honor ctx, and isolate mutable
// value ownership on both Get and Set.
type ProfileAnalysisCache interface {
	// Get returns an ownership-isolated entry, false on miss, and honors ctx.
	Get(
		ctx context.Context,
		username user.Username,
	) (ProfileAnalysisCacheEntry, bool, error)
	// Set stores an ownership-isolated entry and honors ctx.
	Set(
		ctx context.Context,
		username user.Username,
		entry ProfileAnalysisCacheEntry,
	) error
}

// IssueSearchCacheEntry owns the bounded pre-pagination candidate and
// exclusion window used by issue discovery.
type IssueSearchCacheEntry struct {
	Candidates        []issue.Candidate
	ExclusionCounts   map[issue.ExclusionReason]int
	CandidatesChecked int
	UpstreamTotal     int
	IncompleteResults bool
	RateLimit         RateLimit
}

// IssueSearchCache stores canonical issue-search windows. Implementations must
// be concurrency-safe, honor ctx, and isolate mutable value ownership.
type IssueSearchCache interface {
	// Get returns an ownership-isolated entry, false on miss, and honors ctx.
	Get(
		ctx context.Context,
		key string,
	) (IssueSearchCacheEntry, bool, error)
	// Set stores an ownership-isolated entry and honors ctx.
	Set(
		ctx context.Context,
		key string,
		entry IssueSearchCacheEntry,
	) error
}

// RepositoryDiscoveryCacheEntry owns a bounded enriched repository window and
// its partial-result counters.
type RepositoryDiscoveryCacheEntry struct {
	Items                []repository.DiscoveryResult
	CandidatesChecked    int
	UpstreamTotal        int
	SearchIncomplete     bool
	EnrichmentAttempted  int
	EnrichmentFailed     int
	EnrichmentIncomplete bool
	RateLimit            RateLimit
}

// RepositoryDiscoveryCache stores canonical repository-discovery windows.
// Implementations must be concurrency-safe, honor ctx, and isolate mutable
// value ownership.
type RepositoryDiscoveryCache interface {
	// Get returns an ownership-isolated entry, false on miss, and honors ctx.
	Get(
		ctx context.Context,
		key string,
	) (RepositoryDiscoveryCacheEntry, bool, error)
	// Set stores an ownership-isolated entry and honors ctx.
	Set(
		ctx context.Context,
		key string,
		entry RepositoryDiscoveryCacheEntry,
	) error
}

// IssueDetailCache stores normalized public GitHub snapshots by canonical
// owner, repository, and issue number.
type IssueDetailCache interface {
	// Get returns an ownership-isolated detail, false on miss, and honors ctx.
	Get(
		ctx context.Context,
		key string,
	) (GitHubIssueDetailResult, bool, error)
	// Set stores an ownership-isolated detail and honors ctx.
	Set(
		ctx context.Context,
		key string,
		entry GitHubIssueDetailResult,
	) error
}
