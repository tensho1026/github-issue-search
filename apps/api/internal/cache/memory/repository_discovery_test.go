package memory

import (
	"context"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestRepositoryDiscoveryCacheReturnsIndependentCopies(t *testing.T) {
	t.Parallel()

	cache, err := NewRepositoryDiscovery(2, time.Hour)
	if err != nil {
		t.Fatalf("NewRepositoryDiscovery() error = %v", err)
	}
	entry := port.RepositoryDiscoveryCacheEntry{
		Items: []repository.DiscoveryResult{{
			Repository:   repository.Summary{FullName: "octocat/typed-service"},
			Topics:       []string{"go"},
			Technologies: []string{"Gin"},
			Difficulty: repository.PreliminaryDifficulty{
				Reasons: []string{"contributing_guide_available"},
			},
			Readiness: repository.ContributionReadiness{
				Reasons: []string{"readme_available"},
			},
			Warnings: []repository.DiscoveryWarning{
				repository.WarningREADMEContentSampled,
			},
		}},
	}
	if err := cache.Set(context.Background(), "key", entry); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	first, found, err := cache.Get(context.Background(), "key")
	if err != nil || !found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
	first.Items[0].Topics[0] = "mutated"
	first.Items[0].Technologies[0] = "mutated"
	first.Items[0].Difficulty.Reasons[0] = "mutated"
	first.Items[0].Readiness.Reasons[0] = "mutated"
	first.Items[0].Warnings[0] = "mutated"

	second, found, err := cache.Get(context.Background(), "key")
	if err != nil || !found {
		t.Fatalf("Get() found = %t, error = %v", found, err)
	}
	if second.Items[0].Topics[0] != "go" ||
		second.Items[0].Technologies[0] != "Gin" ||
		second.Items[0].Difficulty.Reasons[0] !=
			"contributing_guide_available" ||
		second.Items[0].Readiness.Reasons[0] != "readme_available" ||
		second.Items[0].Warnings[0] !=
			repository.WarningREADMEContentSampled {
		t.Fatalf("cached entry was mutated = %+v", second.Items[0])
	}
}

func TestRepositoryDiscoveryCacheHonorsBoundsExpiryAndCancellation(
	t *testing.T,
) {
	t.Parallel()

	cache, err := NewRepositoryDiscovery(1, time.Minute)
	if err != nil {
		t.Fatalf("NewRepositoryDiscovery() error = %v", err)
	}
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	cache.store.now = func() time.Time { return now }
	ctx := context.Background()
	if err := cache.Set(ctx, "first", port.RepositoryDiscoveryCacheEntry{}); err != nil {
		t.Fatalf("Set(first) error = %v", err)
	}
	if err := cache.Set(ctx, "second", port.RepositoryDiscoveryCacheEntry{}); err != nil {
		t.Fatalf("Set(second) error = %v", err)
	}
	if _, found, _ := cache.Get(ctx, "first"); found {
		t.Fatal("first entry was not evicted")
	}
	now = now.Add(time.Minute)
	if _, found, _ := cache.Get(ctx, "second"); found {
		t.Fatal("expired second entry was found")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := cache.Set(
		cancelled,
		"cancelled",
		port.RepositoryDiscoveryCacheEntry{},
	); err == nil {
		t.Fatal("Set(cancelled) error = nil")
	}
}
