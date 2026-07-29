package usecase

import (
	"context"
	"errors"
	"net/http"
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
		first.Items[0].Issue.Number != 1 ||
		first.Pagination != (SearchIssuesPagination{
			Page:       1,
			PerPage:    1,
			Total:      2,
			TotalPages: 2,
			HasNext:    true,
		}) ||
		first.CandidatesChecked != 4 ||
		first.UpstreamTotal != 900 ||
		!first.IncompleteResults ||
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
		second.Items[0].Issue.Number != 2 ||
		second.Pagination.HasNext ||
		!second.CacheHit ||
		searcher.callCount() != 1 {
		t.Fatalf("second output = %+v, calls = %d", second, searcher.callCount())
	}

	second.Items[0].Issue.Labels[0] = "mutated"
	third, err := usecase.Execute(context.Background(), SearchIssuesInput{
		Criteria:   criteria,
		Pagination: searchPagination(t, 2, 1),
	})
	if err != nil {
		t.Fatalf("Execute(third) error = %v", err)
	}
	if third.Items[0].Issue.Labels[0] != "good first issue" {
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
