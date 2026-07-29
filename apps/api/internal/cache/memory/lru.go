package memory

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"
)

type lruItem[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// lruCache is the shared bounded TTL/LRU storage primitive for in-memory
// adapters. cloneValue prevents callers from mutating cached slice or map
// ownership across concurrent requests.
type lruCache[K comparable, V any] struct {
	mu         sync.Mutex
	capacity   int
	ttl        time.Duration
	now        func() time.Time
	cloneValue func(V) V
	items      map[K]*list.Element
	recency    *list.List
}

func newLRUCache[K comparable, V any](
	capacity int,
	ttl time.Duration,
	cloneValue func(V) V,
) (*lruCache[K, V], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("cache capacity must be positive")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("cache TTL must be positive")
	}
	if cloneValue == nil {
		return nil, fmt.Errorf("cache clone function is required")
	}
	return &lruCache[K, V]{
		capacity:   capacity,
		ttl:        ttl,
		now:        time.Now,
		cloneValue: cloneValue,
		items:      make(map[K]*list.Element, capacity),
		recency:    list.New(),
	}, nil
}

func (cache *lruCache[K, V]) get(
	ctx context.Context,
	key K,
) (V, bool, error) {
	var zero V
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	element, exists := cache.items[key]
	if !exists {
		return zero, false, nil
	}
	item, valid := element.Value.(*lruItem[K, V])
	if !valid {
		delete(cache.items, key)
		cache.recency.Remove(element)
		return zero, false, fmt.Errorf("cache contains an invalid item")
	}
	if !cache.now().Before(item.expiresAt) {
		cache.remove(element)
		return zero, false, nil
	}

	cache.recency.MoveToFront(element)
	return cache.cloneValue(item.value), true, nil
}

func (cache *lruCache[K, V]) set(
	ctx context.Context,
	key K,
	value V,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if element, exists := cache.items[key]; exists {
		item, valid := element.Value.(*lruItem[K, V])
		if !valid {
			delete(cache.items, key)
			cache.recency.Remove(element)
			return fmt.Errorf("cache contains an invalid item")
		}
		item.value = cache.cloneValue(value)
		item.expiresAt = cache.now().Add(cache.ttl)
		cache.recency.MoveToFront(element)
		return nil
	}

	item := &lruItem[K, V]{
		key:       key,
		value:     cache.cloneValue(value),
		expiresAt: cache.now().Add(cache.ttl),
	}
	cache.items[key] = cache.recency.PushFront(item)
	if cache.recency.Len() > cache.capacity {
		cache.remove(cache.recency.Back())
	}
	return nil
}

func (cache *lruCache[K, V]) remove(element *list.Element) {
	if element == nil {
		return
	}
	item, valid := element.Value.(*lruItem[K, V])
	if valid {
		delete(cache.items, item.key)
	}
	cache.recency.Remove(element)
}
