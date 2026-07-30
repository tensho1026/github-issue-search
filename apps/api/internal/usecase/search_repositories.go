package usecase

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

type SearchRepositoriesInput struct {
	Criteria   repository.DiscoveryCriteria
	Pagination repository.DiscoveryPagination
}

type SearchRepositoriesPagination struct {
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	HasNext    bool
}

type RepositoryDiscoveryWarning string

const (
	RepositoryDiscoveryWarningGitHubIncomplete     RepositoryDiscoveryWarning = "github_results_incomplete"
	RepositoryDiscoveryWarningEnrichmentIncomplete RepositoryDiscoveryWarning = "enrichment_incomplete"
)

type SearchRepositoriesOutput struct {
	Items                []repository.DiscoveryResult
	Pagination           SearchRepositoriesPagination
	CandidatesChecked    int
	UpstreamTotal        int
	EnrichmentAttempted  int
	EnrichmentFailed     int
	GitHubIncomplete     bool
	EnrichmentIncomplete bool
	Warnings             []RepositoryDiscoveryWarning
	RateLimit            port.RateLimit
	CacheHit             bool
}

type SearchRepositories interface {
	Execute(
		ctx context.Context,
		input SearchRepositoriesInput,
	) (SearchRepositoriesOutput, error)
}

type searchRepositories struct {
	searcher        port.GitHubRepositoryDiscoverySearcher
	enricher        port.GitHubRepositoryDiscoveryEnricher
	cache           port.RepositoryDiscoveryCache
	resultLimit     int
	enrichmentLimit int
	requests        singleflight.Group
	now             func() time.Time
}

func NewSearchRepositories(
	searcher port.GitHubRepositoryDiscoverySearcher,
	enricher port.GitHubRepositoryDiscoveryEnricher,
	cache port.RepositoryDiscoveryCache,
	resultLimit int,
	enrichmentLimit int,
) (SearchRepositories, error) {
	if searcher == nil {
		return nil, fmt.Errorf(
			"compose repository discovery: GitHub searcher is required",
		)
	}
	if enricher == nil {
		return nil, fmt.Errorf(
			"compose repository discovery: GitHub enricher is required",
		)
	}
	if cache == nil {
		return nil, fmt.Errorf(
			"compose repository discovery: cache is required",
		)
	}
	if resultLimit < 1 ||
		resultLimit > repository.MaximumDiscoveryCandidateResults {
		return nil, fmt.Errorf(
			"compose repository discovery: result limit must be between 1 and %d",
			repository.MaximumDiscoveryCandidateResults,
		)
	}
	if enrichmentLimit < 1 ||
		enrichmentLimit > resultLimit ||
		enrichmentLimit > repository.MaximumDiscoveryEnrichmentResults {
		return nil, fmt.Errorf(
			"compose repository discovery: enrichment limit must be between 1 and %d",
			min(resultLimit, repository.MaximumDiscoveryEnrichmentResults),
		)
	}
	return &searchRepositories{
		searcher:        searcher,
		enricher:        enricher,
		cache:           cache,
		resultLimit:     resultLimit,
		enrichmentLimit: enrichmentLimit,
		now:             time.Now,
	}, nil
}

func (usecase *searchRepositories) Execute(
	ctx context.Context,
	input SearchRepositoriesInput,
) (SearchRepositoriesOutput, error) {
	if err := ctx.Err(); err != nil {
		return SearchRepositoriesOutput{}, mapRepositoryDiscoveryError(err)
	}

	key := input.Criteria.CacheKey()
	if cached, found, err := usecase.cache.Get(ctx, key); err == nil && found {
		return repositoryDiscoveryOutput(cached, input.Pagination, true), nil
	} else if err != nil && ctx.Err() != nil {
		return SearchRepositoriesOutput{}, mapRepositoryDiscoveryError(err)
	}

	resultChannel := usecase.requests.DoChan(key, func() (any, error) {
		if cached, found, err := usecase.cache.Get(ctx, key); err == nil && found {
			return cached, nil
		} else if err != nil && ctx.Err() != nil {
			return port.RepositoryDiscoveryCacheEntry{}, err
		}

		entry, err := usecase.loadRepositoryDiscovery(ctx, input.Criteria)
		if err != nil {
			return port.RepositoryDiscoveryCacheEntry{}, err
		}
		_ = usecase.cache.Set(ctx, key, entry)
		return entry, nil
	})

	select {
	case <-ctx.Done():
		return SearchRepositoriesOutput{}, mapRepositoryDiscoveryError(ctx.Err())
	case result := <-resultChannel:
		if result.Err != nil {
			return SearchRepositoriesOutput{},
				mapRepositoryDiscoveryError(result.Err)
		}
		entry, valid := result.Val.(port.RepositoryDiscoveryCacheEntry)
		if !valid {
			return SearchRepositoriesOutput{}, apperror.New(
				apperror.CodeInternal,
				"An unexpected error occurred",
				http.StatusInternalServerError,
			)
		}
		return repositoryDiscoveryOutput(entry, input.Pagination, false), nil
	}
}

func (usecase *searchRepositories) loadRepositoryDiscovery(
	ctx context.Context,
	criteria repository.DiscoveryCriteria,
) (port.RepositoryDiscoveryCacheEntry, error) {
	searchResult, err := usecase.searcher.SearchRepositories(
		ctx,
		criteria,
		usecase.resultLimit,
	)
	if err != nil {
		return port.RepositoryDiscoveryCacheEntry{}, err
	}

	now := usecase.now().UTC()
	candidates := prefilterDiscoveryCandidates(
		searchResult.Candidates,
		criteria,
		now,
	)
	if len(candidates) > usecase.enrichmentLimit {
		candidates = candidates[:usecase.enrichmentLimit]
	}

	summaries := make([]repository.Summary, len(candidates))
	for index, candidate := range candidates {
		summaries[index] = candidate.Repository
	}

	enrichment := port.GitHubRepositoryEnrichmentResult{
		Items: make(map[string]repository.DiscoveryEnrichment),
	}
	enrichmentFailed := 0
	if len(summaries) > 0 {
		enrichment, err = usecase.enricher.EnrichRepositories(ctx, summaries)
		if err != nil {
			if ctx.Err() != nil {
				return port.RepositoryDiscoveryCacheEntry{}, ctx.Err()
			}
			enrichment = port.GitHubRepositoryEnrichmentResult{
				Items:             make(map[string]repository.DiscoveryEnrichment),
				IncompleteResults: true,
			}
			enrichmentFailed = len(summaries)
		}
	}

	results := make([]repository.DiscoveryResult, 0, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.Repository.FullName)
		evidence, found := enrichment.Items[key]
		if !found {
			evidence = repository.DiscoveryEnrichment{}
			if err == nil {
				enrichmentFailed++
			}
		}
		result := repository.AnalyzeDiscovery(
			candidate,
			evidence,
			criteria.Technologies(),
			now,
		)
		if matchesAnalyzedDiscovery(result, criteria) {
			results = append(results, result)
		}
	}
	repository.SortDiscoveryResults(results)

	return port.RepositoryDiscoveryCacheEntry{
		Items:               results,
		CandidatesChecked:   len(searchResult.Candidates),
		UpstreamTotal:       searchResult.TotalCount,
		SearchIncomplete:    searchResult.IncompleteResults,
		EnrichmentAttempted: len(candidates),
		EnrichmentFailed:    enrichmentFailed,
		EnrichmentIncomplete: enrichment.IncompleteResults ||
			enrichmentFailed > 0,
		RateLimit: mergeRateLimits(
			searchResult.RateLimit,
			enrichment.RateLimit,
		),
	}, nil
}

func prefilterDiscoveryCandidates(
	source []repository.DiscoveryCandidate,
	criteria repository.DiscoveryCriteria,
	now time.Time,
) []repository.DiscoveryCandidate {
	candidates := make([]repository.DiscoveryCandidate, 0, len(source))
	cutoff := now.AddDate(0, 0, -criteria.UpdatedWithinDays())
	for _, candidate := range source {
		summary := candidate.Repository
		if criteria.ExcludesArchived() && summary.IsArchived {
			continue
		}
		switch criteria.ForkPolicy() {
		case repository.ForkPolicyExclude:
			if summary.IsFork {
				continue
			}
		case repository.ForkPolicyOnly:
			if !summary.IsFork {
				continue
			}
		}
		if summary.Stars < criteria.MinimumStars() ||
			summary.Forks < criteria.MinimumForks() ||
			summary.OpenIssues < criteria.MinimumOpenIssues() ||
			summary.PushedAt.Before(cutoff) {
			continue
		}
		if maximum, configured := criteria.MaximumOpenIssues(); configured &&
			summary.OpenIssues > maximum {
			continue
		}
		if !matchesLanguage(summary.MainLanguage, criteria.Languages()) ||
			!matchesLicense(candidate, criteria.Licenses()) {
			continue
		}
		candidates = append(candidates, candidate)
	}

	slices.SortStableFunc(
		candidates,
		func(left, right repository.DiscoveryCandidate) int {
			if order := cmp.Compare(
				right.Repository.Stars,
				left.Repository.Stars,
			); order != 0 {
				return order
			}
			if order := right.Repository.PushedAt.Compare(
				left.Repository.PushedAt,
			); order != 0 {
				return order
			}
			return strings.Compare(
				strings.ToLower(left.Repository.FullName),
				strings.ToLower(right.Repository.FullName),
			)
		},
	)
	return candidates
}

func matchesAnalyzedDiscovery(
	result repository.DiscoveryResult,
	criteria repository.DiscoveryCriteria,
) bool {
	if !containsCategory(criteria.Categories(), result.Category) ||
		(len(criteria.Technologies()) > 0 && len(result.Technologies) == 0) ||
		result.Difficulty.Level > criteria.MaximumDifficulty() ||
		result.Readiness.Score < criteria.MinimumReadiness() {
		return false
	}
	if expected, configured := criteria.HasJapaneseREADME(); configured {
		evidence := result.Documentation.JapaneseREADME
		if evidence.Status == repository.EvidenceUnavailable ||
			evidence.Detected != expected {
			return false
		}
	}
	return true
}

func matchesLanguage(
	actual string,
	expected []repository.FilterValue,
) bool {
	if len(expected) == 0 {
		return true
	}
	for _, language := range expected {
		if strings.EqualFold(actual, language.String()) {
			return true
		}
	}
	return false
}

func matchesLicense(
	candidate repository.DiscoveryCandidate,
	expected []repository.SPDXLicense,
) bool {
	if len(expected) == 0 {
		return true
	}
	if !candidate.LicenseKnown {
		return false
	}
	return slices.Contains(expected, candidate.License)
}

func containsCategory(
	expected []repository.Category,
	actual repository.Category,
) bool {
	return len(expected) == 0 || slices.Contains(expected, actual)
}

func repositoryDiscoveryOutput(
	entry port.RepositoryDiscoveryCacheEntry,
	pagination repository.DiscoveryPagination,
	cacheHit bool,
) SearchRepositoriesOutput {
	total := len(entry.Items)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pagination.PerPage - 1) / pagination.PerPage
	}

	items := make([]repository.DiscoveryResult, 0)
	start := (pagination.Page - 1) * pagination.PerPage
	if start < total {
		end := min(start+pagination.PerPage, total)
		items = append(items, entry.Items[start:end]...)
	}

	warnings := make([]RepositoryDiscoveryWarning, 0, 2)
	if entry.SearchIncomplete {
		warnings = append(
			warnings,
			RepositoryDiscoveryWarningGitHubIncomplete,
		)
	}
	if entry.EnrichmentIncomplete {
		warnings = append(
			warnings,
			RepositoryDiscoveryWarningEnrichmentIncomplete,
		)
	}

	return SearchRepositoriesOutput{
		Items: items,
		Pagination: SearchRepositoriesPagination{
			Page:       pagination.Page,
			PerPage:    pagination.PerPage,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    pagination.Page < totalPages,
		},
		CandidatesChecked:    entry.CandidatesChecked,
		UpstreamTotal:        entry.UpstreamTotal,
		EnrichmentAttempted:  entry.EnrichmentAttempted,
		EnrichmentFailed:     entry.EnrichmentFailed,
		GitHubIncomplete:     entry.SearchIncomplete,
		EnrichmentIncomplete: entry.EnrichmentIncomplete,
		Warnings:             warnings,
		RateLimit:            entry.RateLimit,
		CacheHit:             cacheHit,
	}
}

func mapRepositoryDiscoveryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidDiscoveryCriteria):
		return apperror.Wrap(
			apperror.CodeInvalidRequest,
			"Repository discovery criteria are invalid",
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
			"Unable to search GitHub repositories",
			http.StatusBadGateway,
			err,
		)
	}
}

var _ SearchRepositories = (*searchRepositories)(nil)
