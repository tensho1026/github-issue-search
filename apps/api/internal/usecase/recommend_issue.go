package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// RecommendIssueInput identifies one public issue and the contributor skills
// used for the explicit match denominator.
type RecommendIssueInput struct {
	Reference     issue.Reference
	DesiredSkills []string
}

// RecommendIssueOutput contains the shared list/detail analysis plus
// operational metadata that remains outside the domain model.
type RecommendIssueOutput struct {
	Item         issue.RankedIssue
	Dependencies []string
	RateLimit    port.RateLimit
	Incomplete   bool
	CacheHit     bool
}

// IssueRecommender provides cached detail reads and a no-I/O fallback for
// candidates beyond the bounded enrichment window.
type IssueRecommender interface {
	// Execute returns cached or freshly loaded detail analysis, collapses
	// concurrent misses, and honors ctx cancellation.
	Execute(
		ctx context.Context,
		input RecommendIssueInput,
	) (RecommendIssueOutput, error)
	// EvaluateCandidate performs deterministic no-I/O fallback analysis with
	// unavailable repository evidence.
	EvaluateCandidate(
		candidate issue.Candidate,
		desiredSkills []string,
	) issue.RankedIssue
}

type recommendIssue struct {
	reader   port.GitHubIssueDetailReader
	cache    port.IssueDetailCache
	requests singleflight.Group
	now      func() time.Time
}

// NewRecommendIssue composes detailed GitHub inspection with a concurrency-safe
// cache. Both dependencies are required.
func NewRecommendIssue(
	reader port.GitHubIssueDetailReader,
	cache port.IssueDetailCache,
) (IssueRecommender, error) {
	if reader == nil {
		return nil, fmt.Errorf(
			"compose issue recommendation: GitHub detail reader is required",
		)
	}
	if cache == nil {
		return nil, fmt.Errorf(
			"compose issue recommendation: detail cache is required",
		)
	}
	return &recommendIssue{
		reader: reader,
		cache:  cache,
		now:    time.Now,
	}, nil
}

func (usecase *recommendIssue) Execute(
	ctx context.Context,
	input RecommendIssueInput,
) (RecommendIssueOutput, error) {
	if err := ctx.Err(); err != nil {
		return RecommendIssueOutput{}, mapIssueDetailError(err)
	}
	key := input.Reference.CacheKey()
	if cached, found, err := usecase.cache.Get(ctx, key); err == nil && found {
		return usecase.output(cached, input.DesiredSkills, true), nil
	} else if err != nil && ctx.Err() != nil {
		return RecommendIssueOutput{}, mapIssueDetailError(err)
	}

	resultChannel := usecase.requests.DoChan(key, func() (any, error) {
		if cached, found, err := usecase.cache.Get(ctx, key); err == nil && found {
			return issueDetailLoad{detail: cached, cacheHit: true}, nil
		} else if err != nil && ctx.Err() != nil {
			return issueDetailLoad{}, err
		}

		detail, err := usecase.reader.GetIssueDetail(
			ctx,
			input.Reference.Owner(),
			input.Reference.RepositoryName(),
			input.Reference.Number(),
		)
		if err != nil {
			return issueDetailLoad{}, err
		}
		_ = usecase.cache.Set(ctx, key, detail)
		return issueDetailLoad{detail: detail}, nil
	})

	select {
	case <-ctx.Done():
		return RecommendIssueOutput{}, mapIssueDetailError(ctx.Err())
	case result := <-resultChannel:
		if result.Err != nil {
			return RecommendIssueOutput{}, mapIssueDetailError(result.Err)
		}
		load, valid := result.Val.(issueDetailLoad)
		if !valid {
			return RecommendIssueOutput{}, apperror.New(
				apperror.CodeInternal,
				"An unexpected error occurred",
				http.StatusInternalServerError,
			)
		}
		return usecase.output(
			load.detail,
			input.DesiredSkills,
			load.cacheHit,
		), nil
	}
}

type issueDetailLoad struct {
	detail   port.GitHubIssueDetailResult
	cacheHit bool
}

func (usecase *recommendIssue) output(
	detail port.GitHubIssueDetailResult,
	desiredSkills []string,
	cacheHit bool,
) RecommendIssueOutput {
	return RecommendIssueOutput{
		Item: evaluateIssueRecommendation(
			detail.Candidate,
			detail.Dependencies,
			detail.RepositorySignals,
			detail.Activity,
			issue.DetectClaim(
				detail.Comments,
				detail.CommentsTruncated,
			),
			desiredSkills,
			usecase.now(),
		),
		Dependencies: append(
			make([]string, 0, len(detail.Dependencies)),
			detail.Dependencies...,
		),
		RateLimit:  detail.RateLimit,
		Incomplete: detail.Incomplete,
		CacheHit:   cacheHit,
	}
}

func (usecase *recommendIssue) EvaluateCandidate(
	candidate issue.Candidate,
	desiredSkills []string,
) issue.RankedIssue {
	lastMeaningfulUpdate := candidate.Repository.UpdatedAt
	if candidate.Repository.PushedAt.After(lastMeaningfulUpdate) {
		lastMeaningfulUpdate = candidate.Repository.PushedAt
	}
	return evaluateIssueRecommendation(
		candidate,
		nil,
		nil,
		issue.ActivityMetrics{
			LastMeaningfulUpdate: lastMeaningfulUpdate,
			CI:                   issue.CIStateUnknown,
		},
		issue.DetectClaim(nil, true),
		desiredSkills,
		usecase.now(),
	)
}

func evaluateIssueRecommendation(
	candidate issue.Candidate,
	dependencies []string,
	repositorySignals []issue.RepositorySignal,
	activity issue.ActivityMetrics,
	claim issue.ClaimEvidence,
	desiredSkills []string,
	now time.Time,
) issue.RankedIssue {
	hasMaintainerGuidance := false
	for _, signal := range repositorySignals {
		if signal.Key == issue.RepositoryContributing &&
			signal.State == issue.SignalPresent {
			hasMaintainerGuidance = true
			break
		}
	}
	analysis := issue.AnalyzeIssue(issue.AnalysisInput{
		Candidate:             candidate,
		Dependencies:          dependencies,
		HasMaintainerGuidance: hasMaintainerGuidance,
	})
	recommendation := issue.Recommend(issue.RecommendationInput{
		Candidate:         candidate,
		Analysis:          analysis,
		DesiredSkills:     append([]string(nil), desiredSkills...),
		RepositorySignals: repositorySignals,
		Activity:          activity,
		Claim:             claim,
		Now:               now,
	})
	return issue.RankedIssue{
		Candidate:      candidate,
		Analysis:       analysis,
		Recommendation: recommendation,
	}
}

func mapIssueDetailError(err error) error {
	switch {
	case errors.Is(err, issue.ErrInvalidReference):
		return apperror.Wrap(
			apperror.CodeInvalidRequest,
			"Issue reference is invalid",
			http.StatusBadRequest,
			err,
		)
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
			apperror.CodeNotFound,
			"GitHub issue was not found",
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
			"Unable to retrieve GitHub issue details",
			http.StatusBadGateway,
			err,
		)
	}
}

var _ IssueRecommender = (*recommendIssue)(nil)
