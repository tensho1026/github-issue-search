package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestProfileAnalysisCacheReturnsIndependentCopies(t *testing.T) {
	cache := newTestProfileAnalysisCache(t, 2, time.Hour)
	entry := testProfileAnalysisEntry("octocat", "Go")
	if err := cache.Set(context.Background(), "OctoCat", entry); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	first, found, err := cache.Get(context.Background(), "octocat")
	if err != nil || !found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
	first.Analysis.Languages[0].Name = "mutated"
	first.Analysis.Frameworks[0] = "mutated"
	first.Analysis.Warnings[0].Code = "mutated"

	second, found, err := cache.Get(context.Background(), "OCTOCAT")
	if err != nil || !found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
	if second.Analysis.Languages[0].Name != "Go" ||
		second.Analysis.Frameworks[0] != "Gin" ||
		second.Analysis.Warnings[0].Code != "partial_data" {
		t.Fatalf("cached entry was mutated = %+v", second)
	}
}

func TestProfileAnalysisCacheExpiresEntries(t *testing.T) {
	cache := newTestProfileAnalysisCache(t, 2, time.Minute)
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	cache.store.now = func() time.Time { return now }

	if err := cache.Set(
		context.Background(),
		"octocat",
		testProfileAnalysisEntry("octocat", "Go"),
	); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	now = now.Add(time.Minute)

	_, found, err := cache.Get(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found || len(cache.store.items) != 0 {
		t.Fatalf(
			"expired entry found = %t, item count = %d",
			found,
			len(cache.store.items),
		)
	}
}

func TestProfileAnalysisCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := newTestProfileAnalysisCache(t, 2, time.Hour)
	ctx := context.Background()
	for _, username := range []string{"alpha", "beta"} {
		if err := cache.Set(ctx, user.Username(username), testProfileAnalysisEntry(username, "Go")); err != nil {
			t.Fatalf("Set(%s) error = %v", username, err)
		}
	}
	if _, found, _ := cache.Get(ctx, "alpha"); !found {
		t.Fatal("Get(alpha) was not found")
	}
	if err := cache.Set(ctx, "gamma", testProfileAnalysisEntry("gamma", "Rust")); err != nil {
		t.Fatalf("Set(gamma) error = %v", err)
	}

	if _, found, _ := cache.Get(ctx, "beta"); found {
		t.Fatal("least recently used beta entry was not evicted")
	}
	for _, username := range []string{"alpha", "gamma"} {
		if _, found, _ := cache.Get(ctx, user.Username(username)); !found {
			t.Fatalf("Get(%s) was not found", username)
		}
	}
}

func TestProfileAnalysisCacheHonorsCancellation(t *testing.T) {
	cache := newTestProfileAnalysisCache(t, 1, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := cache.Set(ctx, "octocat", testProfileAnalysisEntry("octocat", "Go")); err == nil {
		t.Fatal("Set() error = nil, want cancellation")
	}
	if _, found, err := cache.Get(ctx, "octocat"); err == nil || found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
}

func TestProfileAnalysisCacheSupportsConcurrentAccess(t *testing.T) {
	cache := newTestProfileAnalysisCache(t, 10, time.Hour)
	var waitGroup sync.WaitGroup
	for index := 0; index < 100; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			entry := testProfileAnalysisEntry("octocat", "Go")
			if err := cache.Set(context.Background(), "octocat", entry); err != nil {
				t.Errorf("Set() error = %v", err)
			}
			if _, found, err := cache.Get(context.Background(), "octocat"); err != nil || !found {
				t.Errorf("Get() found = %t, error = %v", found, err)
			}
		}()
	}
	waitGroup.Wait()
}

func newTestProfileAnalysisCache(
	t *testing.T,
	capacity int,
	ttl time.Duration,
) *ProfileAnalysis {
	t.Helper()
	cache, err := NewProfileAnalysis(capacity, ttl)
	if err != nil {
		t.Fatalf("NewProfileAnalysis() error = %v", err)
	}
	return cache
}

func testProfileAnalysisEntry(
	username string,
	language string,
) port.ProfileAnalysisCacheEntry {
	return port.ProfileAnalysisCacheEntry{
		Analysis: profile.Analysis{
			Username: user.Username(username),
			Languages: []profile.LanguageShare{{
				Name:       language,
				Percentage: 100,
			}},
			Frameworks:           []string{"Gin"},
			RepositoriesAnalyzed: 1,
			Warnings: []profile.Warning{{
				Code:    "partial_data",
				Message: "Some repository data was unavailable",
			}},
		},
		RateLimit: port.RateLimit{Known: true, Remaining: 50},
	}
}
