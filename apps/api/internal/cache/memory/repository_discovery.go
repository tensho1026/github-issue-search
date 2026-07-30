package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// RepositoryDiscovery stores bounded normalized public result windows. It
// never stores the anonymous request body or any authenticated data.
type RepositoryDiscovery struct {
	store *lruCache[string, port.RepositoryDiscoveryCacheEntry]
}

func NewRepositoryDiscovery(
	capacity int,
	ttl time.Duration,
) (*RepositoryDiscovery, error) {
	store, err := newLRUCache[string, port.RepositoryDiscoveryCacheEntry](
		capacity,
		ttl,
		cloneRepositoryDiscoveryEntry,
	)
	if err != nil {
		return nil, fmt.Errorf("create repository discovery cache: %w", err)
	}
	return &RepositoryDiscovery{store: store}, nil
}

func (cache *RepositoryDiscovery) Get(
	ctx context.Context,
	key string,
) (port.RepositoryDiscoveryCacheEntry, bool, error) {
	return cache.store.get(ctx, key)
}

func (cache *RepositoryDiscovery) Set(
	ctx context.Context,
	key string,
	entry port.RepositoryDiscoveryCacheEntry,
) error {
	return cache.store.set(ctx, key, entry)
}

func cloneRepositoryDiscoveryEntry(
	entry port.RepositoryDiscoveryCacheEntry,
) port.RepositoryDiscoveryCacheEntry {
	cloned := entry
	cloned.Items = make([]repository.DiscoveryResult, len(entry.Items))
	for index, item := range entry.Items {
		cloned.Items[index] = item
		cloned.Items[index].Topics = append([]string(nil), item.Topics...)
		cloned.Items[index].Technologies = append(
			[]string(nil),
			item.Technologies...,
		)
		cloned.Items[index].Difficulty.Reasons = append(
			[]string(nil),
			item.Difficulty.Reasons...,
		)
		cloned.Items[index].Readiness.Reasons = append(
			[]string(nil),
			item.Readiness.Reasons...,
		)
		cloned.Items[index].Warnings = append(
			[]repository.DiscoveryWarning(nil),
			item.Warnings...,
		)
	}
	return cloned
}

var _ port.RepositoryDiscoveryCache = (*RepositoryDiscovery)(nil)
