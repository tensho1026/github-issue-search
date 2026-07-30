package repository

import (
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
