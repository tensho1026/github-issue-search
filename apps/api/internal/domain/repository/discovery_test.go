package repository

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeDiscoveryExplainsReadyJapaneseRepository(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	candidate := discoveryCandidateFixture(now)
	candidate.Topics = []string{"developer-tools", "react", "documentation"}
	candidate.GoodFirstIssues = 8
	candidate.HelpWantedIssues = 4
	candidate.HasDiscussions = true
	candidate.HasCodeOfConduct = true
	candidate.HasSecurityPolicy = true
	japanese := strings.Repeat("これは日本語の説明です。", 20)

	result := AnalyzeDiscovery(
		candidate,
		DiscoveryEnrichment{
			Available:              true,
			READMEAvailable:        true,
			READMEContentAvailable: true,
			READMEText:             "React\n" + japanese,
			ContributingAvailable:  true,
		},
		[]FilterValue{"React"},
		now,
	)

	if !result.Documentation.JapaneseREADME.Detected {
		t.Fatal("Japanese README detected = false, want true")
	}
	if result.Documentation.JapaneseREADME.Confidence != ConfidenceHigh {
		t.Fatalf(
			"Japanese README confidence = %q, want %q",
			result.Documentation.JapaneseREADME.Confidence,
			ConfidenceHigh,
		)
	}
	if result.Difficulty.Level != 1 {
		t.Fatalf("Difficulty.Level = %d, want 1", result.Difficulty.Level)
	}
	if result.Readiness.Band != ReadinessReady {
		t.Fatalf("Readiness.Band = %q, want %q", result.Readiness.Band, ReadinessReady)
	}
	if len(result.Technologies) != 1 || result.Technologies[0] != "React" {
		t.Fatalf("Technologies = %#v, want React", result.Technologies)
	}
	if result.Category != CategoryDocumentation {
		t.Fatalf("Category = %q, want %q", result.Category, CategoryDocumentation)
	}
}

func TestAnalyzeDiscoveryMarksUnavailableAndSampledEvidence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	candidate := discoveryCandidateFixture(now)

	unavailable := AnalyzeDiscovery(
		candidate,
		DiscoveryEnrichment{},
		nil,
		now,
	)
	if unavailable.Documentation.Status != EvidenceUnavailable ||
		unavailable.Documentation.JapaneseREADME.Confidence != ConfidenceUnavailable {
		t.Fatalf("unavailable documentation = %#v", unavailable.Documentation)
	}
	if len(unavailable.Warnings) != 1 ||
		unavailable.Warnings[0] != WarningEnrichmentUnavailable {
		t.Fatalf("unavailable warnings = %#v", unavailable.Warnings)
	}

	sampled := AnalyzeDiscovery(
		candidate,
		DiscoveryEnrichment{
			Available:              true,
			READMEAvailable:        true,
			READMEContentAvailable: true,
			READMEText:             strings.Repeat("日本語", 20),
			READMEContentSampled:   true,
		},
		nil,
		now,
	)
	if sampled.Documentation.Status != EvidenceSampled ||
		sampled.Documentation.JapaneseREADME.Status != EvidenceSampled {
		t.Fatalf("sampled documentation = %#v", sampled.Documentation)
	}
}

func TestSortDiscoveryResultsIsDeterministic(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	results := []DiscoveryResult{
		{
			Repository: Summary{FullName: "zeta/repo", Stars: 10, PushedAt: now},
			Readiness:  ContributionReadiness{Score: 50},
		},
		{
			Repository: Summary{FullName: "alpha/repo", Stars: 20, PushedAt: now},
			Readiness:  ContributionReadiness{Score: 50},
		},
		{
			Repository: Summary{FullName: "ready/repo", Stars: 1, PushedAt: now},
			Readiness:  ContributionReadiness{Score: 90},
		},
	}

	SortDiscoveryResults(results)

	got := []string{
		results[0].Repository.FullName,
		results[1].Repository.FullName,
		results[2].Repository.FullName,
	}
	want := []string{"ready/repo", "alpha/repo", "zeta/repo"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestClassifyDiscoveryCategoryUsesDocumentedPriority(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		description string
		topics      []string
		want        Category
	}{
		{
			name:   "security has highest priority",
			topics: []string{"security", "database", "documentation"},
			want:   CategorySecurity,
		},
		{name: "data", topics: []string{"machine-learning"}, want: CategoryData},
		{
			name:   "infrastructure",
			topics: []string{"kubernetes"},
			want:   CategoryInfrastructure,
		},
		{
			name:   "documentation",
			topics: []string{"documentation"},
			want:   CategoryDocumentation,
		},
		{
			name:        "education",
			description: "An education course",
			want:        CategoryEducation,
		},
		{name: "framework", topics: []string{"web-framework"}, want: CategoryFramework},
		{name: "library", topics: []string{"sdk"}, want: CategoryLibrary},
		{name: "tooling", topics: []string{"linter"}, want: CategoryTooling},
		{name: "application fallback", want: CategoryApplication},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			candidate := DiscoveryCandidate{
				Repository: Summary{Description: testCase.description},
				Topics:     testCase.topics,
			}
			if got := ClassifyDiscoveryCategory(candidate); got != testCase.want {
				t.Fatalf(
					"ClassifyDiscoveryCategory() = %q, want %q",
					got,
					testCase.want,
				)
			}
		})
	}
}

func discoveryCandidateFixture(now time.Time) DiscoveryCandidate {
	return DiscoveryCandidate{
		Repository: Summary{
			ID:           1,
			Owner:        "octocat",
			Name:         "typed-service",
			FullName:     "octocat/typed-service",
			Description:  "A developer tool",
			URL:          "https://github.com/octocat/typed-service",
			MainLanguage: "TypeScript",
			Stars:        120,
			Forks:        12,
			OpenIssues:   8,
			UpdatedAt:    now.Add(-24 * time.Hour),
			PushedAt:     now.Add(-24 * time.Hour),
		},
		License:          "MIT",
		LicenseKnown:     true,
		HasIssuesEnabled: true,
	}
}

func BenchmarkAnalyzeRepositoryDiscoveryBounded(b *testing.B) {
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	candidates := make([]DiscoveryCandidate, MaximumDiscoveryEnrichmentResults)
	enrichments := make(
		[]DiscoveryEnrichment,
		MaximumDiscoveryEnrichmentResults,
	)
	technologies := make(
		[]FilterValue,
		MaximumDiscoveryFilterValues,
	)
	for index := range technologies {
		technologies[index] = FilterValue(
			"technology-" + strconv.Itoa(index),
		)
	}
	for index := range candidates {
		candidate := discoveryCandidateFixture(now)
		candidate.Repository.ID = int64(index + 1)
		candidate.Repository.Name = "repository-" + strconv.Itoa(index)
		candidate.Repository.FullName = "benchmark/" + candidate.Repository.Name
		candidate.Topics = []string{
			"developer-tools",
			"documentation",
			"technology-0",
			"technology-9",
		}
		candidate.GoodFirstIssues = 10
		candidate.HelpWantedIssues = 10
		candidate.HasDiscussions = true
		candidate.HasCodeOfConduct = true
		candidate.HasSecurityPolicy = true
		candidates[index] = candidate
		enrichments[index] = DiscoveryEnrichment{
			Available:              true,
			READMEAvailable:        true,
			READMEContentAvailable: true,
			READMEText: strings.Repeat(
				"日本語 documentation technology-5 ",
				2048,
			),
			ContributingAvailable: true,
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		results := make(
			[]DiscoveryResult,
			0,
			MaximumDiscoveryEnrichmentResults,
		)
		for index := range candidates {
			results = append(
				results,
				AnalyzeDiscovery(
					candidates[index],
					enrichments[index],
					technologies,
					now,
				),
			)
		}
		SortDiscoveryResults(results)
		repositoryDiscoveryBenchmarkSink = results
	}
}

var repositoryDiscoveryBenchmarkSink []DiscoveryResult
