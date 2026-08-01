package memory

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestLRUCacheRemainsBoundedUnderConcurrentChurn(t *testing.T) {
	t.Parallel()
	const (
		capacity = 32
		workers  = 24
		writes   = 200
	)
	cache, err := newLRUCache[string, []string](
		capacity,
		time.Hour,
		func(value []string) []string {
			return append([]string(nil), value...)
		},
	)
	if err != nil {
		t.Fatalf("newLRUCache() error = %v", err)
	}

	var waitGroup sync.WaitGroup
	for worker := range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for write := range writes {
				key := strconv.Itoa(worker) + "/" + strconv.Itoa(write)
				if err := cache.set(
					context.Background(),
					key,
					[]string{key},
				); err != nil {
					t.Errorf("set(%q) error = %v", key, err)
					return
				}
				if _, _, err := cache.get(context.Background(), key); err != nil {
					t.Errorf("get(%q) error = %v", key, err)
					return
				}
			}
		}()
	}
	waitGroup.Wait()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.items) > capacity || cache.recency.Len() > capacity {
		t.Fatalf(
			"cache map length = %d, recency length = %d, capacity = %d",
			len(cache.items),
			cache.recency.Len(),
			capacity,
		)
	}
	if len(cache.items) != cache.recency.Len() {
		t.Fatalf(
			"cache map length = %d, recency length = %d",
			len(cache.items),
			cache.recency.Len(),
		)
	}
}
