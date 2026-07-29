package memory

import (
	"context"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestIssueDetailCacheReturnsIndependentCopies(t *testing.T) {
	t.Parallel()
	cache := newTestIssueDetailCache(t, 2, time.Hour)
	entry := port.GitHubIssueDetailResult{
		Candidate: issue.Candidate{
			Issue: issue.Summary{
				Labels:    []string{"help wanted"},
				Assignees: []string{"contributor"},
			},
		},
		RepositorySignals: []issue.RepositorySignal{{
			Key:   issue.RepositoryREADME,
			State: issue.SignalPresent,
			Evidence: []issue.Evidence{{
				RuleID: "repository.signal.readme",
			}},
		}},
		Comments: []issue.CommentObservation{{
			AuthorLogin: "reader",
		}},
	}
	if err := cache.Set(context.Background(), "owner/repo#1", entry); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	first, found, err := cache.Get(context.Background(), "owner/repo#1")
	if err != nil || !found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
	first.Candidate.Issue.Labels[0] = "mutated"
	first.Candidate.Issue.Assignees[0] = "mutated"
	first.RepositorySignals[0].Evidence[0].RuleID = "mutated"
	first.Comments[0].AuthorLogin = "mutated"

	second, found, err := cache.Get(context.Background(), "owner/repo#1")
	if err != nil || !found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
	if second.Candidate.Issue.Labels[0] != "help wanted" ||
		second.Candidate.Issue.Assignees[0] != "contributor" ||
		second.RepositorySignals[0].Evidence[0].RuleID !=
			"repository.signal.readme" ||
		second.Comments[0].AuthorLogin != "reader" {
		t.Fatalf("cached detail was mutated = %+v", second)
	}
}

func TestIssueDetailCacheExpiresEvictsAndHonorsCancellation(t *testing.T) {
	t.Parallel()
	cache := newTestIssueDetailCache(t, 1, time.Minute)
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	cache.store.now = func() time.Time { return now }
	ctx := context.Background()
	entry := port.GitHubIssueDetailResult{}

	if err := cache.Set(ctx, "first", entry); err != nil {
		t.Fatalf("Set(first) error = %v", err)
	}
	if err := cache.Set(ctx, "second", entry); err != nil {
		t.Fatalf("Set(second) error = %v", err)
	}
	if _, found, _ := cache.Get(ctx, "first"); found {
		t.Fatal("least recently used entry was not evicted")
	}

	now = now.Add(time.Minute)
	if _, found, err := cache.Get(ctx, "second"); err != nil || found {
		t.Fatalf("expired Get() found = %t, error = %v", found, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := cache.Set(cancelled, "third", entry); err == nil {
		t.Fatal("cancelled Set() error = nil")
	}
	if _, found, err := cache.Get(cancelled, "third"); err == nil || found {
		t.Fatalf("cancelled Get() found = %t, error = %v", found, err)
	}
}

func TestIssueDetailCacheRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	for _, input := range []struct {
		capacity int
		ttl      time.Duration
	}{
		{capacity: 0, ttl: time.Minute},
		{capacity: 1, ttl: 0},
	} {
		if _, err := NewIssueDetail(input.capacity, input.ttl); err == nil {
			t.Fatalf("NewIssueDetail(%d, %s) error = nil", input.capacity, input.ttl)
		}
	}
}

func newTestIssueDetailCache(
	t *testing.T,
	capacity int,
	ttl time.Duration,
) *IssueDetail {
	t.Helper()
	cache, err := NewIssueDetail(capacity, ttl)
	if err != nil {
		t.Fatalf("NewIssueDetail() error = %v", err)
	}
	return cache
}
