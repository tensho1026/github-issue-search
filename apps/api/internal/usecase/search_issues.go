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
	Items             []issue.Candidate
	Pagination        SearchIssuesPagination
	ExclusionCounts   map[issue.ExclusionReason]int
	CandidatesChecked int
	UpstreamTotal     int
	IncompleteResults bool
	RateLimit         port.RateLimit
	CacheHit          bool
}

type SearchIssues interface {
	Execute(
		ctx context.Context,
		input SearchIssuesInput,
	) (SearchIssuesOutput, error)
}

type searchIssues struct {
	searcher    port.GitHubIssueSearcher
	cache       port.IssueSearchCache
	resultLimit int
	requests    singleflight.Group
	now         func() time.Time
}

func NewSearchIssues(
	searcher port.GitHubIssueSearcher,
	cache port.IssueSearchCache,
	resultLimit int,
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
	return &searchIssues{
		searcher:    searcher,
		cache:       cache,
		resultLimit: resultLimit,
		now:         time.Now,
	}, nil
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
		return issueSearchOutput(cached, input.Pagination, true), nil
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
		return issueSearchOutput(entry, input.Pagination, false), nil
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

func issueSearchOutput(
	entry port.IssueSearchCacheEntry,
	requested issue.Pagination,
	cacheHit bool,
) SearchIssuesOutput {
	total := len(entry.Candidates)
	totalPages := 0
	if total > 0 {
		totalPages = (total + requested.PerPage - 1) / requested.PerPage
	}

	items := make([]issue.Candidate, 0)
	pageIndex := requested.Page - 1
	if total > 0 && pageIndex <= total/requested.PerPage {
		start := pageIndex * requested.PerPage
		if start < total {
			end := min(start+requested.PerPage, total)
			items = cloneCandidates(entry.Candidates[start:end])
		}
	}

	return SearchIssuesOutput{
		Items: items,
		Pagination: SearchIssuesPagination{
			Page:       requested.Page,
			PerPage:    requested.PerPage,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    requested.Page < totalPages,
		},
		ExclusionCounts:   cloneExclusionCounts(entry.ExclusionCounts),
		CandidatesChecked: entry.CandidatesChecked,
		UpstreamTotal:     entry.UpstreamTotal,
		IncompleteResults: entry.IncompleteResults,
		RateLimit:         entry.RateLimit,
		CacheHit:          cacheHit,
	}
}

func cloneCandidates(candidates []issue.Candidate) []issue.Candidate {
	cloned := make([]issue.Candidate, len(candidates))
	for index, candidate := range candidates {
		cloned[index] = candidate
		cloned[index].Issue.Labels = append(
			[]string(nil),
			candidate.Issue.Labels...,
		)
		cloned[index].Issue.Assignees = append(
			[]string(nil),
			candidate.Issue.Assignees...,
		)
	}
	return cloned
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
