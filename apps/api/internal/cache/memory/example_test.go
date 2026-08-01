package memory_test

import (
	"context"
	"fmt"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func ExampleIssueSearch() {
	cache, err := memory.NewIssueSearch(2, time.Minute)
	if err != nil {
		panic(err)
	}
	entry := port.IssueSearchCacheEntry{
		CandidatesChecked: 3,
		ExclusionCounts: map[issue.ExclusionReason]int{
			issue.ExclusionStale: 1,
		},
	}
	err = cache.Set(context.Background(), "criteria-key", entry)
	if err != nil {
		panic(err)
	}

	// The cache owns a deep copy rather than the caller's mutable map.
	entry.ExclusionCounts[issue.ExclusionStale] = 99
	loaded, found, err := cache.Get(context.Background(), "criteria-key")
	fmt.Printf(
		"found=%t stale=%d error=%v\n",
		found,
		loaded.ExclusionCounts[issue.ExclusionStale],
		err,
	)

	// Output:
	// found=true stale=1 error=<nil>
}
