package usecase

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestSearchIssuesFiltersPaginatesAndCachesCandidates(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	eligibleFirst := searchCandidate(now, 1, 120)
	eligibleSecond := searchCandidate(now, 2, 80)
	assigned := searchCandidate(now, 3, 120)
	assigned.Issue.Assignees = []string{"maintainer"}
	lowStars := searchCandidate(now, 4, 5)
	searcher := &issueSearcherStub{
		result: port.GitHubIssueSearchResult{
			Candidates: []issue.Candidate{
				eligibleFirst,
				assigned,
				eligibleSecond,
				lowStars,
			},
			TotalCount:        900,
			IncompleteResults: true,
			RateLimit:         port.RateLimit{Known: true, Remaining: 27},
		},
	}
	usecase := newIssueSearchUsecase(t, searcher, now)
	criteria := searchCriteria(t, issue.SearchCriteriaOptions{Username: "octocat"})

	first, err := usecase.Execute(context.Background(), SearchIssuesInput{
		Criteria:   criteria,
		Pagination: searchPagination(t, 1, 1),
	})
	if err != nil {
		t.Fatalf("Execute(first) error = %v", err)
	}
	if len(first.Items) != 1 ||
		first.Items[0].Candidate.Issue.Number != 1 ||
		first.Pagination != (SearchIssuesPagination{
			Page:       1,
			PerPage:    1,
			Total:      2,
			TotalPages: 2,
			HasNext:    true,
		}) ||
		first.CandidatesChecked != 4 ||
		first.UpstreamTotal != 900 ||
		!first.GitHubIncomplete ||
		first.EnrichmentIncomplete ||
		first.RateLimit.Remaining != 27 ||
		first.CacheHit {
		t.Fatalf("first output = %+v", first)
	}
	if first.ExclusionCounts[issue.ExclusionAlreadyAssigned] != 1 ||
		first.ExclusionCounts[issue.ExclusionBelowMinimumStars] != 1 {
		t.Fatalf("exclusions = %+v", first.ExclusionCounts)
	}

	second, err := usecase.Execute(context.Background(), SearchIssuesInput{
		Criteria:   criteria,
		Pagination: searchPagination(t, 2, 1),
	})
	if err != nil {
		t.Fatalf("Execute(second) error = %v", err)
	}
	if len(second.Items) != 1 ||
		second.Items[0].Candidate.Issue.Number != 2 ||
		second.Pagination.HasNext ||
		!second.CacheHit ||
		searcher.callCount() != 1 {
		t.Fatalf("second output = %+v, calls = %d", second, searcher.callCount())
	}

	second.Items[0].Candidate.Issue.Labels[0] = "mutated"
	third, err := usecase.Execute(context.Background(), SearchIssuesInput{
		Criteria:   criteria,
		Pagination: searchPagination(t, 2, 1),
	})
	if err != nil {
		t.Fatalf("Execute(third) error = %v", err)
	}
	if third.Items[0].Candidate.Issue.Labels[0] != "good first issue" {
		t.Fatalf("cached output was mutated = %+v", third.Items[0])
	}
}

func TestSearchIssuesCanonicalCriteriaShareCache(t *testing.T) {
	now := time.Now().UTC()
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{searchCandidate(now, 1, 20)},
		TotalCount: 1,
	}}
	usecase := newIssueSearchUsecase(t, searcher, now)
	first := searchCriteria(t, issue.SearchCriteriaOptions{
		Username:  "OctoCat",
		Languages: []string{"TypeScript", "Go"},
	})
	second := searchCriteria(t, issue.SearchCriteriaOptions{
		Username:  "octocat",
		Languages: []string{"go", "typescript"},
	})
	input := SearchIssuesInput{
		Criteria:   first,
		Pagination: searchPagination(t, 1, 20),
	}
	if _, err := usecase.Execute(context.Background(), input); err != nil {
		t.Fatalf("Execute(first) error = %v", err)
	}
	input.Criteria = second
	output, err := usecase.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute(second) error = %v", err)
	}
	if searcher.callCount() != 1 || !output.CacheHit {
		t.Fatalf("calls = %d, cache hit = %t", searcher.callCount(), output.CacheHit)
	}
}

func TestSearchIssuesFiltersByEffortBeforePaginationAndReusesDiscoveryCache(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{
			searchCandidate(now, 1, 30),
			searchCandidate(now, 2, 20),
			searchCandidate(now, 3, 10),
		},
		TotalCount: 3,
	}}
	for index := range searcher.result.Candidates {
		name := "repo-" + strconv.Itoa(index+1)
		searcher.result.Candidates[index].Repository.Name = name
		searcher.result.Candidates[index].Repository.FullName = "example/" + name
	}
	recommender := &searchRecommenderStub{
		now:    now,
		scores: map[int]int{1: 90, 2: 80, 3: 70},
		efforts: map[int]issue.EffortBand{
			1: issue.EffortThirtyMinutes,
			2: issue.EffortHalfDay,
			3: issue.EffortThreeDays,
		},
	}
	cache, err := memory.NewIssueSearch(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	contract, err := NewSearchIssues(
		searcher,
		cache,
		50,
		WithIssueRecommendationEnrichment(recommender, 3, 1),
	)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}
	maximumEffort := string(issue.EffortHalfDay)
	limitedCriteria := searchCriteria(t, issue.SearchCriteriaOptions{
		Username:      "octocat",
		MaximumEffort: &maximumEffort,
	})

	limited, err := contract.Execute(context.Background(), SearchIssuesInput{
		Criteria:   limitedCriteria,
		Pagination: searchPagination(t, 2, 1),
	})
	if err != nil {
		t.Fatalf("Execute(limited) error = %v", err)
	}
	if len(limited.Items) != 1 ||
		limited.Items[0].Candidate.Issue.Number != 2 ||
		limited.Pagination.Total != 2 ||
		limited.Pagination.TotalPages != 2 ||
		limited.Pagination.HasNext {
		t.Fatalf("limited output = %+v", limited)
	}

	unlimited, err := contract.Execute(context.Background(), SearchIssuesInput{
		Criteria: searchCriteria(
			t,
			issue.SearchCriteriaOptions{Username: "octocat"},
		),
		Pagination: searchPagination(t, 1, 20),
	})
	if err != nil {
		t.Fatalf("Execute(unlimited) error = %v", err)
	}
	if unlimited.Pagination.Total != 3 ||
		!unlimited.CacheHit ||
		searcher.callCount() != 1 {
		t.Fatalf(
			"unlimited output = %+v, search calls = %d",
			unlimited,
			searcher.callCount(),
		)
	}
}

func TestSearchIssuesDeduplicatesConcurrentMisses(t *testing.T) {
	now := time.Now().UTC()
	started := make(chan struct{})
	release := make(chan struct{})
	searcher := &issueSearcherStub{
		started: started,
		release: release,
		result: port.GitHubIssueSearchResult{
			Candidates: []issue.Candidate{searchCandidate(now, 1, 20)},
			TotalCount: 1,
		},
	}
	usecase := newIssueSearchUsecase(t, searcher, now)
	input := SearchIssuesInput{
		Criteria: searchCriteria(
			t,
			issue.SearchCriteriaOptions{Username: "octocat"},
		),
		Pagination: searchPagination(t, 1, 20),
	}

	const callers = 12
	var waitGroup sync.WaitGroup
	results := make(chan error, callers)
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := usecase.Execute(context.Background(), input)
			results <- err
		}()
	}
	<-started
	close(release)
	waitGroup.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}
	if searcher.callCount() != 1 {
		t.Fatalf("search calls = %d, want 1", searcher.callCount())
	}
}

func TestSearchIssuesReturnsEmptyOutOfRangePage(t *testing.T) {
	now := time.Now().UTC()
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{searchCandidate(now, 1, 20)},
		TotalCount: 1,
	}}
	usecase := newIssueSearchUsecase(t, searcher, now)
	output, err := usecase.Execute(context.Background(), SearchIssuesInput{
		Criteria: searchCriteria(
			t,
			issue.SearchCriteriaOptions{Username: "octocat"},
		),
		Pagination: searchPagination(t, int(^uint(0)>>1), 20),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(output.Items) != 0 ||
		output.Pagination.Total != 1 ||
		output.Pagination.TotalPages != 1 ||
		output.Pagination.HasNext {
		t.Fatalf("output = %+v", output)
	}
}

func TestSearchIssuesEnrichesBoundedCandidatesAndRanksDeterministically(
	t *testing.T,
) {
	t.Parallel()
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{
			searchCandidate(now, 1, 10),
			searchCandidate(now, 2, 20),
			searchCandidate(now, 3, 30),
			searchCandidate(now, 4, 40),
		},
		TotalCount: 4,
	}}
	for index := range searcher.result.Candidates {
		suffix := strconv.Itoa(index + 1)
		searcher.result.Candidates[index].Repository.Name = "repo-" + suffix
		searcher.result.Candidates[index].Repository.FullName =
			"example/repo-" + suffix
	}
	recommender := &searchRecommenderStub{
		now:    now,
		scores: map[int]int{1: 40, 2: 90},
		delay:  5 * time.Millisecond,
	}
	cache, err := memory.NewIssueSearch(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	contract, err := NewSearchIssues(
		searcher,
		cache,
		50,
		WithIssueRecommendationEnrichment(recommender, 2, 2),
	)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}
	output, err := contract.Execute(context.Background(), SearchIssuesInput{
		Criteria: searchCriteria(t, issue.SearchCriteriaOptions{
			Username:  "octocat",
			Languages: []string{"Go"},
		}),
		Pagination: searchPagination(t, 1, 4),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	numbers := make([]int, 0, len(output.Items))
	for _, item := range output.Items {
		numbers = append(numbers, item.Candidate.Issue.Number)
	}
	want := []int{2, 1, 4, 3}
	if !slices.Equal(numbers, want) ||
		output.EnrichmentAttempted != 2 ||
		output.EnrichmentFailed != 0 ||
		recommender.Calls() != 2 ||
		recommender.MaxActive() > 2 {
		t.Fatalf(
			"numbers = %v, output = %+v, calls = %d, max active = %d",
			numbers,
			output,
			recommender.Calls(),
			recommender.MaxActive(),
		)
	}
}

func TestSearchIssuesProductionLoadBounds(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	candidates := make([]issue.Candidate, issue.MaximumCandidateResults)
	for index := range candidates {
		number := index + 1
		candidates[index] = searchCandidate(now, number, 100+number)
		suffix := strconv.Itoa(number)
		candidates[index].Repository.Name = "repo-" + suffix
		candidates[index].Repository.FullName = "example/repo-" + suffix
	}
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: candidates,
		TotalCount: 100_000,
	}}
	recommender := &searchRecommenderStub{
		now:   now,
		delay: 20 * time.Millisecond,
	}
	cache, err := memory.NewIssueSearch(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	const (
		analysisLimit = 20
		concurrency   = 5
	)
	contract, err := NewSearchIssues(
		searcher,
		cache,
		issue.MaximumCandidateResults,
		WithIssueRecommendationEnrichment(
			recommender,
			analysisLimit,
			concurrency,
		),
	)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}

	startedAt := time.Now()
	output, err := contract.Execute(context.Background(), SearchIssuesInput{
		Criteria: searchCriteria(
			t,
			issue.SearchCriteriaOptions{Username: "octocat"},
		),
		Pagination: searchPagination(t, 1, 20),
	})
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if searcher.requestedLimit() != issue.MaximumCandidateResults ||
		output.CandidatesChecked != issue.MaximumCandidateResults ||
		output.EnrichmentAttempted != analysisLimit ||
		recommender.Calls() != analysisLimit ||
		recommender.MaxActive() > concurrency {
		t.Fatalf(
			"limit = %d, output = %+v, calls = %d, max active = %d",
			searcher.requestedLimit(),
			output,
			recommender.Calls(),
			recommender.MaxActive(),
		)
	}
	// The deterministic dependency fixture represents 20 detail requests that
	// each take 20 ms. Five-way fan-out should finish well below the product's
	// three-second normal-request target even on constrained CI workers.
	if elapsed >= time.Second {
		t.Fatalf("bounded search took %s, want less than 1s", elapsed)
	}
}

func TestSearchIssuesCancelsProductionFanout(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	candidates := make([]issue.Candidate, 20)
	for index := range candidates {
		number := index + 1
		candidates[index] = searchCandidate(now, number, 100)
		suffix := strconv.Itoa(number)
		candidates[index].Repository.Name = "repo-" + suffix
		candidates[index].Repository.FullName = "example/repo-" + suffix
	}
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: candidates,
		TotalCount: len(candidates),
	}}
	recommender := &searchRecommenderStub{
		now:   now,
		delay: time.Second,
	}
	cache, err := memory.NewIssueSearch(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	contract, err := NewSearchIssues(
		searcher,
		cache,
		50,
		WithIssueRecommendationEnrichment(recommender, 20, 5),
	)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	_, err = contract.Execute(ctx, SearchIssuesInput{
		Criteria: searchCriteria(
			t,
			issue.SearchCriteriaOptions{Username: "octocat"},
		),
		Pagination: searchPagination(t, 1, 20),
	})
	elapsed := time.Since(startedAt)
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) ||
		applicationError.Code != apperror.CodeRequestTimeout {
		t.Fatalf("Execute() error = %v", err)
	}
	if recommender.MaxActive() > 5 {
		t.Fatalf("max active = %d, want at most 5", recommender.MaxActive())
	}
	if elapsed >= time.Second {
		t.Fatalf("cancellation took %s, want less than 1s", elapsed)
	}
}

func TestSearchIssuesReusesRepositoryInspectionWithinWindow(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{
			searchCandidate(now, 1, 20),
			searchCandidate(now, 2, 20),
			searchCandidate(now, 3, 20),
		},
		TotalCount: 3,
	}}
	recommender := &searchRecommenderStub{
		now:          now,
		scores:       map[int]int{1: 80},
		dependencies: []string{"react"},
	}
	cache, err := memory.NewIssueSearch(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	contract, err := NewSearchIssues(
		searcher,
		cache,
		50,
		WithIssueRecommendationEnrichment(recommender, 3, 2),
	)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}
	output, err := contract.Execute(context.Background(), SearchIssuesInput{
		Criteria: searchCriteria(t, issue.SearchCriteriaOptions{
			Username:  "octocat",
			Languages: []string{"Go"},
		}),
		Pagination: searchPagination(t, 1, 20),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if recommender.Calls() != 1 ||
		output.EnrichmentAttempted != 1 ||
		output.EnrichmentFailed != 0 ||
		len(output.Items) != 3 {
		t.Fatalf(
			"output = %+v, calls = %d",
			output,
			recommender.Calls(),
		)
	}
	sharedWarnings := 0
	manifestEvidence := 0
	for _, item := range output.Items {
		for _, warning := range item.Recommendation.Warnings {
			if warning.Code == "claim_evidence_unavailable" {
				sharedWarnings++
			}
		}
		for _, technology := range item.Analysis.RequiredTechnologies {
			if technology.Name == "React" &&
				technology.Confidence == issue.ConfidenceHigh {
				manifestEvidence++
			}
		}
	}
	if sharedWarnings != 2 || manifestEvidence != 2 {
		t.Fatalf(
			"shared claim warnings = %d, manifest evidence = %d",
			sharedWarnings,
			manifestEvidence,
		)
	}
}

func TestSearchIssuesFallsBackWhenOptionalEnrichmentFails(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	searcher := &issueSearcherStub{result: port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{searchCandidate(now, 1, 20)},
		TotalCount: 1,
	}}
	recommender := &searchRecommenderStub{
		now:        now,
		failNumber: 1,
	}
	cache, err := memory.NewIssueSearch(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	contract, err := NewSearchIssues(
		searcher,
		cache,
		50,
		WithIssueRecommendationEnrichment(recommender, 1, 1),
	)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}
	output, err := contract.Execute(context.Background(), SearchIssuesInput{
		Criteria: searchCriteria(
			t,
			issue.SearchCriteriaOptions{Username: "octocat"},
		),
		Pagination: searchPagination(t, 1, 20),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.EnrichmentFailed != 1 ||
		!output.EnrichmentIncomplete ||
		output.GitHubIncomplete ||
		len(output.Items) != 1 {
		t.Fatalf("output = %+v", output)
	}
	foundWarning := false
	for _, warning := range output.Items[0].Recommendation.Warnings {
		if warning.Code == "detail_enrichment_unavailable" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("warnings = %+v", output.Items[0].Recommendation.Warnings)
	}
}

func TestSearchIssuesMapsErrors(t *testing.T) {
	tests := []struct {
		name       string
		searchErr  error
		wantCode   apperror.Code
		wantStatus int
	}{
		{
			name:       "invalid criteria",
			searchErr:  issue.ErrInvalidSearchCriteria,
			wantCode:   apperror.CodeInvalidRequest,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "rate limited",
			searchErr: &port.GitHubError{
				Kind: port.GitHubErrorRateLimited,
			},
			wantCode:   apperror.CodeRateLimit,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "cancelled",
			searchErr:  context.Canceled,
			wantCode:   apperror.CodeRequestTimeout,
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name: "upstream",
			searchErr: &port.GitHubError{
				Kind: port.GitHubErrorUpstream,
			},
			wantCode:   apperror.CodeGitHubAPI,
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now().UTC()
			usecase := newIssueSearchUsecase(
				t,
				&issueSearcherStub{err: test.searchErr},
				now,
			)
			_, err := usecase.Execute(context.Background(), SearchIssuesInput{
				Criteria: searchCriteria(
					t,
					issue.SearchCriteriaOptions{Username: "octocat"},
				),
				Pagination: searchPagination(t, 1, 20),
			})

			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) {
				t.Fatalf("Execute() error = %v", err)
			}
			if applicationError.Code != test.wantCode ||
				applicationError.HTTPStatus != test.wantStatus ||
				!errors.Is(applicationError, test.searchErr) {
				t.Fatalf("application error = %+v", applicationError)
			}
		})
	}
}

func TestNewSearchIssuesRejectsInvalidDependencies(t *testing.T) {
	cache, err := memory.NewIssueSearch(1, time.Minute)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	searcher := &issueSearcherStub{}

	tests := []struct {
		name     string
		searcher port.GitHubIssueSearcher
		cache    port.IssueSearchCache
		limit    int
	}{
		{name: "missing searcher", cache: cache, limit: 50},
		{name: "missing cache", searcher: searcher, limit: 50},
		{name: "invalid limit", searcher: searcher, cache: cache, limit: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewSearchIssues(
				test.searcher,
				test.cache,
				test.limit,
			); err == nil {
				t.Fatal("NewSearchIssues() error = nil")
			}
		})
	}

	recommender := &searchRecommenderStub{}
	for name, option := range map[string]SearchIssuesOption{
		"missing recommender": WithIssueRecommendationEnrichment(nil, 1, 1),
		"invalid analysis limit": WithIssueRecommendationEnrichment(
			recommender,
			51,
			1,
		),
		"invalid concurrency": WithIssueRecommendationEnrichment(
			recommender,
			2,
			3,
		),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewSearchIssues(
				searcher,
				cache,
				50,
				option,
			); err == nil {
				t.Fatal("NewSearchIssues() option error = nil")
			}
		})
	}
}

type issueSearcherStub struct {
	mu      sync.Mutex
	result  port.GitHubIssueSearchResult
	err     error
	started chan struct{}
	once    sync.Once
	release chan struct{}
	calls   int
	limit   int
}

type searchRecommenderStub struct {
	mu           sync.Mutex
	now          time.Time
	scores       map[int]int
	efforts      map[int]issue.EffortBand
	failNumber   int
	dependencies []string
	delay        time.Duration
	calls        int
	active       int
	maxActive    int
}

func (stub *searchRecommenderStub) Execute(
	ctx context.Context,
	input RecommendIssueInput,
) (RecommendIssueOutput, error) {
	stub.mu.Lock()
	stub.calls++
	stub.active++
	if stub.active > stub.maxActive {
		stub.maxActive = stub.active
	}
	stub.mu.Unlock()
	defer func() {
		stub.mu.Lock()
		stub.active--
		stub.mu.Unlock()
	}()

	if stub.delay > 0 {
		timer := time.NewTimer(stub.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return RecommendIssueOutput{}, ctx.Err()
		}
	}
	if input.Reference.Number() == stub.failNumber {
		return RecommendIssueOutput{}, &port.GitHubError{
			Kind: port.GitHubErrorUpstream,
		}
	}
	score := stub.scores[input.Reference.Number()]
	effort := stub.efforts[input.Reference.Number()]
	candidate := searchCandidate(
		stub.now,
		input.Reference.Number(),
		input.Reference.Number()*10,
	)
	candidate.Repository.Owner = input.Reference.Owner()
	candidate.Repository.Name = input.Reference.RepositoryName()
	candidate.Repository.FullName = input.Reference.Owner() + "/" +
		input.Reference.RepositoryName()
	return RecommendIssueOutput{
		Item: issue.RankedIssue{
			Candidate: candidate,
			Analysis: issue.Analysis{
				Effort: issue.EffortEstimate{Band: effort},
			},
			Recommendation: issue.Recommendation{
				Score: score,
				RepositorySignals: []issue.RepositorySignal{{
					Key:   issue.RepositoryREADME,
					State: issue.SignalPresent,
				}},
				Activity: issue.ActivityMetrics{
					LastMeaningfulUpdate: stub.now,
					CI:                   issue.CIStateSuccess,
				},
			},
		},
		Dependencies: append(
			make([]string, 0, len(stub.dependencies)),
			stub.dependencies...,
		),
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 40 - input.Reference.Number(),
		},
	}, nil
}

func (stub *searchRecommenderStub) EvaluateCandidate(
	candidate issue.Candidate,
	_ []string,
) issue.RankedIssue {
	return issue.RankedIssue{
		Candidate: candidate,
		Recommendation: issue.Recommendation{
			Score: candidate.Issue.Number,
		},
	}
}

func (stub *searchRecommenderStub) Calls() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls
}

func (stub *searchRecommenderStub) MaxActive() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.maxActive
}

func (stub *issueSearcherStub) SearchIssues(
	ctx context.Context,
	_ issue.SearchCriteria,
	limit int,
) (port.GitHubIssueSearchResult, error) {
	stub.mu.Lock()
	stub.calls++
	stub.limit = limit
	stub.mu.Unlock()
	if stub.started != nil {
		stub.once.Do(func() { close(stub.started) })
	}
	if stub.release != nil {
		select {
		case <-stub.release:
		case <-ctx.Done():
			return port.GitHubIssueSearchResult{}, ctx.Err()
		}
	}
	return stub.result, stub.err
}

func (stub *issueSearcherStub) callCount() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls
}

func (stub *issueSearcherStub) requestedLimit() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.limit
}

func newIssueSearchUsecase(
	t *testing.T,
	searcher port.GitHubIssueSearcher,
	now time.Time,
) *searchIssues {
	t.Helper()
	cache, err := memory.NewIssueSearch(100, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewIssueSearch() error = %v", err)
	}
	contract, err := NewSearchIssues(searcher, cache, 50)
	if err != nil {
		t.Fatalf("NewSearchIssues() error = %v", err)
	}
	concrete, valid := contract.(*searchIssues)
	if !valid {
		t.Fatal("NewSearchIssues() returned an unexpected implementation")
	}
	concrete.now = func() time.Time { return now }
	return concrete
}

func searchCriteria(
	t *testing.T,
	options issue.SearchCriteriaOptions,
) issue.SearchCriteria {
	t.Helper()
	criteria, err := issue.NewSearchCriteria(options)
	if err != nil {
		t.Fatalf("NewSearchCriteria() error = %v", err)
	}
	return criteria
}

func searchPagination(t *testing.T, page, perPage int) issue.Pagination {
	t.Helper()
	pagination, err := issue.NewPagination(page, perPage)
	if err != nil {
		t.Fatalf("NewPagination() error = %v", err)
	}
	return pagination
}

func searchCandidate(
	now time.Time,
	number int,
	stars int,
) issue.Candidate {
	return issue.Candidate{
		Repository: repository.Summary{
			ID:           int64(number),
			Owner:        "example",
			Name:         "repo",
			FullName:     "example/repo",
			Description:  "A maintained repository",
			URL:          "https://github.com/example/repo",
			MainLanguage: "Go",
			Stars:        stars,
			UpdatedAt:    now.Add(-time.Hour),
		},
		Issue: issue.Summary{
			Number:      number,
			Title:       "Improve request validation",
			Body:        "Add request validation, precise errors, regression tests, and documented acceptance criteria.",
			URL:         "https://github.com/example/repo/issues/1",
			State:       issue.StateOpen,
			Labels:      []string{"good first issue"},
			AuthorLogin: "contributor",
			AuthorType:  issue.AuthorHuman,
			CreatedAt:   now.Add(-48 * time.Hour),
			UpdatedAt:   now.Add(-time.Hour),
		},
	}
}
