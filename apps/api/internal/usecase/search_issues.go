package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// SearchIssuesInput contains validated domain criteria and the requested
// application-level page.
type SearchIssuesInput struct {
	Criteria   issue.SearchCriteria
	Pagination issue.Pagination
}

// SearchIssuesPagination describes a page over at most the configured
// candidate window, after every eligibility rule has run.
type SearchIssuesPagination struct {
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	HasNext    bool
}

// SearchIssuesOutput retains operational metadata without exposing GitHub
// payloads directly to the transport layer.
type SearchIssuesOutput struct {
	Items               []issue.RankedIssue
	Pagination          SearchIssuesPagination
	ExclusionCounts     map[issue.ExclusionReason]int
	CandidatesChecked   int
	UpstreamTotal       int
	EnrichmentAttempted int
	EnrichmentFailed    int
	IncompleteResults   bool
	RateLimit           port.RateLimit
	CacheHit            bool
}

type SearchIssues interface {
	Execute(
		ctx context.Context,
		input SearchIssuesInput,
	) (SearchIssuesOutput, error)
}

type searchIssues struct {
	searcher       port.GitHubIssueSearcher
	cache          port.IssueSearchCache
	resultLimit    int
	recommender    IssueRecommender
	analysisLimit  int
	maxConcurrency int
	requests       singleflight.Group
	now            func() time.Time
}

// SearchIssuesOption configures optional bounded detail enrichment while
// preserving the candidate-only constructor used by isolated search tests.
type SearchIssuesOption func(*searchIssues) error

// WithIssueRecommendationEnrichment enables detailed analysis for at most
// analysisLimit eligible candidates with bounded parallel GitHub requests.
func WithIssueRecommendationEnrichment(
	recommender IssueRecommender,
	analysisLimit int,
	maxConcurrency int,
) SearchIssuesOption {
	return func(usecase *searchIssues) error {
		if recommender == nil {
			return fmt.Errorf("issue recommender is required")
		}
		if analysisLimit < 1 || analysisLimit > usecase.resultLimit {
			return fmt.Errorf(
				"analysis limit must be between 1 and %d",
				usecase.resultLimit,
			)
		}
		if maxConcurrency < 1 || maxConcurrency > analysisLimit {
			return fmt.Errorf(
				"analysis concurrency must be between 1 and %d",
				analysisLimit,
			)
		}
		usecase.recommender = recommender
		usecase.analysisLimit = analysisLimit
		usecase.maxConcurrency = maxConcurrency
		return nil
	}
}

func NewSearchIssues(
	searcher port.GitHubIssueSearcher,
	cache port.IssueSearchCache,
	resultLimit int,
	options ...SearchIssuesOption,
) (SearchIssues, error) {
	if searcher == nil {
		return nil, fmt.Errorf("compose issue search: GitHub searcher is required")
	}
	if cache == nil {
		return nil, fmt.Errorf("compose issue search: cache is required")
	}
	if resultLimit < 1 || resultLimit > issue.MaximumCandidateResults {
		return nil, fmt.Errorf(
			"compose issue search: result limit must be between 1 and %d",
			issue.MaximumCandidateResults,
		)
	}
	contract := &searchIssues{
		searcher:       searcher,
		cache:          cache,
		resultLimit:    resultLimit,
		maxConcurrency: 1,
		now:            time.Now,
	}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("compose issue search: option is required")
		}
		if err := option(contract); err != nil {
			return nil, fmt.Errorf("compose issue search: %w", err)
		}
	}
	return contract, nil
}

func (usecase *searchIssues) Execute(
	ctx context.Context,
	input SearchIssuesInput,
) (SearchIssuesOutput, error) {
	if err := ctx.Err(); err != nil {
		return SearchIssuesOutput{}, mapIssueSearchError(err)
	}

	key := input.Criteria.CacheKey()
	if cached, found, err := usecase.cache.Get(ctx, key); err == nil && found {
		return usecase.issueSearchOutput(
			ctx,
			cached,
			input,
			true,
		)
	} else if err != nil && ctx.Err() != nil {
		return SearchIssuesOutput{}, mapIssueSearchError(err)
	}

	resultChannel := usecase.requests.DoChan(key, func() (any, error) {
		if cached, found, err := usecase.cache.Get(ctx, key); err == nil && found {
			return cached, nil
		} else if err != nil && ctx.Err() != nil {
			return port.IssueSearchCacheEntry{}, err
		}

		result, err := usecase.searcher.SearchIssues(
			ctx,
			input.Criteria,
			usecase.resultLimit,
		)
		if err != nil {
			return port.IssueSearchCacheEntry{}, err
		}

		entry := filterIssueCandidates(input.Criteria, result, usecase.now())
		_ = usecase.cache.Set(ctx, key, entry)
		return entry, nil
	})

	select {
	case <-ctx.Done():
		return SearchIssuesOutput{}, mapIssueSearchError(ctx.Err())
	case result := <-resultChannel:
		if result.Err != nil {
			return SearchIssuesOutput{}, mapIssueSearchError(result.Err)
		}
		entry, valid := result.Val.(port.IssueSearchCacheEntry)
		if !valid {
			return SearchIssuesOutput{}, apperror.New(
				apperror.CodeInternal,
				"An unexpected error occurred",
				http.StatusInternalServerError,
			)
		}
		return usecase.issueSearchOutput(
			ctx,
			entry,
			input,
			false,
		)
	}
}

func filterIssueCandidates(
	criteria issue.SearchCriteria,
	result port.GitHubIssueSearchResult,
	now time.Time,
) port.IssueSearchCacheEntry {
	candidates := make([]issue.Candidate, 0, len(result.Candidates))
	exclusionCounts := make(map[issue.ExclusionReason]int)
	for _, candidate := range result.Candidates {
		reasons := issue.ExclusionReasons(criteria, candidate, now)
		if len(reasons) == 0 {
			candidates = append(candidates, candidate)
			continue
		}
		for _, reason := range reasons {
			exclusionCounts[reason]++
		}
	}

	return port.IssueSearchCacheEntry{
		Candidates:        candidates,
		ExclusionCounts:   exclusionCounts,
		CandidatesChecked: len(result.Candidates),
		UpstreamTotal:     result.TotalCount,
		IncompleteResults: result.IncompleteResults,
		RateLimit:         result.RateLimit,
	}
}

func (usecase *searchIssues) issueSearchOutput(
	ctx context.Context,
	entry port.IssueSearchCacheEntry,
	input SearchIssuesInput,
	cacheHit bool,
) (SearchIssuesOutput, error) {
	ranked, recommendationMeta, err := usecase.recommendCandidates(
		ctx,
		entry.Candidates,
		input.Criteria,
	)
	if err != nil {
		return SearchIssuesOutput{}, mapIssueSearchError(err)
	}
	total := len(ranked)
	totalPages := 0
	if total > 0 {
		totalPages = (total + input.Pagination.PerPage - 1) /
			input.Pagination.PerPage
	}

	items := make([]issue.RankedIssue, 0)
	pageIndex := input.Pagination.Page - 1
	if total > 0 && pageIndex <= total/input.Pagination.PerPage {
		start := pageIndex * input.Pagination.PerPage
		if start < total {
			end := min(start+input.Pagination.PerPage, total)
			items = append(items, ranked[start:end]...)
		}
	}

	rateLimit := mergeRateLimits(entry.RateLimit, recommendationMeta.rateLimit)
	return SearchIssuesOutput{
		Items: items,
		Pagination: SearchIssuesPagination{
			Page:       input.Pagination.Page,
			PerPage:    input.Pagination.PerPage,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    input.Pagination.Page < totalPages,
		},
		ExclusionCounts:     cloneExclusionCounts(entry.ExclusionCounts),
		CandidatesChecked:   entry.CandidatesChecked,
		UpstreamTotal:       entry.UpstreamTotal,
		EnrichmentAttempted: recommendationMeta.attempted,
		EnrichmentFailed:    recommendationMeta.failed,
		IncompleteResults: entry.IncompleteResults ||
			recommendationMeta.incomplete,
		RateLimit: rateLimit,
		CacheHit:  cacheHit,
	}, nil
}

type issueRecommendationMeta struct {
	attempted  int
	failed     int
	incomplete bool
	rateLimit  port.RateLimit
}

func (usecase *searchIssues) recommendCandidates(
	ctx context.Context,
	candidates []issue.Candidate,
	criteria issue.SearchCriteria,
) ([]issue.RankedIssue, issueRecommendationMeta, error) {
	desiredSkills := desiredIssueSkills(criteria)
	ranked := make([]issue.RankedIssue, len(candidates))
	limit := 0
	if usecase.recommender != nil {
		limit = min(usecase.analysisLimit, len(candidates))
	}
	meta := issueRecommendationMeta{attempted: limit}
	detailOutputs := make([]RecommendIssueOutput, limit)
	detailErrors := make([]error, limit)

	group, groupContext := errgroup.WithContext(ctx)
	group.SetLimit(usecase.maxConcurrency)
	for index := range limit {
		index := index
		group.Go(func() error {
			candidate := candidates[index]
			reference, err := issue.NewReference(
				candidate.Repository.Owner,
				candidate.Repository.Name,
				candidate.Issue.Number,
			)
			if err != nil {
				detailErrors[index] = err
				return nil
			}
			output, err := usecase.recommender.Execute(
				groupContext,
				RecommendIssueInput{
					Reference:     reference,
					DesiredSkills: desiredSkills,
				},
			)
			if err != nil {
				if groupContext.Err() != nil {
					return groupContext.Err()
				}
				detailErrors[index] = err
				return nil
			}
			detailOutputs[index] = output
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, issueRecommendationMeta{}, err
	}

	for index, candidate := range candidates {
		if index < limit && detailErrors[index] == nil {
			output := detailOutputs[index]
			ranked[index] = output.Item
			meta.incomplete = meta.incomplete || output.Incomplete
			meta.rateLimit = mergeRateLimits(meta.rateLimit, output.RateLimit)
			continue
		}
		ranked[index] = fallbackRecommendation(
			usecase,
			candidate,
			desiredSkills,
			index < limit,
		)
		if index < limit {
			meta.failed++
			meta.incomplete = true
		}
	}
	return issue.RankIssues(ranked), meta, nil
}

func fallbackRecommendation(
	recommender *searchIssues,
	candidate issue.Candidate,
	desiredSkills []string,
	enrichmentFailed bool,
) issue.RankedIssue {
	var ranked issue.RankedIssue
	if recommender.recommender != nil {
		ranked = recommender.recommender.EvaluateCandidate(
			candidate,
			desiredSkills,
		)
	} else {
		ranked = evaluateIssueRecommendation(
			candidate,
			nil,
			issue.ActivityMetrics{
				LastMeaningfulUpdate: candidate.Repository.UpdatedAt,
				CI:                   issue.CIStateUnknown,
			},
			issue.DetectClaim(nil, true),
			desiredSkills,
			recommender.now(),
		)
	}
	if enrichmentFailed {
		ranked.Recommendation.Warnings = append(
			ranked.Recommendation.Warnings,
			issue.Warning{
				Code:     "detail_enrichment_unavailable",
				Severity: issue.SeverityInfo,
				Message: "Detailed repository inspection was unavailable; " +
					"the score uses bounded candidate metadata",
				Evidence: []issue.Evidence{{
					RuleID:      "recommendation.detail.unavailable",
					Source:      issue.EvidenceDerived,
					Description: "bounded detail enrichment did not complete",
				}},
			},
		)
	}
	return ranked
}

func desiredIssueSkills(criteria issue.SearchCriteria) []string {
	languages := criteria.Languages()
	frameworks := criteria.Frameworks()
	skills := make([]string, 0, len(languages)+len(frameworks))
	for _, language := range languages {
		skills = append(skills, language.String())
	}
	for _, framework := range frameworks {
		skills = append(skills, framework.String())
	}
	return skills
}

func cloneExclusionCounts(
	counts map[issue.ExclusionReason]int,
) map[issue.ExclusionReason]int {
	cloned := make(map[issue.ExclusionReason]int, len(counts))
	for reason, count := range counts {
		cloned[reason] = count
	}
	return cloned
}

func mapIssueSearchError(err error) error {
	switch {
	case errors.Is(err, issue.ErrInvalidSearchCriteria):
		return apperror.Wrap(
			apperror.CodeInvalidRequest,
			"Issue search criteria are invalid",
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
			"Unable to search GitHub issues",
			http.StatusBadGateway,
			err,
		)
	}
}

var _ SearchIssues = (*searchIssues)(nil)
