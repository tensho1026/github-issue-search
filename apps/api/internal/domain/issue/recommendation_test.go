package issue

import (
	"reflect"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

func TestRecommendEnforcesDocumentedScoreInvariant(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	input := completeRecommendationInput(now)

	got := Recommend(input)

	if got.Score != MaximumRecommendationScore {
		t.Fatalf("Recommend() score = %d, want %d", got.Score, MaximumRecommendationScore)
	}
	components := scoreComponents(got.Breakdown)
	total := 0
	maximum := 0
	for _, component := range components {
		if component.Score < 0 || component.Score > component.Maximum {
			t.Fatalf("component %q = %d/%d", component.Name, component.Score, component.Maximum)
		}
		total += component.Score
		maximum += component.Maximum
	}
	if total != got.Score || maximum != MaximumRecommendationScore {
		t.Fatalf("component totals = %d/%d, score = %d", total, maximum, got.Score)
	}
}

func TestRecommendUsesExplicitSkillDenominator(t *testing.T) {
	t.Parallel()
	input := completeRecommendationInput(time.Now())
	input.DesiredSkills = []string{" go ", "React", "GO"}
	input.Analysis.RequiredTechnologies = []RequiredTechnology{
		{Name: "Go", Confidence: ConfidenceHigh},
		{Name: "React", Confidence: ConfidenceMedium},
		{Name: "PostgreSQL", Confidence: ConfidenceHigh},
		{Name: "GraphQL", Confidence: ConfidenceLow},
	}

	got := Recommend(input).SkillMatch

	if got.Percentage != 67 || got.Matched != 2 || got.Denominator != 3 {
		t.Fatalf("skill match = %+v", got)
	}
	statuses := map[string]MatchStatus{}
	for _, skill := range got.Skills {
		statuses[skill.Technology] = skill.Status
	}
	want := map[string]MatchStatus{
		"Go":         MatchMatched,
		"React":      MatchMatched,
		"PostgreSQL": MatchUnmatched,
		"GraphQL":    MatchUnknown,
	}
	if !reflect.DeepEqual(statuses, want) {
		t.Fatalf("skill statuses = %#v, want %#v", statuses, want)
	}
}

func TestRecommendDoesNotTreatUnknownRepositorySignalsAsAbsent(t *testing.T) {
	t.Parallel()
	input := completeRecommendationInput(time.Now())
	input.RepositorySignals = []RepositorySignal{{
		Key:   RepositoryREADME,
		State: SignalPresent,
	}}

	got := Recommend(input)

	if got.Breakdown.RepositoryQuality.Score != 3 {
		t.Fatalf("repository score = %d", got.Breakdown.RepositoryQuality.Score)
	}
	unknown := 0
	for _, signal := range got.RepositorySignals {
		if signal.State == SignalUnknown {
			unknown++
		}
	}
	if unknown != 4 {
		t.Fatalf("unknown signals = %d, want 4", unknown)
	}
	if len(got.Warnings) != 4 {
		t.Fatalf("warnings = %+v", got.Warnings)
	}
}

func TestRecommendationWarningsAreConservativeAndDeterministic(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	input := completeRecommendationInput(now)
	input.Claim = ClaimEvidence{
		Claimed:    true,
		Confidence: ConfidenceHigh,
	}
	input.Activity.LastMeaningfulUpdate = now.AddDate(0, 0, -181)
	input.Activity.CI = CIStateFailure
	input.Activity.IssueResponse = SummarizeDurations(
		[]time.Duration{15 * 24 * time.Hour},
		180,
		false,
	)
	input.Activity.PullRequestMergeTime = SummarizeDurations(
		[]time.Duration{70 * 24 * time.Hour},
		180,
		false,
	)
	input.Activity.StaleOpenPullRequests = SummarizeCount(1, 3, 180, false)
	input.Activity.UnansweredIssues = SummarizeCount(1, 3, 180, false)

	got := Recommend(input)
	codes := make([]string, 0, len(got.Warnings))
	for _, warning := range got.Warnings {
		codes = append(codes, warning.Code)
	}
	want := []string{
		"failing_ci",
		"stale_repository",
		"abandoned_pull_request_risk",
		"likely_claimed",
		"slow_issue_response",
		"unanswered_issue_risk",
	}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("warning codes = %#v, want %#v", codes, want)
	}
}

func TestDetectClaimRejectsAmbiguousAndBotComments(t *testing.T) {
	t.Parallel()
	comments := []CommentObservation{
		{AuthorLogin: "helper", AuthorType: "User", Body: "Can I work on this?"},
		{AuthorLogin: "bot[bot]", AuthorType: "Bot", Body: "I am working on this"},
		{AuthorLogin: "reader", AuthorType: "User", Body: "This looks useful"},
	}
	if got := DetectClaim(comments, false); got.Claimed {
		t.Fatalf("DetectClaim() = %+v, want unclaimed", got)
	}

	comments = append(comments, CommentObservation{
		AuthorLogin: "contributor",
		AuthorType:  "User",
		Body:        "I'll start working on this tomorrow.",
	})
	if got := DetectClaim(comments, false); !got.Claimed ||
		got.Confidence != ConfidenceHigh {
		t.Fatalf("DetectClaim() = %+v, want explicit claim", got)
	}
}

func TestDetectClaimMarksTruncatedAbsenceLowConfidence(t *testing.T) {
	t.Parallel()
	got := DetectClaim([]CommentObservation{{
		AuthorLogin: "reader",
		AuthorType:  "User",
		Body:        "Can I work on this?",
	}}, true)
	if got.Claimed || got.Confidence != ConfidenceLow {
		t.Fatalf("DetectClaim() = %+v", got)
	}
}

func TestSummarizeDurationsBoundsSamplesAndOutliers(t *testing.T) {
	t.Parallel()
	samples := []time.Duration{
		-1,
		0,
		time.Hour,
		2 * time.Hour,
		200 * 24 * time.Hour,
	}
	got := SummarizeDurations(samples, 180, false)
	if got.Status != AggregateAvailable ||
		got.SampleSize != 2 ||
		got.Median != time.Hour ||
		got.Percentile90 != 2*time.Hour {
		t.Fatalf("SummarizeDurations() = %+v", got)
	}

	tooMany := make([]time.Duration, MaximumMetricSamples+1)
	for index := range tooMany {
		tooMany[index] = time.Duration(index+1) * time.Minute
	}
	if bounded := SummarizeDurations(tooMany, 180, false); !bounded.Truncated ||
		bounded.SampleSize != MaximumMetricSamples {
		t.Fatalf("bounded aggregate = %+v", bounded)
	}
}

func TestSummarizeRatioDistinguishesEmptySample(t *testing.T) {
	t.Parallel()
	if got := SummarizeRatio(0, 0, 180, false); got.Status != AggregateUnavailable {
		t.Fatalf("empty ratio = %+v", got)
	}
	if got := SummarizeRatio(2, 3, 180, false); got.Status != AggregateAvailable ||
		got.Percentage != 67 ||
		got.Denominator != 3 {
		t.Fatalf("ratio = %+v", got)
	}
}

func TestSummarizeCountRejectsImpossibleValues(t *testing.T) {
	t.Parallel()
	if got := SummarizeCount(4, 3, 180, false); got.Status != AggregateUnavailable {
		t.Fatalf("impossible count = %+v", got)
	}
	if got := SummarizeCount(3, 5, 180, true); got.Status != AggregateAvailable ||
		got.Value != 3 ||
		got.SampleSize != 5 ||
		!got.Truncated {
		t.Fatalf("count = %+v", got)
	}
}

func TestRankIssuesUsesStableTieBreakersWithoutMutatingInput(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	input := []RankedIssue{
		rankedFixture("z/repo", 10, 50, 80, now, 2),
		rankedFixture("a/repo", 20, 50, 80, now, 3),
		rankedFixture("b/repo", 20, 60, 90, now, 4),
		rankedFixture("c/repo", 20, 60, 90, now.Add(time.Hour), 1),
		rankedFixture("d/repo", 20, 50, 90, now, 5),
	}
	originalFirst := input[0].Candidate.Repository.FullName

	ranked := RankIssues(input)

	got := make([]string, 0, len(ranked))
	for _, item := range ranked {
		got = append(got, item.Candidate.Repository.FullName)
	}
	want := []string{"c/repo", "b/repo", "d/repo", "a/repo", "z/repo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rank order = %#v, want %#v", got, want)
	}
	if input[0].Candidate.Repository.FullName != originalFirst {
		t.Fatal("RankIssues() mutated input")
	}
}

func TestRecommendDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	input := completeRecommendationInput(time.Now())
	before := append([]string(nil), input.DesiredSkills...)
	evidence := append(
		[]Evidence(nil),
		input.Analysis.RequiredTechnologies[0].Evidence...,
	)

	_ = Recommend(input)

	if !reflect.DeepEqual(input.DesiredSkills, before) ||
		!reflect.DeepEqual(
			input.Analysis.RequiredTechnologies[0].Evidence,
			evidence,
		) {
		t.Fatal("Recommend() mutated input")
	}
}

func completeRecommendationInput(now time.Time) RecommendationInput {
	return RecommendationInput{
		Candidate: Candidate{
			Repository: repository.Summary{
				FullName:  "owner/repo",
				Stars:     100,
				UpdatedAt: now.Add(-time.Hour),
			},
			Issue: Summary{
				Number:    1,
				UpdatedAt: now.Add(-time.Hour),
			},
		},
		Analysis: Analysis{
			Quality: QualityAssessment{Score: 100},
			RequiredTechnologies: []RequiredTechnology{{
				Name:       "Go",
				Confidence: ConfidenceHigh,
				Evidence: []Evidence{{
					RuleID: "technology.go",
					Source: EvidenceRepositoryLanguage,
				}},
			}},
		},
		DesiredSkills: []string{"Go"},
		RepositorySignals: []RepositorySignal{
			{Key: RepositoryREADME, State: SignalPresent},
			{Key: RepositoryContributing, State: SignalPresent},
			{Key: RepositoryCI, State: SignalPresent},
			{Key: RepositoryTests, State: SignalPresent},
			{Key: RepositoryCodeOfConduct, State: SignalPresent},
		},
		Activity: ActivityMetrics{
			LastMeaningfulUpdate: now.Add(-time.Hour),
			CI:                   CIStateSuccess,
			Contributors: CountAggregate{
				Status:     AggregateAvailable,
				Value:      5,
				SampleSize: 5,
				WindowDays: 180,
				Confidence: ConfidenceMedium,
			},
			PullRequestsOpened: CountAggregate{
				Status:     AggregateAvailable,
				Value:      10,
				SampleSize: 10,
				WindowDays: 180,
				Confidence: ConfidenceHigh,
			},
			StaleOpenPullRequests: CountAggregate{
				Status:     AggregateAvailable,
				Value:      0,
				SampleSize: 10,
				WindowDays: 180,
				Confidence: ConfidenceHigh,
			},
			UnansweredIssues: CountAggregate{
				Status:     AggregateAvailable,
				Value:      0,
				SampleSize: 10,
				WindowDays: 180,
				Confidence: ConfidenceHigh,
			},
			PullRequestMerge: SummarizeRatio(8, 10, 180, false),
			IssueResponse: SummarizeDurations(
				[]time.Duration{time.Hour},
				180,
				false,
			),
			PullRequestReview: SummarizeDurations(
				[]time.Duration{time.Hour},
				180,
				false,
			),
			PullRequestMergeTime: SummarizeDurations(
				[]time.Duration{time.Hour},
				180,
				false,
			),
		},
		Claim: ClaimEvidence{
			Claimed:    false,
			Confidence: ConfidenceHigh,
		},
		Now: now,
	}
}

func rankedFixture(
	fullName string,
	stars int,
	score int,
	skill int,
	updatedAt time.Time,
	number int,
) RankedIssue {
	return RankedIssue{
		Candidate: Candidate{
			Repository: repository.Summary{
				FullName: fullName,
				Stars:    stars,
			},
			Issue: Summary{
				Number:    number,
				UpdatedAt: updatedAt,
			},
		},
		Recommendation: Recommendation{
			Score: score,
			SkillMatch: SkillMatchAssessment{
				Percentage: skill,
			},
		},
	}
}
