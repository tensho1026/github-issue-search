package issue

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

const recommendationActivityWindowDays = 180

// Recommend calculates the documented 100-point score and stable reasons and
// warnings. It performs no I/O and does not mutate the supplied slices.
func Recommend(input RecommendationInput) Recommendation {
	now := input.Now.UTC()
	if now.IsZero() {
		now = input.Candidate.Issue.UpdatedAt.UTC()
	}

	skillMatch := assessSkillMatch(
		input.Analysis.RequiredTechnologies,
		input.DesiredSkills,
	)
	signals := normalizeRepositorySignals(input.RepositorySignals)
	breakdown := ScoreBreakdown{
		SkillMatch:        scoreSkillMatch(skillMatch),
		IssueQuality:      scoreIssueQuality(input.Analysis.Quality),
		RepositoryQuality: scoreRepositoryQuality(signals),
		Activity:          scoreActivity(input.Activity, now),
		Maintainer:        scoreMaintainer(input.Activity),
		Availability: scoreAvailability(
			input.Candidate,
			input.Claim,
			now,
		),
	}

	components := scoreComponents(breakdown)
	total := 0
	reasons := make([]string, 0, len(components))
	for _, component := range components {
		total += component.Score
		reasons = append(reasons, component.Reasons...)
	}
	total = min(total, MaximumRecommendationScore)
	slices.Sort(reasons)
	reasons = slices.Compact(reasons)

	return Recommendation{
		Score:             total,
		Breakdown:         breakdown,
		SkillMatch:        skillMatch,
		RepositorySignals: signals,
		Activity:          input.Activity,
		Claim:             cloneClaimEvidence(input.Claim),
		Reasons:           reasons,
		Warnings:          recommendationWarnings(input, signals, now),
	}
}

func assessSkillMatch(
	required []RequiredTechnology,
	desired []string,
) SkillMatchAssessment {
	desiredSet := make(map[string]struct{}, len(desired))
	for _, skill := range desired {
		normalized := normalizeSkill(skill)
		if normalized != "" {
			desiredSet[normalized] = struct{}{}
		}
	}

	requiredByName := make(map[string]RequiredTechnology, len(required))
	for _, technology := range required {
		name := strings.TrimSpace(technology.Name)
		if name == "" {
			continue
		}
		key := normalizeSkill(name)
		if current, exists := requiredByName[key]; exists {
			current.Evidence = append(
				append([]Evidence(nil), current.Evidence...),
				technology.Evidence...,
			)
			if confidenceRank(technology.Confidence) >
				confidenceRank(current.Confidence) {
				current.Confidence = technology.Confidence
			}
			requiredByName[key] = current
			continue
		}
		technology.Evidence = append([]Evidence(nil), technology.Evidence...)
		requiredByName[key] = technology
	}

	keys := make([]string, 0, len(requiredByName))
	for key := range requiredByName {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	matches := make([]SkillMatch, 0, len(keys))
	matched := 0
	denominator := 0
	for _, key := range keys {
		technology := requiredByName[key]
		status := MatchUnknown
		if technology.Confidence != ConfidenceLow && len(desiredSet) > 0 {
			denominator++
			if _, exists := desiredSet[key]; exists {
				status = MatchMatched
				matched++
			} else {
				status = MatchUnmatched
			}
		}
		matches = append(matches, SkillMatch{
			Technology: technology.Name,
			Status:     status,
			Evidence:   append([]Evidence(nil), technology.Evidence...),
		})
	}

	percentage := 0
	if denominator > 0 {
		percentage = roundedPercentage(matched, denominator)
	}
	return SkillMatchAssessment{
		Percentage:  percentage,
		Matched:     matched,
		Denominator: denominator,
		Skills:      matches,
	}
}

func normalizeSkill(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func scoreSkillMatch(match SkillMatchAssessment) ScoreComponent {
	score := scaleScore(match.Percentage, SkillScoreMaximum)
	reasons := []string{}
	if match.Denominator == 0 {
		reasons = append(reasons, "Skill match is unavailable")
	} else {
		reasons = append(
			reasons,
			"Skill match is based on explicit required technologies",
		)
	}
	return ScoreComponent{
		Name:    "skill_match",
		Score:   score,
		Maximum: SkillScoreMaximum,
		Reasons: reasons,
	}
}

func scoreIssueQuality(quality QualityAssessment) ScoreComponent {
	score := scaleScore(clamp(quality.Score, 0, 100), IssueQualityScoreMaximum)
	reasons := []string{"Issue quality uses independently observed description signals"}
	return ScoreComponent{
		Name:    "issue_quality",
		Score:   score,
		Maximum: IssueQualityScoreMaximum,
		Reasons: reasons,
	}
}

func normalizeRepositorySignals(
	input []RepositorySignal,
) []RepositorySignal {
	byKey := make(map[RepositorySignalKey]RepositorySignal, len(input))
	for _, signal := range input {
		if !validRepositorySignalKey(signal.Key) ||
			!validSignalState(signal.State) {
			continue
		}
		signal.Evidence = append([]Evidence(nil), signal.Evidence...)
		byKey[signal.Key] = signal
	}

	keys := []RepositorySignalKey{
		RepositoryREADME,
		RepositoryContributing,
		RepositoryCI,
		RepositoryTests,
		RepositoryCodeOfConduct,
	}
	signals := make([]RepositorySignal, 0, len(keys))
	for _, key := range keys {
		signal, exists := byKey[key]
		if !exists {
			signal = RepositorySignal{Key: key, State: SignalUnknown}
		}
		signals = append(signals, signal)
	}
	return signals
}

func validRepositorySignalKey(key RepositorySignalKey) bool {
	switch key {
	case RepositoryREADME,
		RepositoryContributing,
		RepositoryCI,
		RepositoryTests,
		RepositoryCodeOfConduct:
		return true
	default:
		return false
	}
}

func validSignalState(state SignalState) bool {
	switch state {
	case SignalPresent, SignalAbsent, SignalNotApplicable, SignalUnknown:
		return true
	default:
		return false
	}
}

func scoreRepositoryQuality(signals []RepositorySignal) ScoreComponent {
	weights := map[RepositorySignalKey]int{
		RepositoryREADME:        3,
		RepositoryContributing:  4,
		RepositoryCI:            3,
		RepositoryTests:         3,
		RepositoryCodeOfConduct: 2,
	}
	score := 0
	reasons := make([]string, 0, len(signals))
	for _, signal := range signals {
		if signal.State == SignalPresent {
			score += weights[signal.Key]
			reasons = append(
				reasons,
				"Repository provides "+string(signal.Key),
			)
		}
	}
	return ScoreComponent{
		Name:    "repository_quality",
		Score:   score,
		Maximum: RepositoryScoreMaximum,
		Reasons: reasons,
	}
}

func scoreActivity(
	activity ActivityMetrics,
	now time.Time,
) ScoreComponent {
	score := 0
	reasons := make([]string, 0, 4)
	if !activity.LastMeaningfulUpdate.IsZero() {
		age := now.Sub(activity.LastMeaningfulUpdate.UTC())
		switch {
		case age <= 30*24*time.Hour:
			score += 6
			reasons = append(reasons, "Repository activity was observed within 30 days")
		case age <= 90*24*time.Hour:
			score += 4
			reasons = append(reasons, "Repository activity was observed within 90 days")
		case age <= 180*24*time.Hour:
			score += 2
			reasons = append(reasons, "Repository activity was observed within 180 days")
		}
	}
	if activity.PullRequestMerge.Status == AggregateAvailable {
		switch {
		case activity.PullRequestMerge.Percentage >= 70:
			score += 4
		case activity.PullRequestMerge.Percentage >= 40:
			score += 2
		}
		reasons = append(
			reasons,
			"Pull request cadence uses a bounded 180-day sample",
		)
	}
	if activity.CI == CIStateSuccess {
		score += 3
		reasons = append(reasons, "Default branch checks are successful")
	}
	if activity.Contributors.Status == AggregateAvailable &&
		activity.Contributors.Value >= 2 {
		score += 2
		reasons = append(reasons, "Multiple public contributors were observed")
	}
	return ScoreComponent{
		Name:    "activity",
		Score:   score,
		Maximum: ActivityScoreMaximum,
		Reasons: reasons,
	}
}

func scoreMaintainer(activity ActivityMetrics) ScoreComponent {
	score := durationScore(activity.IssueResponse, 5, 3, 1) +
		durationScore(activity.PullRequestReview, 3, 2, 1) +
		durationScore(activity.PullRequestMergeTime, 2, 1, 1)
	reasons := make([]string, 0, 3)
	if activity.IssueResponse.Status == AggregateAvailable {
		reasons = append(reasons, "Issue response time uses maintainer-only samples")
	}
	if activity.PullRequestReview.Status == AggregateAvailable {
		reasons = append(reasons, "Pull request review time excludes bots and drafts")
	}
	if activity.PullRequestMergeTime.Status == AggregateAvailable {
		reasons = append(reasons, "Pull request merge time uses completed samples")
	}
	return ScoreComponent{
		Name:    "maintainer_responsiveness",
		Score:   score,
		Maximum: MaintainerScoreMaximum,
		Reasons: reasons,
	}
}

func durationScore(
	aggregate DurationAggregate,
	fast int,
	moderate int,
	slow int,
) int {
	if aggregate.Status != AggregateAvailable || aggregate.SampleSize < 1 {
		return 0
	}
	switch {
	case aggregate.Median <= 24*time.Hour:
		return fast
	case aggregate.Median <= 72*time.Hour:
		return moderate
	case aggregate.Median <= 7*24*time.Hour:
		return slow
	default:
		return 0
	}
}

func scoreAvailability(
	candidate Candidate,
	claim ClaimEvidence,
	now time.Time,
) ScoreComponent {
	score := 0
	reasons := make([]string, 0, 3)
	if len(candidate.Issue.Assignees) == 0 {
		score += 4
		reasons = append(reasons, "Issue has no assignee")
	}
	if !claim.Claimed {
		score += 3
		reasons = append(reasons, "No explicit contributor claim was observed")
	}
	if !candidate.Issue.UpdatedAt.IsZero() &&
		now.Sub(candidate.Issue.UpdatedAt.UTC()) <= 90*24*time.Hour {
		score += 3
		reasons = append(reasons, "Issue activity was observed within 90 days")
	}
	return ScoreComponent{
		Name:    "availability",
		Score:   score,
		Maximum: AvailabilityScoreMaximum,
		Reasons: reasons,
	}
}

func recommendationWarnings(
	input RecommendationInput,
	signals []RepositorySignal,
	now time.Time,
) []Warning {
	warnings := make([]Warning, 0, 8)
	activityWindow := time.Duration(recommendationActivityWindowDays) *
		24 * time.Hour
	if input.Claim.Claimed {
		warnings = append(warnings, Warning{
			Code:     "likely_claimed",
			Severity: SeverityWarning,
			Message:  "A contributor explicitly indicated that they are working on this issue",
			Evidence: append([]Evidence(nil), input.Claim.Evidence...),
		})
	}
	if !input.Activity.LastMeaningfulUpdate.IsZero() &&
		now.Sub(input.Activity.LastMeaningfulUpdate.UTC()) > activityWindow {
		warnings = append(warnings, Warning{
			Code:     "stale_repository",
			Severity: SeverityCritical,
			Message:  "No meaningful repository activity was observed within 180 days",
			Evidence: []Evidence{{
				RuleID:      "activity.repository.stale",
				Source:      EvidenceDerived,
				Description: "last meaningful update is older than the activity window",
			}},
		})
	}
	if input.Activity.CI == CIStateFailure {
		warnings = append(warnings, Warning{
			Code:     "failing_ci",
			Severity: SeverityCritical,
			Message:  "The latest default-branch check rollup is failing",
			Evidence: []Evidence{{
				RuleID:      "activity.ci.failure",
				Source:      EvidenceDerived,
				Description: "default branch check rollup reports failure",
			}},
		})
	}
	if input.Activity.IssueResponse.Status == AggregateAvailable &&
		input.Activity.IssueResponse.Median > 14*24*time.Hour {
		warnings = append(warnings, Warning{
			Code:     "slow_issue_response",
			Severity: SeverityWarning,
			Message:  "Maintainer issue responses are slow in the bounded sample",
			Evidence: []Evidence{{
				RuleID:      "maintainer.issue_response.slow",
				Source:      EvidenceDerived,
				Description: "median maintainer response exceeds 14 days",
			}},
		})
	}
	if input.Activity.StaleOpenPullRequests.Status == AggregateAvailable &&
		input.Activity.StaleOpenPullRequests.Value > 0 {
		warnings = append(warnings, Warning{
			Code:     "abandoned_pull_request_risk",
			Severity: SeverityWarning,
			Message:  "Stale open pull requests were observed in the bounded sample",
			Evidence: []Evidence{{
				RuleID:      "maintainer.pull_request.long_lived",
				Source:      EvidenceDerived,
				Description: "an open pull request is older than 60 days and inactive for 30 days",
			}},
		})
	}
	if input.Activity.UnansweredIssues.Status == AggregateAvailable &&
		input.Activity.UnansweredIssues.Value > 0 {
		warnings = append(warnings, Warning{
			Code:     "unanswered_issue_risk",
			Severity: SeverityWarning,
			Message:  "Issues without a maintainer response were observed in the bounded sample",
			Evidence: []Evidence{{
				RuleID:      "maintainer.issue.unanswered",
				Source:      EvidenceDerived,
				Description: "an issue older than 14 days has no observed maintainer response",
			}},
		})
	}

	for _, signal := range signals {
		if signal.State == SignalUnknown {
			warnings = append(warnings, Warning{
				Code:     "repository_signal_unavailable",
				Severity: SeverityInfo,
				Message:  "Repository inspection is incomplete for " + string(signal.Key),
				Evidence: append([]Evidence(nil), signal.Evidence...),
			})
		}
	}
	slices.SortFunc(warnings, func(left, right Warning) int {
		if result := cmp.Compare(severityRank(right.Severity), severityRank(left.Severity)); result != 0 {
			return result
		}
		return cmp.Compare(left.Code, right.Code)
	})
	return warnings
}

func scoreComponents(breakdown ScoreBreakdown) []ScoreComponent {
	return []ScoreComponent{
		breakdown.SkillMatch,
		breakdown.IssueQuality,
		breakdown.RepositoryQuality,
		breakdown.Activity,
		breakdown.Maintainer,
		breakdown.Availability,
	}
}

func roundedPercentage(numerator, denominator int) int {
	if denominator <= 0 {
		return 0
	}
	return clamp((numerator*100+denominator/2)/denominator, 0, 100)
}

func scaleScore(percentage, maximum int) int {
	return (clamp(percentage, 0, 100)*maximum + 50) / 100
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

func cloneClaimEvidence(claim ClaimEvidence) ClaimEvidence {
	claim.Evidence = append([]Evidence(nil), claim.Evidence...)
	return claim
}
