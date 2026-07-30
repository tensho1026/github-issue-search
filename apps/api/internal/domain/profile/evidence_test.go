package profile

import (
	"reflect"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

func TestAnalyzeSnapshotSeparatesExactSampledAndUnavailableEvidence(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	snapshot := ProfileSnapshot{
		Username:   "octocat",
		WindowFrom: now.AddDate(-1, 0, 0),
		WindowTo:   now,
		Owned: RepositoryCollection{
			Available:  true,
			Total:      5,
			TotalKnown: true,
			Limit:      2,
			HasMore:    true,
			Repositories: []RepositoryObservation{
				profileObservation(
					"octocat/api",
					"Go",
					now.Add(-10*24*time.Hour),
					map[string]int64{"Go": 800, "TypeScript": 200},
					Manifest{
						Path: "go.mod",
						Content: []byte(
							"require github.com/gin-gonic/gin v1.12.0",
						),
					},
				),
				profileObservation(
					"octocat/web",
					"TypeScript",
					now.Add(-30*24*time.Hour),
					map[string]int64{"TypeScript": 100},
					Manifest{
						Path: "package.json",
						Content: []byte(
							`{"dependencies":{"react":"19.0.0"}}`,
						),
					},
				),
			},
		},
		Contributed: RepositoryCollection{
			Available:  true,
			Total:      2,
			TotalKnown: true,
			Limit:      20,
			Repositories: []RepositoryObservation{
				profileObservation(
					"community/go-project",
					"Go",
					now.Add(-24*time.Hour),
					nil,
				),
				profileObservation(
					"community/typed-project",
					"TypeScript",
					now.Add(-48*time.Hour),
					nil,
				),
			},
		},
		Starred: RepositoryCollection{
			Available: true,
			Limit:     20,
			Repositories: []RepositoryObservation{
				profileObservation(
					"community/rust-project",
					"Rust",
					now.Add(-72*time.Hour),
					nil,
				),
			},
		},
		Forked: RepositoryCollection{
			Available:  true,
			Total:      1,
			TotalKnown: true,
			Limit:      20,
			Repositories: []RepositoryObservation{
				profileObservation(
					"octocat/python-fork",
					"Python",
					now.Add(-96*time.Hour),
					nil,
				),
			},
		},
		Contributions: ContributionSnapshot{
			Available: true,
			Commits: CountEvidence{
				Available: true,
				Value:     12,
			},
			IssuesOpened: CountEvidence{
				Available: true,
				Value:     3,
				Complete:  true,
			},
			PullRequestsOpened: CountEvidence{
				Available: true,
				Value:     5,
				Complete:  true,
			},
			PullRequestReviews: CountEvidence{
				Available: true,
				Value:     4,
			},
			RepositoriesTouched: CountEvidence{
				Available: true,
				Value:     2,
			},
		},
	}

	analysis := AnalyzeSnapshot(snapshot)

	if analysis.Username != "octocat" ||
		analysis.Window.Days != AnalysisWindowDays ||
		!analysis.Window.PublicOnly ||
		analysis.RepositoriesAnalyzed != 2 {
		t.Fatalf("analysis identity/window = %+v", analysis)
	}
	if got, want := analysis.Languages, []LanguageShare{
		{Name: "Go", Percentage: 73},
		{Name: "TypeScript", Percentage: 27},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("languages = %+v, want %+v", got, want)
	}
	if analysis.LanguageStatus != EvidenceSampled {
		t.Fatalf("language status = %q, want sampled", analysis.LanguageStatus)
	}
	if got, want := analysis.Frameworks, []string{"Gin", "React"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("frameworks = %v, want %v", got, want)
	}
	if analysis.RepositoryEvidence.Owned.Status != EvidenceSampled ||
		analysis.RepositoryEvidence.Owned.Observed != 2 ||
		analysis.RepositoryEvidence.Owned.Total == nil ||
		*analysis.RepositoryEvidence.Owned.Total != 5 ||
		analysis.RepositoryEvidence.Contributed.Status != EvidenceExact ||
		analysis.RepositoryEvidence.Starred.Status != EvidenceSampled ||
		analysis.RepositoryEvidence.Starred.Total != nil ||
		analysis.RepositoryEvidence.Forked.Status != EvidenceExact {
		t.Fatalf("repository evidence = %+v", analysis.RepositoryEvidence)
	}
	if analysis.Contributions.Commits != (CountMetric{
		Value: 12, Status: EvidenceSampled,
	}) ||
		analysis.Contributions.PullRequestsOpened != (CountMetric{
			Value: 5, Status: EvidenceExact,
		}) ||
		analysis.Contributions.PullRequestReviews.Status != EvidenceSampled {
		t.Fatalf("contributions = %+v", analysis.Contributions)
	}
	if analysis.OSSExperience.Level != "active" ||
		analysis.OSSExperience.Confidence != ConfidenceHigh ||
		!analysis.OSSExperience.PublicOnly {
		t.Fatalf("OSS experience = %+v", analysis.OSSExperience)
	}
	if len(analysis.RecentTechnologies) != 5 ||
		analysis.RecentTechnologies[0].Name != "Go" ||
		analysis.RecentTechnologies[0].RepositoryCount != 2 ||
		!reflect.DeepEqual(
			analysis.RecentTechnologies[0].RepositorySources,
			[]RepositorySource{
				RepositoryContributed,
				RepositoryOwned,
			},
		) {
		t.Fatalf("recent technologies = %+v", analysis.RecentTechnologies)
	}
	goProficiency := findProficiency(t, analysis.Proficiency, "Go")
	if goProficiency.Level != 3 ||
		goProficiency.Label != "intermediate" ||
		goProficiency.Confidence != ConfidenceMedium {
		t.Fatalf("Go proficiency = %+v", goProficiency)
	}
}

func TestAnalyzeSnapshotCanReachEveryFiveLevelProficiencyCeiling(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	owned := make([]RepositoryObservation, 0, 10)
	contributed := make([]RepositoryObservation, 0, 6)
	for index := range 10 {
		owned = append(owned, profileObservation(
			"octocat/owned-"+string(rune('a'+index)),
			"Go",
			now.Add(-time.Duration(index)*time.Hour),
			map[string]int64{"Go": 100},
		))
	}
	for index := range 6 {
		contributed = append(contributed, profileObservation(
			"community/contributed-"+string(rune('a'+index)),
			"Go",
			now.Add(-time.Duration(index)*time.Hour),
			nil,
		))
	}
	analysis := AnalyzeSnapshot(ProfileSnapshot{
		Username:   "octocat",
		WindowFrom: now.AddDate(-1, 0, 0),
		WindowTo:   now,
		Owned: RepositoryCollection{
			Available:    true,
			Repositories: owned,
			Total:        len(owned),
			TotalKnown:   true,
			Limit:        20,
		},
		Contributed: RepositoryCollection{
			Available:    true,
			Repositories: contributed,
			Total:        len(contributed),
			TotalKnown:   true,
			Limit:        20,
		},
		Forked: RepositoryCollection{
			Available:  true,
			TotalKnown: true,
			Limit:      20,
		},
	})

	goProficiency := findProficiency(t, analysis.Proficiency, "Go")
	if goProficiency.Score != 90 ||
		goProficiency.Level != 5 ||
		goProficiency.Label != "expert" ||
		goProficiency.Confidence != ConfidenceHigh {
		t.Fatalf("Go proficiency = %+v", goProficiency)
	}
}

func TestAnalyzeSnapshotRepresentsUnavailableContributionSegments(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	analysis := AnalyzeSnapshot(ProfileSnapshot{
		Username:   "new-user",
		WindowFrom: now.AddDate(-1, 0, 0),
		WindowTo:   now,
	})

	if analysis.Contributions.Commits.Status != EvidenceUnavailable ||
		analysis.RepositoryEvidence.Owned.Status != EvidenceUnavailable ||
		analysis.LanguageStatus != EvidenceUnavailable ||
		analysis.OSSExperience.Level != "unavailable" ||
		analysis.OSSExperience.Confidence != ConfidenceUnavailable ||
		len(analysis.Languages) != 0 ||
		len(analysis.Frameworks) != 0 ||
		len(analysis.Proficiency) != 0 {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestAnalyzeSnapshotMarksTruncatedLanguageEdgesAsSampled(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	observation := profileObservation(
		"octocat/polyglot",
		"Go",
		now.Add(-time.Hour),
		map[string]int64{"Go": 900, "TypeScript": 100},
	)
	observation.LanguagesComplete = false
	analysis := AnalyzeSnapshot(ProfileSnapshot{
		Username:   "octocat",
		WindowFrom: now.AddDate(-1, 0, 0),
		WindowTo:   now,
		Owned: RepositoryCollection{
			Available:    true,
			Repositories: []RepositoryObservation{observation},
			Total:        1,
			TotalKnown:   true,
			Limit:        20,
		},
		Contributed: RepositoryCollection{
			Available:  true,
			TotalKnown: true,
			Limit:      20,
		},
		Forked: RepositoryCollection{
			Available:  true,
			TotalKnown: true,
			Limit:      20,
		},
	})

	if analysis.LanguageStatus != EvidenceSampled {
		t.Fatalf("language status = %q, want sampled", analysis.LanguageStatus)
	}
	goProficiency := findProficiency(t, analysis.Proficiency, "Go")
	if goProficiency.Evidence[0].Status != EvidenceSampled {
		t.Fatalf("language evidence = %+v", goProficiency.Evidence[0])
	}
}

func TestAnalyzeSnapshotDoesNotPresentArchivedRepositoriesAsRecent(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	archived := profileObservation(
		"community/archived",
		"Rust",
		now.Add(-time.Hour),
		nil,
	)
	archived.Repository.IsArchived = true
	analysis := AnalyzeSnapshot(ProfileSnapshot{
		Username:   "octocat",
		WindowFrom: now.AddDate(-1, 0, 0),
		WindowTo:   now,
		Contributed: RepositoryCollection{
			Available:    true,
			Repositories: []RepositoryObservation{archived},
			Total:        1,
			TotalKnown:   true,
			Limit:        20,
		},
	})

	if analysis.RepositoryEvidence.Contributed.ActiveInWindow != 0 ||
		len(analysis.RecentTechnologies) != 0 {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestTopLanguageSharesKeepsPercentageTotalBounded(t *testing.T) {
	languages := make([]LanguageShare, 0, 12)
	for index := range 12 {
		languages = append(languages, LanguageShare{
			Name:       string(rune('A' + index)),
			Percentage: 8,
		})
	}
	languages[0].Percentage = 12

	got := topLanguageShares(languages, 10)

	if len(got) != 10 || got[9].Name != "Other" ||
		got[9].Percentage != 24 {
		t.Fatalf("topLanguageShares() = %+v", got)
	}
	total := 0
	for _, language := range got {
		total += language.Percentage
	}
	if total != 100 {
		t.Fatalf("percentage total = %d", total)
	}
}

func profileObservation(
	fullName string,
	mainLanguage string,
	updatedAt time.Time,
	languages map[string]int64,
	manifests ...Manifest,
) RepositoryObservation {
	return RepositoryObservation{
		Repository: repository.Summary{
			FullName:     fullName,
			MainLanguage: mainLanguage,
			UpdatedAt:    updatedAt,
		},
		Languages:         languages,
		LanguagesComplete: languages != nil,
		Manifests:         manifests,
	}
}

func findProficiency(
	t *testing.T,
	proficiency []TechnologyProficiency,
	name string,
) TechnologyProficiency {
	t.Helper()
	for _, technology := range proficiency {
		if technology.Name == name {
			return technology
		}
	}
	t.Fatalf("technology %q not found in %+v", name, proficiency)
	return TechnologyProficiency{}
}

func BenchmarkAnalyzeProfileSnapshotBounded(b *testing.B) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	owned := make([]RepositoryObservation, 0, 20)
	contributed := make([]RepositoryObservation, 0, 20)
	starred := make([]RepositoryObservation, 0, 20)
	forked := make([]RepositoryObservation, 0, 20)
	languages := map[string]int64{
		"C#":         100,
		"C++":        100,
		"Go":         100,
		"Java":       100,
		"JavaScript": 100,
		"Kotlin":     100,
		"Python":     100,
		"Ruby":       100,
		"Rust":       100,
		"TypeScript": 100,
	}
	for index := range 20 {
		suffix := string(rune('a' + index))
		owned = append(owned, profileObservation(
			"octocat/owned-"+suffix,
			"TypeScript",
			now.Add(-time.Duration(index)*time.Hour),
			languages,
			Manifest{
				Path: "package.json",
				Content: []byte(
					`{"dependencies":{"next":"16","react":"19","tailwindcss":"4"}}`,
				),
			},
		))
		contributed = append(contributed, profileObservation(
			"community/contributed-"+suffix,
			"Go",
			now.Add(-time.Duration(index)*time.Hour),
			nil,
		))
		starred = append(starred, profileObservation(
			"community/starred-"+suffix,
			"Rust",
			now.Add(-time.Duration(index)*time.Hour),
			nil,
		))
		forked = append(forked, profileObservation(
			"octocat/forked-"+suffix,
			"Python",
			now.Add(-time.Duration(index)*time.Hour),
			nil,
		))
	}
	snapshot := ProfileSnapshot{
		Username:   "octocat",
		WindowFrom: now.AddDate(0, 0, -AnalysisWindowDays),
		WindowTo:   now,
		Owned: boundedBenchmarkCollection(
			owned,
			100,
		),
		Contributed: boundedBenchmarkCollection(
			contributed,
			100,
		),
		Starred: RepositoryCollection{
			Available:    true,
			Repositories: starred,
			Limit:        20,
			HasMore:      true,
		},
		Forked: boundedBenchmarkCollection(
			forked,
			100,
		),
		Contributions: ContributionSnapshot{
			Available: true,
			Commits: CountEvidence{
				Available: true,
				Value:     500,
			},
			IssuesOpened: CountEvidence{
				Available: true,
				Value:     50,
				Complete:  true,
			},
			PullRequestsOpened: CountEvidence{
				Available: true,
				Value:     80,
				Complete:  true,
			},
			PullRequestReviews: CountEvidence{
				Available: true,
				Value:     120,
			},
			RepositoriesTouched: CountEvidence{
				Available: true,
				Value:     100,
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		profileAnalysisBenchmarkSink = AnalyzeSnapshot(snapshot)
	}
}

func boundedBenchmarkCollection(
	repositories []RepositoryObservation,
	total int,
) RepositoryCollection {
	return RepositoryCollection{
		Available:    true,
		Repositories: repositories,
		Total:        total,
		TotalKnown:   true,
		Limit:        20,
		HasMore:      true,
	}
}

var profileAnalysisBenchmarkSink Analysis
