package issue

import (
	"slices"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

func TestExclusionReasonsAcceptsEligibleCandidate(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	criteria := testCriteria(t, SearchCriteriaOptions{
		Username:   "octocat",
		Languages:  []string{"Go"},
		Frameworks: []string{"Gin"},
	})
	candidate := eligibleCandidate(now)

	if reasons := ExclusionReasons(criteria, candidate, now); len(reasons) != 0 {
		t.Fatalf("ExclusionReasons() = %v", reasons)
	}
}

func TestExclusionReasonsReportsEveryApplicableReasonInStableOrder(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	minimumStars := 100
	maximumDifficulty := 2
	updatedWithinDays := 30
	includeEnglish := false
	criteria := testCriteria(t, SearchCriteriaOptions{
		Username:          "octocat",
		Languages:         []string{"Rust"},
		Frameworks:        []string{"React"},
		Labels:            []string{"beginner"},
		MinimumStars:      &minimumStars,
		MaximumDifficulty: &maximumDifficulty,
		UpdatedWithinDays: &updatedWithinDays,
		IncludeEnglish:    &includeEnglish,
	})
	candidate := eligibleCandidate(now)
	candidate.Repository.IsArchived = true
	candidate.Repository.Stars = 5
	candidate.Repository.MainLanguage = "Go"
	candidate.Repository.UpdatedAt = now.AddDate(0, 0, -31)
	candidate.Issue.State = "closed"
	candidate.Issue.IsPullRequest = true
	candidate.Issue.Assignees = []string{"maintainer"}
	candidate.Issue.AuthorLogin = "dependabot[bot]"
	candidate.Issue.AuthorType = AuthorBot
	candidate.Issue.Title = "Update leaked token"
	candidate.Issue.Body = "access_token=super-secret-token-value"
	candidate.Issue.Labels = []string{"hard"}
	candidate.Issue.UpdatedAt = now.AddDate(0, 0, -31)

	got := ExclusionReasons(criteria, candidate, now)
	want := []ExclusionReason{
		ExclusionPullRequest,
		ExclusionNotOpen,
		ExclusionArchivedRepository,
		ExclusionAlreadyAssigned,
		ExclusionStale,
		ExclusionBotGenerated,
		ExclusionSensitiveContent,
		ExclusionInsufficientDescription,
		ExclusionBelowMinimumStars,
		ExclusionLanguageMismatch,
		ExclusionFrameworkMismatch,
		ExclusionLabelMismatch,
		ExclusionAboveMaximumDifficulty,
		ExclusionEnglishNotAllowed,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ExclusionReasons() = %v, want %v", got, want)
	}
}

func TestExclusionReasonsAllowsArchivedAndEnglishWhenRequested(t *testing.T) {
	now := time.Now().UTC()
	excludeArchived := false
	includeEnglish := true
	criteria := testCriteria(t, SearchCriteriaOptions{
		Username:        "octocat",
		ExcludeArchived: &excludeArchived,
		IncludeEnglish:  &includeEnglish,
	})
	candidate := eligibleCandidate(now)
	candidate.Repository.IsArchived = true

	if reasons := ExclusionReasons(criteria, candidate, now); len(reasons) != 0 {
		t.Fatalf("ExclusionReasons() = %v", reasons)
	}
}

func TestEstimateDifficultyUsesExplicitLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   Difficulty
	}{
		{name: "unknown defaults to medium", labels: []string{"bug"}, want: 3},
		{name: "good first issue", labels: []string{"good first issue"}, want: 1},
		{name: "documentation", labels: []string{"Documentation"}, want: 2},
		{name: "numeric", labels: []string{"difficulty: 4"}, want: 4},
		{
			name:   "highest explicit risk wins",
			labels: []string{"good first issue", "complex"},
			want:   4,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EstimateDifficulty(test.labels); got != test.want {
				t.Fatalf("EstimateDifficulty() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCandidateSafetyHeuristics(t *testing.T) {
	if !IsBotGenerated(Summary{AuthorType: "Bot"}) ||
		!IsBotGenerated(Summary{AuthorLogin: "renovate[bot]"}) ||
		!IsBotGenerated(Summary{Title: "chore(deps): update modules"}) ||
		IsBotGenerated(Summary{AuthorLogin: "octocat", Title: "Fix dependencies"}) {
		t.Fatal("IsBotGenerated() returned an unexpected result")
	}

	for _, content := range []string{
		"-----BEGIN PRIVATE KEY-----",
		"ghp_123456789012345678901234567890",
		"AKIA1234567890ABCDEF",
		"password=correct-horse-battery-staple",
	} {
		if !ContainsSensitiveContent("", content) {
			t.Fatalf("ContainsSensitiveContent(%q) = false", content)
		}
	}
	if ContainsSensitiveContent(
		"Improve password validation",
		"Document how token expiration works without including any credentials.",
	) {
		t.Fatal("ordinary security terminology was treated as a credential")
	}

	if !HasInsufficientDescription("Too short") {
		t.Fatal("short description was accepted")
	}
	if HasInsufficientDescription(
		"Describe the observed behavior, expected behavior, reproduction steps, and acceptance criteria.",
	) {
		t.Fatal("meaningful English description was rejected")
	}
	if HasInsufficientDescription(
		"この問題を再現するための手順と期待する動作を具体的に説明し、完了条件と関連情報も記載します。",
	) {
		t.Fatal("meaningful non-space-separated description was rejected")
	}

	if !IsProbablyEnglish(
		"This issue explains the expected behavior and complete reproduction steps.",
	) {
		t.Fatal("English content was not detected")
	}
	if IsProbablyEnglish("この不具合の再現手順と期待する動作を詳しく説明します。") {
		t.Fatal("Japanese content was detected as English")
	}
}

func TestFrameworkMatchingUsesTermBoundaries(t *testing.T) {
	now := time.Now().UTC()
	criteria := testCriteria(t, SearchCriteriaOptions{
		Username:   "octocat",
		Frameworks: []string{"Gin"},
	})
	candidate := eligibleCandidate(now)
	candidate.Repository.Description = "A production logging service"
	candidate.Issue.Title = "Improve structured logging output"
	candidate.Issue.Body = "The logging output should include request metadata and clear acceptance criteria."
	if !slices.Contains(
		ExclusionReasons(criteria, candidate, now),
		ExclusionFrameworkMismatch,
	) {
		t.Fatal("Gin matched inside logging")
	}
	candidate.Issue.Body = "The Gin handler needs validation, tests, and documented acceptance criteria."
	if slices.Contains(
		ExclusionReasons(criteria, candidate, now),
		ExclusionFrameworkMismatch,
	) {
		t.Fatal("Gin term did not match")
	}
}

func eligibleCandidate(now time.Time) Candidate {
	return Candidate{
		Repository: repository.Summary{
			Owner:        "example",
			Name:         "example-api",
			FullName:     "example/example-api",
			Description:  "A production Gin service",
			URL:          "https://github.com/example/example-api",
			MainLanguage: "Go",
			Stars:        120,
			UpdatedAt:    now.Add(-time.Hour),
			PushedAt:     now.Add(-time.Hour),
		},
		Issue: Summary{
			Number:      123,
			Title:       "Add request validation to the Gin API",
			Body:        "The handler needs request validation, clear errors, regression tests, and documented acceptance criteria.",
			URL:         "https://github.com/example/example-api/issues/123",
			State:       StateOpen,
			Labels:      []string{"good first issue"},
			AuthorLogin: "contributor",
			AuthorType:  AuthorHuman,
			CreatedAt:   now.Add(-48 * time.Hour),
			UpdatedAt:   now.Add(-time.Hour),
		},
	}
}

func testCriteria(
	t *testing.T,
	options SearchCriteriaOptions,
) SearchCriteria {
	t.Helper()
	criteria, err := NewSearchCriteria(options)
	if err != nil {
		t.Fatalf("NewSearchCriteria() error = %v", err)
	}
	return criteria
}
