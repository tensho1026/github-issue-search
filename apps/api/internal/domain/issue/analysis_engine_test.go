package issue

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

func TestAnalyzeIssueAssessesConcreteBugQualitySignals(t *testing.T) {
	body := `
## Problem description
The login dialog submits twice and the request fails.
## Expected behavior
One request should be sent.
## Current behavior
Two requests are currently sent.
## Steps to reproduce
1. Open the dialog.
2. Select submit.
## Proposed solution
Debounce the handler in src/components/LoginDialog.tsx.
## Screenshot
![duplicate request](/assets/request.png)
## Test plan
Add a regression test and verify one request.
## Acceptance criteria
- [ ] Keyboard submission sends one request.
`
	result := AnalyzeIssue(AnalysisInput{
		Candidate: analysisCandidate(
			"Fix duplicate login request",
			body,
			[]string{"bug", "frontend"},
			"TypeScript",
			4,
		),
		Dependencies: []string{"react", "typescript", "vitest"},
	})

	if result.Category.Primary != CategoryBug {
		t.Fatalf("category = %+v", result.Category)
	}
	if result.Quality.Score != 100 ||
		result.Quality.Confidence != ConfidenceHigh {
		t.Fatalf("quality = %+v", result.Quality)
	}
	for _, key := range qualitySignalOrder {
		signal := qualitySignalByKey(t, result.Quality, key)
		if signal.State != SignalPresent {
			t.Errorf("signal %q = %q", key, signal.State)
		}
		if len(signal.Evidence) == 0 {
			t.Errorf("signal %q has no evidence", key)
		}
	}
	if result.Scope.DatabaseChange != SignalAbsent {
		t.Fatalf("database change = %q", result.Scope.DatabaseChange)
	}
}

func TestAnalyzeIssueDistinguishesUnknownAbsentAndNotApplicable(t *testing.T) {
	empty := AnalyzeIssue(AnalysisInput{
		Candidate: analysisCandidate(
			"Update behavior",
			"",
			nil,
			"",
			0,
		),
	})
	for _, signal := range empty.Quality.Signals {
		if signal.State != SignalUnknown {
			t.Fatalf("empty signal %q = %q", signal.Key, signal.State)
		}
	}
	if empty.Scope.DatabaseChange != SignalUnknown ||
		empty.Confidence != ConfidenceLow {
		t.Fatalf("empty analysis = %+v", empty)
	}

	documentation := AnalyzeIssue(AnalysisInput{
		Candidate: analysisCandidate(
			"Document the API",
			"## Description\nAdd a clear API guide for new contributors and update README wording.",
			[]string{"documentation"},
			"Go",
			0,
		),
	})
	if got := qualitySignalByKey(
		t,
		documentation.Quality,
		QualityExpectedBehavior,
	).State; got != SignalNotApplicable {
		t.Fatalf("expected behavior state = %q", got)
	}
	if got := qualitySignalByKey(
		t,
		documentation.Quality,
		QualityTestMethod,
	).State; got != SignalAbsent {
		t.Fatalf("test method state = %q", got)
	}
	if documentation.Scope.DatabaseChange != SignalAbsent {
		t.Fatalf("database change = %q", documentation.Scope.DatabaseChange)
	}
}

func TestAnalyzeIssueClassifiesEverySupportedCategory(t *testing.T) {
	tests := []struct {
		label    string
		category Category
	}{
		{label: "bug", category: CategoryBug},
		{label: "enhancement", category: CategoryFeature},
		{label: "documentation", category: CategoryDocumentation},
		{label: "testing", category: CategoryTesting},
		{label: "refactoring", category: CategoryRefactoring},
		{label: "performance", category: CategoryPerformance},
		{label: "accessibility", category: CategoryAccessibility},
		{label: "ui", category: CategoryUI},
		{label: "backend", category: CategoryBackend},
		{label: "devops", category: CategoryDevOps},
		{label: "localization", category: CategoryLocalization},
	}
	for _, test := range tests {
		t.Run(string(test.category), func(t *testing.T) {
			result := AnalyzeIssue(AnalysisInput{
				Candidate: analysisCandidate(
					"Improve the project",
					"Provide enough implementation details for contributors to complete this scoped change.",
					[]string{test.label},
					"Go",
					0,
				),
			})
			if result.Category.Primary != test.category ||
				result.Category.Confidence != ConfidenceHigh {
				t.Fatalf("category = %+v", result.Category)
			}
			if len(result.Category.Matches) == 0 ||
				result.Category.Matches[0] != test.category {
				t.Fatalf("matches = %v", result.Category.Matches)
			}
		})
	}
}

func TestAnalyzeIssueInfersAndDeduplicatesRequiredTechnologies(t *testing.T) {
	input := AnalysisInput{
		Candidate: analysisCandidate(
			"Add React GraphQL support",
			"Implement the TypeScript API endpoint with Redis caching and regression testing. An ongoing task is unrelated.",
			[]string{"enhancement"},
			"Go",
			0,
		),
		Dependencies: []string{
			"react",
			"react-dom",
			"github.com/redis/go-redis/v9",
			"spring",
			"typescript",
			"vitest",
			"react",
		},
	}
	result := AnalyzeIssue(input)

	for _, name := range []string{
		"Go", "GraphQL", "React", "Redis", "Testing", "TypeScript",
	} {
		if !hasTechnology(result.RequiredTechnologies, name) {
			t.Errorf("missing technology %q: %+v", name, result.RequiredTechnologies)
		}
	}
	if countTechnology(result.RequiredTechnologies, "React") != 1 {
		t.Fatalf("React was not deduplicated: %+v", result.RequiredTechnologies)
	}
	react := technologyByName(t, result.RequiredTechnologies, "React")
	if react.Confidence != ConfidenceHigh ||
		len(react.Evidence) != 2 {
		t.Fatalf("React = %+v", react)
	}
	if result.RequiredTechnologies[0].Confidence != ConfidenceHigh {
		t.Fatalf("technology ordering = %+v", result.RequiredTechnologies)
	}
	if hasTechnology(result.RequiredTechnologies, "PostgreSQL") {
		t.Fatalf("substring dependency produced PostgreSQL: %+v", result.RequiredTechnologies)
	}
}

func TestAnalyzeIssueMapsDifficultyBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		body      string
		labels    []string
		wantLevel int
		wantLabel string
	}{
		{
			name:      "very easy documentation",
			title:     "Fix README typo",
			body:      "Correct one misspelled word in README.md and verify the rendered documentation.",
			labels:    []string{"documentation", "good first issue"},
			wantLevel: 1,
			wantLabel: "Very Easy",
		},
		{
			name:      "easy bug",
			title:     "Fix incorrect validation message",
			body:      "The current validation message is incorrect. Expected behavior is a clear error for the user.",
			labels:    []string{"bug"},
			wantLevel: 2,
			wantLabel: "Easy",
		},
		{
			name:      "medium backend",
			title:     "Add API endpoint",
			body:      "Implement one API endpoint and add a test plan for request validation.",
			labels:    []string{"backend"},
			wantLevel: 3,
			wantLabel: "Medium",
		},
		{
			name:      "hard database migration",
			title:     "Add account storage",
			body:      "Implement backend database access and a database schema migration for the accounts table.",
			labels:    []string{"backend"},
			wantLevel: 4,
			wantLabel: "Hard",
		},
		{
			name:      "very hard security",
			title:     "Fix authentication vulnerability",
			body:      "Resolve a security vulnerability across authentication services and add regression tests.",
			labels:    []string{"bug"},
			wantLevel: 5,
			wantLabel: "Very Hard",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := AnalyzeIssue(AnalysisInput{
				Candidate: analysisCandidate(
					test.title,
					test.body,
					test.labels,
					"Go",
					0,
				),
			})
			if result.Difficulty.Level.Int() != test.wantLevel ||
				result.Difficulty.Label != test.wantLabel {
				t.Fatalf("difficulty = %+v", result.Difficulty)
			}
		})
	}
}

func TestAnalyzeIssueMapsEveryEffortBand(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		body   string
		labels []string
		want   EffortBand
	}{
		{
			name:   "thirty minutes",
			title:  "Fix README typo",
			body:   "Correct one typo in README.md and verify the documentation.",
			labels: []string{"documentation", "good first issue"},
			want:   EffortThirtyMinutes,
		},
		{
			name:   "two hours",
			title:  "Fix validation bug",
			body:   "The current validation message fails. Expected behavior is a clear error.",
			labels: []string{"bug"},
			want:   EffortTwoHours,
		},
		{
			name:   "half day",
			title:  "Update responsive button",
			body:   "Change one UI component and its responsive CSS behavior.",
			labels: []string{"ui"},
			want:   EffortHalfDay,
		},
		{
			name:   "one day",
			title:  "Add API endpoint",
			body:   "Implement one backend API endpoint and request validation.",
			labels: []string{"backend"},
			want:   EffortOneDay,
		},
		{
			name:   "three days",
			title:  "Resolve security architecture issue",
			body:   "Change authentication architecture across multiple services and add security regression tests.",
			labels: []string{"bug"},
			want:   EffortThreeDays,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := AnalyzeIssue(AnalysisInput{
				Candidate: analysisCandidate(
					test.title,
					test.body,
					test.labels,
					"Go",
					0,
				),
			})
			if result.Effort.Band != test.want ||
				result.Effort.Label == "" {
				t.Fatalf("effort = %+v", result.Effort)
			}
		})
	}
}

func TestAnalyzeIssueEstimatesBroadCrossLayerScope(t *testing.T) {
	result := AnalyzeIssue(AnalysisInput{
		Candidate: analysisCandidate(
			"Introduce cross-service account architecture",
			`
Implement React frontend and backend API changes.
Add regression testing and a large database schema migration.
Update GitHub Actions deployment infrastructure and documentation.
This is a breaking architecture change across multiple services.
`,
			[]string{"enhancement"},
			"Go",
			25,
		),
		Dependencies: []string{"react", "typescript"},
	})

	wantAreas := []ChangeArea{
		ChangeFrontend,
		ChangeBackend,
		ChangeTests,
		ChangeMigration,
		ChangeDocumentation,
		ChangeInfrastructure,
	}
	if !reflect.DeepEqual(result.Scope.Areas, wantAreas) {
		t.Fatalf("areas = %v, want %v", result.Scope.Areas, wantAreas)
	}
	if result.Scope.DatabaseChange != SignalPresent ||
		result.Scope.FileCount.Minimum != 9 ||
		result.Scope.FileCount.Maximum != 0 ||
		result.Difficulty.Level.Int() != 5 {
		t.Fatalf("analysis = %+v", result)
	}
}

func TestAnalyzeIssueHonorsExplicitNoDatabaseChangeAndDifficultyFloor(
	t *testing.T,
) {
	result := AnalyzeIssue(AnalysisInput{
		Candidate: analysisCandidate(
			"Update API validation",
			"Implement one backend validation rule. DB change: none. Add a focused verification step.",
			[]string{"backend", "good first issue", "difficulty: 4"},
			"Go",
			0,
		),
	})
	if result.Scope.DatabaseChange != SignalAbsent ||
		slices.Contains(result.Scope.Areas, ChangeMigration) {
		t.Fatalf("scope = %+v", result.Scope)
	}
	if result.Difficulty.Level.Int() != 4 {
		t.Fatalf("difficulty = %+v", result.Difficulty)
	}
}

func TestAnalyzeIssueIsStableBoundedAndDoesNotMutateInput(t *testing.T) {
	labels := make([]string, MaximumAnalysisLabels+20)
	labels[0] = "backend"
	labels[1] = "enhancement"
	for index := 2; index < len(labels); index++ {
		labels[index] = "label-" + string(rune('a'+index%26))
	}
	dependencies := make([]string, MaximumAnalysisDependencies+20)
	for index := range dependencies {
		dependencies[index] = "dependency-" + string(rune('a'+index%26))
	}
	secret := "ghp_123456789012345678901234567890"
	body := strings.Repeat("Implementation details and test plan. ", 3000) +
		secret
	input := AnalysisInput{
		Candidate: analysisCandidate(
			"Implement bounded analysis",
			body,
			labels,
			"Go",
			2,
		),
		Dependencies: dependencies,
	}
	labelsBefore := slices.Clone(labels)
	dependenciesBefore := slices.Clone(dependencies)

	first := AnalyzeIssue(input)
	second := AnalyzeIssue(input)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("analysis is unstable:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if !reflect.DeepEqual(labels, labelsBefore) ||
		!reflect.DeepEqual(dependencies, dependenciesBefore) {
		t.Fatal("AnalyzeIssue mutated caller-owned slices")
	}
	normalized := normalizeAnalysisInput(input)
	if len(normalized.body) > MaximumAnalysisTextBytes ||
		len(normalized.labels) > MaximumAnalysisLabels ||
		len(normalized.dependencies) > MaximumAnalysisDependencies ||
		!normalized.bodyWasTruncated {
		t.Fatalf("normalized bounds = %+v", normalized)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), secret) ||
		!reflect.DeepEqual(encoded, mustMarshalAnalysis(t, second)) {
		t.Fatal("analysis leaked raw content or was not byte-stable")
	}
}

func TestContainsNormalizedTermUsesWordBoundaries(t *testing.T) {
	tests := []struct {
		text string
		term string
		want bool
	}{
		{text: "ongoing work", term: "go", want: false},
		{text: "use go modules", term: "go", want: true},
		{text: "redis_cache", term: "redis", want: false},
		{text: "redis-cache", term: "redis", want: true},
		{text: "日本語のテスト方法", term: "テスト方法", want: true},
	}
	for _, test := range tests {
		if got := containsNormalizedTerm(test.text, test.term); got != test.want {
			t.Errorf(
				"containsNormalizedTerm(%q, %q) = %t, want %t",
				test.text,
				test.term,
				got,
				test.want,
			)
		}
	}
}

func TestAnalyzeIssueUsesMaintainerGuidanceForConfidence(t *testing.T) {
	input := AnalysisInput{
		Candidate: analysisCandidate(
			"Add backend validation",
			"Implement one backend API validation rule with a focused regression test and clear verification.",
			[]string{"backend"},
			"Go",
			2,
		),
	}
	withoutGuidance := AnalyzeIssue(input)
	input.HasMaintainerGuidance = true
	withGuidance := AnalyzeIssue(input)

	if confidenceRank(withGuidance.Difficulty.Confidence) <
		confidenceRank(withoutGuidance.Difficulty.Confidence) {
		t.Fatalf(
			"guidance reduced confidence: without=%+v with=%+v",
			withoutGuidance.Difficulty,
			withGuidance.Difficulty,
		)
	}
}

func analysisCandidate(
	title string,
	body string,
	labels []string,
	language string,
	comments int,
) Candidate {
	return Candidate{
		Repository: repository.Summary{
			Owner:        "example",
			Name:         "project",
			FullName:     "example/project",
			MainLanguage: language,
		},
		Issue: Summary{
			Number:   42,
			Title:    title,
			Body:     body,
			Labels:   labels,
			Comments: comments,
		},
	}
}

func qualitySignalByKey(
	t *testing.T,
	assessment QualityAssessment,
	key QualitySignalKey,
) QualitySignal {
	t.Helper()
	for _, signal := range assessment.Signals {
		if signal.Key == key {
			return signal
		}
	}
	t.Fatalf("quality signal %q not found", key)
	return QualitySignal{}
}

func hasTechnology(technologies []RequiredTechnology, name string) bool {
	return countTechnology(technologies, name) > 0
}

func countTechnology(technologies []RequiredTechnology, name string) int {
	count := 0
	for _, technology := range technologies {
		if technology.Name == name {
			count++
		}
	}
	return count
}

func technologyByName(
	t *testing.T,
	technologies []RequiredTechnology,
	name string,
) RequiredTechnology {
	t.Helper()
	for _, technology := range technologies {
		if technology.Name == name {
			return technology
		}
	}
	t.Fatalf("technology %q not found", name)
	return RequiredTechnology{}
}

func mustMarshalAnalysis(t *testing.T, analysis Analysis) []byte {
	t.Helper()
	encoded, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
