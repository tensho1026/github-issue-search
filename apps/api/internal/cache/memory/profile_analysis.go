package memory

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

type profileAnalysisItem struct {
	key       string
	entry     port.ProfileAnalysisCacheEntry
	expiresAt time.Time
}

// ProfileAnalysis is a bounded, concurrency-safe LRU cache. Its port can be
// replaced by a distributed cache without changing the analysis usecase.
type ProfileAnalysis struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	now      func() time.Time
	items    map[string]*list.Element
	recency  *list.List
}

func NewProfileAnalysis(
	capacity int,
	ttl time.Duration,
) (*ProfileAnalysis, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("profile analysis cache capacity must be positive")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("profile analysis cache TTL must be positive")
	}

	return &ProfileAnalysis{
		capacity: capacity,
		ttl:      ttl,
		now:      time.Now,
		items:    make(map[string]*list.Element, capacity),
		recency:  list.New(),
	}, nil
}

func (c *ProfileAnalysis) Get(
	ctx context.Context,
	username user.Username,
) (port.ProfileAnalysisCacheEntry, bool, error) {
	if err := ctx.Err(); err != nil {
		return port.ProfileAnalysisCacheEntry{}, false, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := profileAnalysisKey(username)
	element, exists := c.items[key]
	if !exists {
		return port.ProfileAnalysisCacheEntry{}, false, nil
	}
	item, valid := element.Value.(*profileAnalysisItem)
	if !valid {
		delete(c.items, key)
		c.recency.Remove(element)
		return port.ProfileAnalysisCacheEntry{}, false, fmt.Errorf(
			"profile analysis cache contains an invalid item",
		)
	}
	if !c.now().Before(item.expiresAt) {
		c.remove(element)
		return port.ProfileAnalysisCacheEntry{}, false, nil
	}

	c.recency.MoveToFront(element)
	return cloneProfileAnalysisEntry(item.entry), true, nil
}

func (c *ProfileAnalysis) Set(
	ctx context.Context,
	username user.Username,
	entry port.ProfileAnalysisCacheEntry,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	key := profileAnalysisKey(username)
	if element, exists := c.items[key]; exists {
		item, valid := element.Value.(*profileAnalysisItem)
		if !valid {
			delete(c.items, key)
			c.recency.Remove(element)
			return fmt.Errorf("profile analysis cache contains an invalid item")
		}
		item.entry = cloneProfileAnalysisEntry(entry)
		item.expiresAt = c.now().Add(c.ttl)
		c.recency.MoveToFront(element)
		return nil
	}

	item := &profileAnalysisItem{
		key:       key,
		entry:     cloneProfileAnalysisEntry(entry),
		expiresAt: c.now().Add(c.ttl),
	}
	c.items[key] = c.recency.PushFront(item)
	if c.recency.Len() > c.capacity {
		c.remove(c.recency.Back())
	}
	return nil
}

func (c *ProfileAnalysis) remove(element *list.Element) {
	if element == nil {
		return
	}
	item, valid := element.Value.(*profileAnalysisItem)
	if !valid {
		c.recency.Remove(element)
		return
	}
	delete(c.items, item.key)
	c.recency.Remove(element)
}

func profileAnalysisKey(username user.Username) string {
	return "github:profile-analysis:" + strings.ToLower(username.String())
}

func cloneProfileAnalysisEntry(
	entry port.ProfileAnalysisCacheEntry,
) port.ProfileAnalysisCacheEntry {
	cloned := entry
	cloned.Analysis = profile.Analysis{
		Username:             entry.Analysis.Username,
		Languages:            append([]profile.LanguageShare(nil), entry.Analysis.Languages...),
		Frameworks:           append([]string(nil), entry.Analysis.Frameworks...),
		RepositoriesAnalyzed: entry.Analysis.RepositoriesAnalyzed,
		Warnings:             append([]profile.Warning(nil), entry.Analysis.Warnings...),
	}
	return cloned
}

var _ port.ProfileAnalysisCache = (*ProfileAnalysis)(nil)
