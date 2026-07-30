package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestSearchRepositoriesFiltersEnrichesSortsPaginatesAndCaches(
	t *testing.T,
) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	searcher := &repositoryDiscoverySearcherStub{
		result: port.GitHubRepositoryDiscoveryResult{
			Candidates: []repository.DiscoveryCandidate{
				repositoryDiscoveryCandidate("alpha/ready", 500, now),
				repositoryDiscoveryCandidate("beta/english", 300, now),
				repositoryDiscoveryCandidate("gamma/low-star", 1, now),
			},
			TotalCount:        80,
			IncompleteResults: true,
			RateLimit:         port.RateLimit{Known: true, Remaining: 90},
		},
	}
	enricher := &repositoryDiscoveryEnricherStub{
		result: port.GitHubRepositoryEnrichmentResult{
			Items: map[string]repository.DiscoveryEnrichment{
				"alpha/ready": {
					Available:              true,
					READMEAvailable:        true,
					READMEContentAvailable: true,
					READMEText:             "React " + strings.Repeat("日本語の説明", 30),
					ContributingAvailable:  true,
				},
				"beta/english": {
					Available:              true,
					READMEAvailable:        true,
					READMEContentAvailable: true,
					READMEText:             "React documentation in English",
				},
			},
			RateLimit: port.RateLimit{Known: true, Remaining: 88},
		},
	}
	cache, err := memory.NewRepositoryDiscovery(10, time.Hour)
	if err != nil {
		t.Fatalf("NewRepositoryDiscovery() error = %v", err)
	}
	contract, err := NewSearchRepositories(searcher, enricher, cache, 3, 2)
	if err != nil {
		t.Fatalf("NewSearchRepositories() error = %v", err)
	}
	implementation, ok := contract.(*searchRepositories)
	if !ok {
		t.Fatalf("NewSearchRepositories() type = %T", contract)
	}
	nowCalls := 0
	implementation.now = func() time.Time {
		nowCalls++
		return now
	}
	hasJapanese := true
	minimumStars := 10
	criteria, err := repository.NewDiscoveryCriteria(
		repository.DiscoveryCriteriaOptions{
			Technologies:      []string{"React"},
			MinimumStars:      &minimumStars,
			HasJapaneseREADME: &hasJapanese,
		},
	)
	if err != nil {
		t.Fatalf("NewDiscoveryCriteria() error = %v", err)
	}
	pagination, _ := repository.NewDiscoveryPagination(1, 1)

	first, err := contract.Execute(
		context.Background(),
		SearchRepositoriesInput{
			Criteria:   criteria,
			Pagination: pagination,
		},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(first.Items) != 1 ||
		first.Items[0].Repository.FullName != "alpha/ready" {
		t.Fatalf("Execute().Items = %#v", first.Items)
	}
	if first.CandidatesChecked != 3 ||
		first.EnrichmentAttempted != 2 ||
		first.UpstreamTotal != 80 ||
		!first.GitHubIncomplete ||
		first.RateLimit.Remaining != 88 {
		t.Fatalf("Execute() metadata = %+v", first)
	}
	if first.CacheHit {
		t.Fatal("first Execute().CacheHit = true")
	}

	second, err := contract.Execute(
		context.Background(),
		SearchRepositoriesInput{
			Criteria:   criteria,
			Pagination: pagination,
		},
	)
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if !second.CacheHit ||
		searcher.calls != 1 ||
		enricher.calls != 1 ||
		nowCalls != 1 {
		t.Fatalf(
			"cache hit = %t, search calls = %d, enrichment calls = %d, now calls = %d",
			second.CacheHit,
			searcher.calls,
			enricher.calls,
			nowCalls,
		)
	}
}

func TestSearchRepositoriesDegradesOptionalEnrichmentFailure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	searcher := &repositoryDiscoverySearcherStub{
		result: port.GitHubRepositoryDiscoveryResult{
			Candidates: []repository.DiscoveryCandidate{
				repositoryDiscoveryCandidate("alpha/repo", 100, now),
			},
		},
	}
	enricher := &repositoryDiscoveryEnricherStub{
		err: &port.GitHubError{Kind: port.GitHubErrorUpstream},
	}
	cache, _ := memory.NewRepositoryDiscovery(2, time.Hour)
	contract, err := NewSearchRepositories(searcher, enricher, cache, 1, 1)
	if err != nil {
		t.Fatalf("NewSearchRepositories() error = %v", err)
	}
	implementation, ok := contract.(*searchRepositories)
	if !ok {
		t.Fatalf("NewSearchRepositories() type = %T", contract)
	}
	implementation.now = func() time.Time { return now }
	criteria, _ := repository.NewDiscoveryCriteria(
		repository.DiscoveryCriteriaOptions{},
	)
	pagination, _ := repository.NewDiscoveryPagination(1, 20)

	output, err := contract.Execute(
		context.Background(),
		SearchRepositoriesInput{Criteria: criteria, Pagination: pagination},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(output.Items) != 1 ||
		output.Items[0].Documentation.Status != repository.EvidenceUnavailable ||
		!output.EnrichmentIncomplete ||
		output.EnrichmentFailed != 1 ||
		len(output.Warnings) != 1 {
		t.Fatalf("Execute() output = %+v", output)
	}
}

func TestSearchRepositoriesMapsSearchFailureAndCancellation(t *testing.T) {
	t.Parallel()

	cache, _ := memory.NewRepositoryDiscovery(2, time.Hour)
	upstream := &repositoryDiscoverySearcherStub{
		err: &port.GitHubError{Kind: port.GitHubErrorRateLimited},
	}
	contract, err := NewSearchRepositories(
		upstream,
		&repositoryDiscoveryEnricherStub{},
		cache,
		1,
		1,
	)
	if err != nil {
		t.Fatalf("NewSearchRepositories() error = %v", err)
	}
	criteria, _ := repository.NewDiscoveryCriteria(
		repository.DiscoveryCriteriaOptions{},
	)
	pagination, _ := repository.NewDiscoveryPagination(1, 20)
	_, err = contract.Execute(
		context.Background(),
		SearchRepositoriesInput{Criteria: criteria, Pagination: pagination},
	)
	var appError *apperror.Error
	if !errors.As(err, &appError) ||
		appError.Code != apperror.CodeRateLimit {
		t.Fatalf("Execute() error = %v, want rate-limit app error", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = contract.Execute(
		cancelled,
		SearchRepositoriesInput{Criteria: criteria, Pagination: pagination},
	)
	if !errors.As(err, &appError) ||
		appError.Code != apperror.CodeRequestTimeout {
		t.Fatalf("cancelled Execute() error = %v", err)
	}
}

func TestNewSearchRepositoriesValidatesDependenciesAndBounds(t *testing.T) {
	t.Parallel()

	searcher := &repositoryDiscoverySearcherStub{}
	enricher := &repositoryDiscoveryEnricherStub{}
	cache, _ := memory.NewRepositoryDiscovery(1, time.Hour)
	cases := []struct {
		name       string
		searcher   port.GitHubRepositoryDiscoverySearcher
		enricher   port.GitHubRepositoryDiscoveryEnricher
		cache      port.RepositoryDiscoveryCache
		result     int
		enrichment int
	}{
		{"searcher", nil, enricher, cache, 1, 1},
		{"enricher", searcher, nil, cache, 1, 1},
		{"cache", searcher, enricher, nil, 1, 1},
		{"result", searcher, enricher, cache, 0, 1},
		{"enrichment", searcher, enricher, cache, 1, 2},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewSearchRepositories(
				testCase.searcher,
				testCase.enricher,
				testCase.cache,
				testCase.result,
				testCase.enrichment,
			); err == nil {
				t.Fatal("NewSearchRepositories() error = nil")
			}
		})
	}
}

func TestPrefilterDiscoveryCandidatesAppliesCheapFilters(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	minimumStars := 501
	minimumForks := 21
	minimumOpenIssues := 11
	maximumOpenIssues := 9
	updatedWithinDays := 1
	forkOnly := "only"
	cases := []struct {
		name    string
		options repository.DiscoveryCriteriaOptions
		mutate  func(*repository.DiscoveryCandidate)
	}{
		{
			name: "minimum stars",
			options: repository.DiscoveryCriteriaOptions{
				MinimumStars: &minimumStars,
			},
		},
		{
			name: "minimum forks",
			options: repository.DiscoveryCriteriaOptions{
				MinimumForks: &minimumForks,
			},
		},
		{
			name: "minimum open issues",
			options: repository.DiscoveryCriteriaOptions{
				MinimumOpenIssues: &minimumOpenIssues,
			},
		},
		{
			name: "maximum open issues",
			options: repository.DiscoveryCriteriaOptions{
				MaximumOpenIssues: &maximumOpenIssues,
			},
		},
		{
			name: "activity cutoff",
			options: repository.DiscoveryCriteriaOptions{
				UpdatedWithinDays: &updatedWithinDays,
			},
			mutate: func(candidate *repository.DiscoveryCandidate) {
				candidate.Repository.PushedAt = now.Add(-48 * time.Hour)
			},
		},
		{
			name: "language",
			options: repository.DiscoveryCriteriaOptions{
				Languages: []string{"Go"},
			},
		},
		{
			name: "license",
			options: repository.DiscoveryCriteriaOptions{
				Licenses: []string{"Apache-2.0"},
			},
		},
		{
			name: "fork policy",
			options: repository.DiscoveryCriteriaOptions{
				ForkPolicy: &forkOnly,
			},
		},
		{
			name: "archive policy",
			mutate: func(candidate *repository.DiscoveryCandidate) {
				candidate.Repository.IsArchived = true
			},
		},
		{
			name: "category before enrichment",
			options: repository.DiscoveryCriteriaOptions{
				Categories: []string{"security"},
			},
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			candidate := repositoryDiscoveryCandidate("example/repo", 500, now)
			if testCase.mutate != nil {
				testCase.mutate(&candidate)
			}
			criteria, err := repository.NewDiscoveryCriteria(testCase.options)
			if err != nil {
				t.Fatalf("NewDiscoveryCriteria() error = %v", err)
			}
			if got := prefilterDiscoveryCandidates(
				[]repository.DiscoveryCandidate{candidate},
				criteria,
				now,
			); len(got) != 0 {
				t.Fatalf("prefilterDiscoveryCandidates() = %#v, want empty", got)
			}
		})
	}
}

func TestMatchesAnalyzedDiscoveryAppliesEnrichedFilters(t *testing.T) {
	t.Parallel()

	maximumDifficulty := 3
	minimumReadiness := 60
	hasJapaneseREADME := true
	matchingOptions := repository.DiscoveryCriteriaOptions{
		Technologies:      []string{"React"},
		Categories:        []string{"tooling"},
		MaximumDifficulty: &maximumDifficulty,
		MinimumReadiness:  &minimumReadiness,
		HasJapaneseREADME: &hasJapaneseREADME,
	}
	result := repository.DiscoveryResult{
		Category:     repository.CategoryTooling,
		Technologies: []string{"React"},
		Difficulty:   repository.PreliminaryDifficulty{Level: 3},
		Readiness:    repository.ContributionReadiness{Score: 60},
		Documentation: repository.DocumentationSignals{
			JapaneseREADME: repository.JapaneseREADMEEvidence{
				Detected: true,
				Status:   repository.EvidenceExact,
			},
		},
	}
	criteria, err := repository.NewDiscoveryCriteria(matchingOptions)
	if err != nil {
		t.Fatalf("NewDiscoveryCriteria() error = %v", err)
	}
	if !matchesAnalyzedDiscovery(result, criteria) {
		t.Fatal("matchesAnalyzedDiscovery() = false for matching evidence")
	}

	cases := []struct {
		name   string
		mutate func(*repository.DiscoveryResult)
	}{
		{
			name: "technology",
			mutate: func(value *repository.DiscoveryResult) {
				value.Technologies = []string{}
			},
		},
		{
			name: "category",
			mutate: func(value *repository.DiscoveryResult) {
				value.Category = repository.CategoryLibrary
			},
		},
		{
			name: "difficulty",
			mutate: func(value *repository.DiscoveryResult) {
				value.Difficulty.Level = 4
			},
		},
		{
			name: "readiness",
			mutate: func(value *repository.DiscoveryResult) {
				value.Readiness.Score = 59
			},
		},
		{
			name: "Japanese README",
			mutate: func(value *repository.DiscoveryResult) {
				value.Documentation.JapaneseREADME.Detected = false
			},
		},
		{
			name: "unavailable Japanese README",
			mutate: func(value *repository.DiscoveryResult) {
				value.Documentation.JapaneseREADME.Status =
					repository.EvidenceUnavailable
			},
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			changed := result
			changed.Technologies = append([]string(nil), result.Technologies...)
			testCase.mutate(&changed)
			if matchesAnalyzedDiscovery(changed, criteria) {
				t.Fatal("matchesAnalyzedDiscovery() = true, want false")
			}
		})
	}
}

type repositoryDiscoverySearcherStub struct {
	mu     sync.Mutex
	result port.GitHubRepositoryDiscoveryResult
	err    error
	calls  int
}

func (stub *repositoryDiscoverySearcherStub) SearchRepositories(
	ctx context.Context,
	_ repository.DiscoveryCriteria,
	_ int,
) (port.GitHubRepositoryDiscoveryResult, error) {
	if err := ctx.Err(); err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, err
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.calls++
	return stub.result, stub.err
}

type repositoryDiscoveryEnricherStub struct {
	mu     sync.Mutex
	result port.GitHubRepositoryEnrichmentResult
	err    error
	calls  int
}

func (stub *repositoryDiscoveryEnricherStub) EnrichRepositories(
	ctx context.Context,
	_ []repository.Summary,
) (port.GitHubRepositoryEnrichmentResult, error) {
	if err := ctx.Err(); err != nil {
		return port.GitHubRepositoryEnrichmentResult{}, err
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.calls++
	return stub.result, stub.err
}

func repositoryDiscoveryCandidate(
	fullName string,
	stars int,
	now time.Time,
) repository.DiscoveryCandidate {
	parts := strings.SplitN(fullName, "/", 2)
	return repository.DiscoveryCandidate{
		Repository: repository.Summary{
			ID:           int64(stars),
			Owner:        parts[0],
			Name:         parts[1],
			FullName:     fullName,
			Description:  "developer tool",
			URL:          "https://github.com/" + fullName,
			MainLanguage: "TypeScript",
			Stars:        stars,
			Forks:        20,
			OpenIssues:   10,
			UpdatedAt:    now.Add(-24 * time.Hour),
			PushedAt:     now.Add(-24 * time.Hour),
		},
		Topics:           []string{"react", "developer-tools"},
		License:          "MIT",
		LicenseKnown:     true,
		GoodFirstIssues:  4,
		HelpWantedIssues: 2,
		HasIssuesEnabled: true,
		HasDiscussions:   true,
		HasCodeOfConduct: true,
	}
}
