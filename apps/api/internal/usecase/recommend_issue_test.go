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

func TestRecommendIssueCachesAndReusesCompleteAnalysis(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	reader := &issueDetailReaderStub{result: issueDetailUsecaseFixture(now)}
	cache, err := memory.NewIssueDetail(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueDetail() error = %v", err)
	}
	contract, err := NewRecommendIssue(reader, cache)
	if err != nil {
		t.Fatalf("NewRecommendIssue() error = %v", err)
	}
	implementation, valid := contract.(*recommendIssue)
	if !valid {
		t.Fatal("NewRecommendIssue() returned an unexpected implementation")
	}
	implementation.now = func() time.Time { return now }
	reference, err := issue.NewReference("acme", "rocket", 42)
	if err != nil {
		t.Fatalf("NewReference() error = %v", err)
	}
	input := RecommendIssueInput{
		Reference:     reference,
		DesiredSkills: []string{"Go"},
	}

	first, err := contract.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	second, err := contract.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}

	if first.CacheHit ||
		!second.CacheHit ||
		reader.Calls() != 1 ||
		first.Item.Recommendation.Score != second.Item.Recommendation.Score ||
		first.Item.Recommendation.SkillMatch.Percentage != 50 ||
		first.Item.Analysis.Difficulty.Level < 1 ||
		first.RateLimit.Remaining != 49 {
		t.Fatalf(
			"first = %+v, second = %+v, calls = %d",
			first,
			second,
			reader.Calls(),
		)
	}
}

func TestRecommendIssueDeduplicatesConcurrentMisses(t *testing.T) {
	t.Parallel()
	start := make(chan struct{})
	release := make(chan struct{})
	reader := &issueDetailReaderStub{
		result: issueDetailUsecaseFixture(time.Now()),
		start:  start,
		block:  release,
	}
	cache, err := memory.NewIssueDetail(10, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueDetail() error = %v", err)
	}
	contract, err := NewRecommendIssue(reader, cache)
	if err != nil {
		t.Fatalf("NewRecommendIssue() error = %v", err)
	}
	reference, _ := issue.NewReference("acme", "rocket", 42)

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, executeErr := contract.Execute(
				context.Background(),
				RecommendIssueInput{Reference: reference},
			)
			errs <- executeErr
		}()
	}
	<-start
	close(release)
	for range 2 {
		if executeErr := <-errs; executeErr != nil {
			t.Fatalf("Execute() error = %v", executeErr)
		}
	}
	if reader.Calls() != 1 {
		t.Fatalf("reader calls = %d, want 1", reader.Calls())
	}
}

func TestRecommendIssueFallbackPerformsNoIOAndMarksEvidenceUnknown(t *testing.T) {
	t.Parallel()
	reader := &issueDetailReaderStub{}
	cache, err := memory.NewIssueDetail(1, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueDetail() error = %v", err)
	}
	contract, err := NewRecommendIssue(reader, cache)
	if err != nil {
		t.Fatalf("NewRecommendIssue() error = %v", err)
	}
	candidate := issue.Candidate{
		Repository: repository.Summary{
			FullName:     "acme/rocket",
			MainLanguage: "Go",
			UpdatedAt:    time.Now(),
		},
		Issue: issue.Summary{
			Number:    42,
			Title:     "Add tests",
			Body:      "Add tests for the launch path with acceptance criteria and expected behavior.",
			State:     issue.StateOpen,
			UpdatedAt: time.Now(),
		},
	}

	got := contract.EvaluateCandidate(candidate, []string{"Go"})

	if reader.Calls() != 0 || got.Recommendation.Score < 1 {
		t.Fatalf("fallback = %+v, calls = %d", got, reader.Calls())
	}
	for _, signal := range got.Recommendation.RepositorySignals {
		if signal.State != issue.SignalUnknown {
			t.Fatalf("repository signal = %+v", signal)
		}
	}
	if got.Recommendation.Claim.Confidence != issue.ConfidenceLow {
		t.Fatalf("claim = %+v", got.Recommendation.Claim)
	}
}

func TestRecommendIssueMapsErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		readerErr  error
		wantCode   apperror.Code
		wantStatus int
	}{
		{
			name:       "not found",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorNotFound},
			wantCode:   apperror.CodeNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rate limited",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorRateLimited},
			wantCode:   apperror.CodeRateLimit,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "upstream",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorUpstream},
			wantCode:   apperror.CodeGitHubAPI,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "cancelled",
			readerErr:  context.Canceled,
			wantCode:   apperror.CodeRequestTimeout,
			wantStatus: http.StatusGatewayTimeout,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cache, cacheErr := memory.NewIssueDetail(1, time.Hour)
			if cacheErr != nil {
				t.Fatalf("NewIssueDetail() error = %v", cacheErr)
			}
			contract, composeErr := NewRecommendIssue(
				&issueDetailReaderStub{err: test.readerErr},
				cache,
			)
			if composeErr != nil {
				t.Fatalf("NewRecommendIssue() error = %v", composeErr)
			}
			reference, _ := issue.NewReference("acme", "rocket", 42)
			_, err := contract.Execute(
				context.Background(),
				RecommendIssueInput{Reference: reference},
			)
			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) ||
				applicationError.Code != test.wantCode ||
				applicationError.HTTPStatus != test.wantStatus {
				t.Fatalf("Execute() error = %+v", err)
			}
			if !errors.Is(applicationError, test.readerErr) {
				t.Fatal("Execute() did not preserve cause")
			}
		})
	}
}

func TestNewRecommendIssueRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	cache, err := memory.NewIssueDetail(1, time.Hour)
	if err != nil {
		t.Fatalf("NewIssueDetail() error = %v", err)
	}
	if _, err := NewRecommendIssue(nil, cache); err == nil {
		t.Fatal("nil reader error = nil")
	}
	if _, err := NewRecommendIssue(&issueDetailReaderStub{}, nil); err == nil {
		t.Fatal("nil cache error = nil")
	}
}

type issueDetailReaderStub struct {
	mu     sync.Mutex
	result port.GitHubIssueDetailResult
	err    error
	calls  int
	start  chan struct{}
	block  chan struct{}
}

func (stub *issueDetailReaderStub) GetIssueDetail(
	context.Context,
	string,
	string,
	int,
) (port.GitHubIssueDetailResult, error) {
	stub.mu.Lock()
	stub.calls++
	calls := stub.calls
	stub.mu.Unlock()
	if calls == 1 && stub.start != nil {
		close(stub.start)
	}
	if stub.block != nil {
		<-stub.block
	}
	return stub.result, stub.err
}

func (stub *issueDetailReaderStub) Calls() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls
}

func issueDetailUsecaseFixture(
	now time.Time,
) port.GitHubIssueDetailResult {
	return port.GitHubIssueDetailResult{
		Candidate: issue.Candidate{
			Repository: repository.Summary{
				Owner:        "acme",
				Name:         "rocket",
				FullName:     "acme/rocket",
				MainLanguage: "Go",
				UpdatedAt:    now.Add(-time.Hour),
				PushedAt:     now.Add(-time.Hour),
			},
			Issue: issue.Summary{
				Number: 42,
				Title:  "Improve launch validation",
				Body: "The launch path needs tests, expected behavior, " +
					"implementation guidance, and acceptance criteria.",
				State:     issue.StateOpen,
				UpdatedAt: now.Add(-time.Hour),
				CreatedAt: now.Add(-24 * time.Hour),
			},
		},
		RepositorySignals: []issue.RepositorySignal{
			{Key: issue.RepositoryREADME, State: issue.SignalPresent},
			{
				Key:   issue.RepositoryContributing,
				State: issue.SignalPresent,
			},
			{Key: issue.RepositoryCI, State: issue.SignalPresent},
			{Key: issue.RepositoryTests, State: issue.SignalPresent},
			{
				Key:   issue.RepositoryCodeOfConduct,
				State: issue.SignalPresent,
			},
		},
		Activity: issue.ActivityMetrics{
			LastMeaningfulUpdate: now.Add(-time.Hour),
			CI:                   issue.CIStateSuccess,
			Contributors:         issue.SummarizeCount(3, 5, 180, false),
			PullRequestsOpened:   issue.SummarizeCount(5, 5, 180, false),
			PullRequestMerge:     issue.SummarizeRatio(4, 5, 180, false),
			IssueResponse: issue.SummarizeDurations(
				[]time.Duration{time.Hour},
				180,
				false,
			),
			PullRequestReview: issue.SummarizeDurations(
				[]time.Duration{time.Hour},
				180,
				false,
			),
			PullRequestMergeTime: issue.SummarizeDurations(
				[]time.Duration{time.Hour},
				180,
				false,
			),
		},
		Comments: []issue.CommentObservation{{
			AuthorLogin: "reader",
			AuthorType:  "User",
			Body:        "Can I work on this?",
		}},
		RateLimit: port.RateLimit{Known: true, Remaining: 49},
	}
}
