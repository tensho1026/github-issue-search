package repository

import (
	"strings"
	"testing"
)

func TestNewDiscoveryCriteriaCanonicalizesEquivalentInput(t *testing.T) {
	t.Parallel()

	first, err := NewDiscoveryCriteria(DiscoveryCriteriaOptions{
		Languages:    []string{" Go ", "TypeScript", "go"},
		Technologies: []string{"React", "Gin"},
		Licenses:     []string{"mit", "Apache-2.0"},
		Categories:   []string{"tooling", "library"},
	})
	if err != nil {
		t.Fatalf("NewDiscoveryCriteria() error = %v", err)
	}
	second, err := NewDiscoveryCriteria(DiscoveryCriteriaOptions{
		Languages:    []string{"typescript", "go"},
		Technologies: []string{"gin", "react"},
		Licenses:     []string{"Apache-2.0", "MIT"},
		Categories:   []string{"library", "tooling"},
	})
	if err != nil {
		t.Fatalf("NewDiscoveryCriteria() error = %v", err)
	}

	if first.CacheKey() != second.CacheKey() {
		t.Fatalf("equivalent cache keys differ: %q != %q", first.CacheKey(), second.CacheKey())
	}
	if got := first.ForkPolicy(); got != ForkPolicyExclude {
		t.Fatalf("ForkPolicy() = %q, want %q", got, ForkPolicyExclude)
	}
	if !first.ExcludesArchived() {
		t.Fatal("ExcludesArchived() = false, want true")
	}
}

func TestNewDiscoveryCriteriaRejectsBeforeConstruction(t *testing.T) {
	t.Parallel()

	negative := -1
	lowMaximum := 1
	highMinimum := 2
	tooDifficult := 6
	tooReady := 101
	unsupportedFork := "sometimes"
	cases := []struct {
		name    string
		options DiscoveryCriteriaOptions
	}{
		{name: "unsafe value", options: DiscoveryCriteriaOptions{
			Languages: []string{`Go" stars:>0`},
		}},
		{name: "unsupported license", options: DiscoveryCriteriaOptions{
			Licenses: []string{"WTFPL"},
		}},
		{name: "unsupported category", options: DiscoveryCriteriaOptions{
			Categories: []string{"social"},
		}},
		{name: "negative stars", options: DiscoveryCriteriaOptions{
			MinimumStars: &negative,
		}},
		{name: "inverted issues", options: DiscoveryCriteriaOptions{
			MinimumOpenIssues: &highMinimum,
			MaximumOpenIssues: &lowMaximum,
		}},
		{name: "difficulty", options: DiscoveryCriteriaOptions{
			MaximumDifficulty: &tooDifficult,
		}},
		{name: "readiness", options: DiscoveryCriteriaOptions{
			MinimumReadiness: &tooReady,
		}},
		{name: "fork policy", options: DiscoveryCriteriaOptions{
			ForkPolicy: &unsupportedFork,
		}},
		{name: "collection bound", options: DiscoveryCriteriaOptions{
			Technologies: strings.Fields(
				"one two three four five six seven eight nine ten eleven",
			),
		}},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewDiscoveryCriteria(testCase.options); err == nil {
				t.Fatal("NewDiscoveryCriteria() error = nil, want error")
			}
		})
	}
}

func TestNewDiscoveryPaginationBounds(t *testing.T) {
	t.Parallel()

	if _, err := NewDiscoveryPagination(1, MaximumDiscoveryPageSize); err != nil {
		t.Fatalf("NewDiscoveryPagination() error = %v", err)
	}
	if _, err := NewDiscoveryPagination(0, 20); err == nil {
		t.Fatal("NewDiscoveryPagination(page=0) error = nil")
	}
	if _, err := NewDiscoveryPagination(1, MaximumDiscoveryPageSize+1); err == nil {
		t.Fatal("NewDiscoveryPagination(oversized) error = nil")
	}
}
