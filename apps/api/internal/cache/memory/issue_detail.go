package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// IssueDetail stores bounded normalized repository inspections. Deep copies
// preserve ownership across concurrent list and detail requests.
type IssueDetail struct {
	store *lruCache[string, port.GitHubIssueDetailResult]
}

// NewIssueDetail constructs an LRU cache with positive capacity and TTL.
func NewIssueDetail(
	capacity int,
	ttl time.Duration,
) (*IssueDetail, error) {
	store, err := newLRUCache[string, port.GitHubIssueDetailResult](
		capacity,
		ttl,
		cloneIssueDetailResult,
	)
	if err != nil {
		return nil, fmt.Errorf("create issue detail cache: %w", err)
	}
	return &IssueDetail{store: store}, nil
}

// Get returns a deep copy, reports misses without error, and honors a
// pre-cancelled context before acquiring the cache lock.
func (cache *IssueDetail) Get(
	ctx context.Context,
	key string,
) (port.GitHubIssueDetailResult, bool, error) {
	return cache.store.get(ctx, key)
}

// Set stores a deep copy, refreshes TTL on replacement, and may evict the
// least-recently-used entry.
func (cache *IssueDetail) Set(
	ctx context.Context,
	key string,
	entry port.GitHubIssueDetailResult,
) error {
	return cache.store.set(ctx, key, entry)
}

func cloneIssueDetailResult(
	entry port.GitHubIssueDetailResult,
) port.GitHubIssueDetailResult {
	cloned := entry
	cloned.Candidate.Issue.Labels = append(
		[]string(nil),
		entry.Candidate.Issue.Labels...,
	)
	cloned.Candidate.Issue.Assignees = append(
		[]string(nil),
		entry.Candidate.Issue.Assignees...,
	)
	cloned.Dependencies = append(
		make([]string, 0, len(entry.Dependencies)),
		entry.Dependencies...,
	)
	cloned.RepositorySignals = make(
		[]issue.RepositorySignal,
		len(entry.RepositorySignals),
	)
	for index, signal := range entry.RepositorySignals {
		cloned.RepositorySignals[index] = signal
		cloned.RepositorySignals[index].Evidence = append(
			[]issue.Evidence(nil),
			signal.Evidence...,
		)
	}
	cloned.Comments = append(
		[]issue.CommentObservation(nil),
		entry.Comments...,
	)
	return cloned
}

var _ port.IssueDetailCache = (*IssueDetail)(nil)
