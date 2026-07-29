package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// IssueSearch stores bounded, eligible candidate windows by canonical
// criteria hash. It owns deep copies of every mutable collection.
type IssueSearch struct {
	store *lruCache[string, port.IssueSearchCacheEntry]
}

func NewIssueSearch(capacity int, ttl time.Duration) (*IssueSearch, error) {
	store, err := newLRUCache[string, port.IssueSearchCacheEntry](
		capacity,
		ttl,
		cloneIssueSearchEntry,
	)
	if err != nil {
		return nil, fmt.Errorf("create issue search cache: %w", err)
	}
	return &IssueSearch{store: store}, nil
}

func (cache *IssueSearch) Get(
	ctx context.Context,
	key string,
) (port.IssueSearchCacheEntry, bool, error) {
	return cache.store.get(ctx, key)
}

func (cache *IssueSearch) Set(
	ctx context.Context,
	key string,
	entry port.IssueSearchCacheEntry,
) error {
	return cache.store.set(ctx, key, entry)
}

func cloneIssueSearchEntry(
	entry port.IssueSearchCacheEntry,
) port.IssueSearchCacheEntry {
	cloned := entry
	cloned.Candidates = make([]issue.Candidate, len(entry.Candidates))
	for index, candidate := range entry.Candidates {
		cloned.Candidates[index] = candidate
		cloned.Candidates[index].Issue.Labels = append(
			[]string(nil),
			candidate.Issue.Labels...,
		)
		cloned.Candidates[index].Issue.Assignees = append(
			[]string(nil),
			candidate.Issue.Assignees...,
		)
	}
	cloned.ExclusionCounts = make(
		map[issue.ExclusionReason]int,
		len(entry.ExclusionCounts),
	)
	for reason, count := range entry.ExclusionCounts {
		cloned.ExclusionCounts[reason] = count
	}
	return cloned
}

var _ port.IssueSearchCache = (*IssueSearch)(nil)
