package issue

import (
	"errors"
	"strings"
	"testing"
)

func TestNewSearchCriteriaAppliesMVPDefaults(t *testing.T) {
	criteria, err := NewSearchCriteria(SearchCriteriaOptions{
		Username: "octocat",
	})
	if err != nil {
		t.Fatalf("NewSearchCriteria() error = %v", err)
	}
	_, maximumEffortConfigured := criteria.MaximumEffort()

	if criteria.Username() != "octocat" ||
		criteria.MinimumStars() != DefaultMinimumStars ||
		criteria.MaximumDifficulty().Int() != DefaultMaximumDifficulty ||
		criteria.UpdatedWithinDays() != DefaultUpdatedWithinDays ||
		criteria.IncludesDocumentation() ||
		maximumEffortConfigured ||
		!criteria.IncludesEnglish() ||
		!criteria.ExcludesArchived() {
		t.Fatalf("criteria defaults = %+v", criteria)
	}
	labels := criteria.Labels()
	if len(labels) != 2 ||
		labels[0] != "good first issue" ||
		labels[1] != "help wanted" {
		t.Fatalf("labels = %v", labels)
	}
}

func TestNewSearchCriteriaNormalizesCollectionsAndCanonicalKey(t *testing.T) {
	minimumStars := 0
	maximumDifficulty := 5
	updatedWithinDays := 30
	includeDocumentation := true
	includeEnglish := false
	excludeArchived := false
	maximumEffort := string(EffortHalfDay)

	first, err := NewSearchCriteria(SearchCriteriaOptions{
		Username:             "OctoCat",
		Languages:            []string{" TypeScript ", "go", "GO"},
		Frameworks:           []string{"React", " Gin "},
		Labels:               []string{"help wanted", "Good First Issue"},
		MinimumStars:         &minimumStars,
		MaximumDifficulty:    &maximumDifficulty,
		MaximumEffort:        &maximumEffort,
		UpdatedWithinDays:    &updatedWithinDays,
		IncludeDocumentation: &includeDocumentation,
		IncludeEnglish:       &includeEnglish,
		ExcludeArchived:      &excludeArchived,
	})
	if err != nil {
		t.Fatalf("NewSearchCriteria(first) error = %v", err)
	}
	second, err := NewSearchCriteria(SearchCriteriaOptions{
		Username:             "octocat",
		Languages:            []string{"Go", "typescript"},
		Frameworks:           []string{"gin", "react"},
		Labels:               []string{"documentation", "good first issue", "HELP WANTED"},
		MinimumStars:         &minimumStars,
		MaximumDifficulty:    &maximumDifficulty,
		MaximumEffort:        nil,
		UpdatedWithinDays:    &updatedWithinDays,
		IncludeDocumentation: &includeDocumentation,
		IncludeEnglish:       &includeEnglish,
		ExcludeArchived:      &excludeArchived,
	})
	if err != nil {
		t.Fatalf("NewSearchCriteria(second) error = %v", err)
	}

	if got := first.Languages(); len(got) != 2 ||
		!strings.EqualFold(got[0].String(), "go") ||
		!strings.EqualFold(got[1].String(), "typescript") {
		t.Fatalf("languages = %v", got)
	}
	if got := first.Labels(); len(got) != 3 {
		t.Fatalf("labels = %v", got)
	}
	if first.CacheKey() != second.CacheKey() {
		t.Fatalf("cache keys differ:\n%s\n%s", first.CacheKey(), second.CacheKey())
	}
	if effort, configured := first.MaximumEffort(); !configured ||
		effort != EffortHalfDay {
		t.Fatalf("maximum effort = %q, %t", effort, configured)
	}
	if !strings.HasPrefix(first.CacheKey(), "github:issue-search:") ||
		len(strings.TrimPrefix(first.CacheKey(), "github:issue-search:")) != 64 {
		t.Fatalf("cache key = %q", first.CacheKey())
	}

	languages := first.Languages()
	languages[0] = "mutated"
	if first.Languages()[0] == "mutated" {
		t.Fatal("criteria collection was mutated through its accessor")
	}
}

func TestNewSearchCriteriaRejectsInvalidInputs(t *testing.T) {
	negativeStars := -1
	zeroDifficulty := 0
	tooManyDays := MaximumUpdatedWithinDays + 1
	includeDocumentation := true
	invalidEffort := "weekend"

	tests := []struct {
		name    string
		options SearchCriteriaOptions
	}{
		{
			name: "unsupported maximum effort",
			options: SearchCriteriaOptions{
				Username:      "octocat",
				MaximumEffort: &invalidEffort,
			},
		},
		{
			name:    "invalid username",
			options: SearchCriteriaOptions{Username: "invalid--user"},
		},
		{
			name: "too many languages",
			options: SearchCriteriaOptions{
				Username:  "octocat",
				Languages: make([]string, MaximumFilterValues+1),
			},
		},
		{
			name: "blank language",
			options: SearchCriteriaOptions{
				Username:  "octocat",
				Languages: []string{" "},
			},
		},
		{
			name: "unsafe label",
			options: SearchCriteriaOptions{
				Username: "octocat",
				Labels:   []string{`bug" archived:true`},
			},
		},
		{
			name: "oversized framework",
			options: SearchCriteriaOptions{
				Username:   "octocat",
				Frameworks: []string{strings.Repeat("a", MaximumFilterValueRunes+1)},
			},
		},
		{
			name: "negative stars",
			options: SearchCriteriaOptions{
				Username:     "octocat",
				MinimumStars: &negativeStars,
			},
		},
		{
			name: "difficulty below range",
			options: SearchCriteriaOptions{
				Username:          "octocat",
				MaximumDifficulty: &zeroDifficulty,
			},
		},
		{
			name: "updated days above range",
			options: SearchCriteriaOptions{
				Username:          "octocat",
				UpdatedWithinDays: &tooManyDays,
			},
		},
		{
			name: "documentation exceeds label bound",
			options: SearchCriteriaOptions{
				Username:             "octocat",
				Labels:               distinctValues(MaximumFilterValues),
				IncludeDocumentation: &includeDocumentation,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSearchCriteria(test.options)
			if !errors.Is(err, ErrInvalidSearchCriteria) {
				t.Fatalf("NewSearchCriteria() error = %v", err)
			}
		})
	}
}

func TestNewPaginationValidatesBounds(t *testing.T) {
	pagination, err := NewPagination(2, MaximumPageSize)
	if err != nil {
		t.Fatalf("NewPagination() error = %v", err)
	}
	if pagination.Page != 2 || pagination.PerPage != MaximumPageSize {
		t.Fatalf("pagination = %+v", pagination)
	}

	for _, input := range [][2]int{{0, 20}, {1, 0}, {1, MaximumPageSize + 1}} {
		if _, err := NewPagination(input[0], input[1]); !errors.Is(
			err,
			ErrInvalidSearchCriteria,
		) {
			t.Fatalf("NewPagination(%d, %d) error = %v", input[0], input[1], err)
		}
	}
}

func distinctValues(count int) []string {
	values := make([]string, count)
	for index := range count {
		values[index] = "label-" + string(rune('a'+index))
	}
	return values
}
