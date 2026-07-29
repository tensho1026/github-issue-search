package issue

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	MaximumMetricSamples  = 100
	MaximumClaimComments  = 100
	maxMetricDurationDays = 180
)

var explicitClaimPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:i am|i'm|i’ll|i'll|i will)\s+(?:start(?:ing)?\s+|currently\s+)?work(?:ing)?\s+on\s+(?:this|it)\b`),
	regexp.MustCompile(`(?i)\b(?:assigned to me|please assign (?:this|it) to me)\b`),
	regexp.MustCompile(`(?i)(?:^|\s)/assign(?:\s+@?[a-z0-9-]+)?(?:\s|$)`),
}

// DetectClaim returns true only for explicit work or assignment statements.
// Questions such as "Can I work on this?" intentionally remain unclaimed.
func DetectClaim(
	comments []CommentObservation,
	truncated bool,
) ClaimEvidence {
	limit := min(len(comments), MaximumClaimComments)
	for _, comment := range comments[:limit] {
		if IsBotIdentity(comment.AuthorLogin, comment.AuthorType) {
			continue
		}
		body, _ := normalizeAnalysisText(
			comment.Body,
			MaximumAnalysisTextBytes,
		)
		for index, pattern := range explicitClaimPatterns {
			if !pattern.MatchString(body) {
				continue
			}
			return ClaimEvidence{
				Claimed:    true,
				Confidence: ConfidenceHigh,
				Evidence: []Evidence{{
					RuleID:      "availability.explicit_claim." + string(rune('1'+index)),
					Source:      EvidenceIssueMetadata,
					Description: "a human comment contains an explicit work or assignment statement",
				}},
			}
		}
	}
	return ClaimEvidence{
		Claimed:    false,
		Confidence: claimAbsenceConfidence(len(comments), truncated),
		Evidence: []Evidence{{
			RuleID:      "availability.explicit_claim.none",
			Source:      EvidenceIssueMetadata,
			Description: "no explicit claim was found in the inspected comment window",
		}},
	}
}

// IsBotIdentity normalizes GitHub actor metadata for sampled maintenance
// calculations.
func IsBotIdentity(login, actorType string) bool {
	normalizedLogin := strings.ToLower(strings.TrimSpace(login))
	normalizedType := strings.ToLower(strings.TrimSpace(actorType))
	return normalizedType == "bot" ||
		strings.HasSuffix(normalizedLogin, "[bot]") ||
		normalizedLogin == "dependabot" ||
		normalizedLogin == "renovate" ||
		strings.HasPrefix(normalizedLogin, "dependabot-") ||
		strings.HasPrefix(normalizedLogin, "renovate-")
}

func claimAbsenceConfidence(total int, truncated bool) Confidence {
	switch {
	case truncated || total > MaximumClaimComments:
		return ConfidenceLow
	case total >= 5:
		return ConfidenceHigh
	default:
		return ConfidenceMedium
	}
}

// SummarizeDurations returns robust statistics over valid, bounded completed
// samples. Non-positive durations and durations above 180 days are treated as
// outliers rather than responsiveness evidence.
func SummarizeDurations(
	samples []time.Duration,
	windowDays int,
	truncated bool,
) DurationAggregate {
	bounded := samples
	if len(bounded) > MaximumMetricSamples {
		bounded = bounded[:MaximumMetricSamples]
		truncated = true
	}
	valid := make([]time.Duration, 0, len(bounded))
	maximum := time.Duration(maxMetricDurationDays) * 24 * time.Hour
	for _, sample := range bounded {
		if sample <= 0 || sample > maximum {
			continue
		}
		valid = append(valid, sample)
	}
	if len(valid) == 0 {
		return DurationAggregate{
			Status:     AggregateUnavailable,
			WindowDays: windowDays,
			Truncated:  truncated,
			Confidence: ConfidenceLow,
		}
	}
	slices.Sort(valid)
	return DurationAggregate{
		Status:       AggregateAvailable,
		Median:       percentile(valid, 50),
		Percentile90: percentile(valid, 90),
		SampleSize:   len(valid),
		WindowDays:   windowDays,
		Truncated:    truncated,
		Confidence:   sampleConfidence(len(valid), truncated),
	}
}

// SummarizeRatio records an explicit denominator and never treats an empty
// sample as zero percent.
func SummarizeRatio(
	numerator int,
	denominator int,
	windowDays int,
	truncated bool,
) RatioAggregate {
	if denominator <= 0 || numerator < 0 || numerator > denominator {
		return RatioAggregate{
			Status:     AggregateUnavailable,
			WindowDays: windowDays,
			Truncated:  truncated,
			Confidence: ConfidenceLow,
		}
	}
	return RatioAggregate{
		Status:      AggregateAvailable,
		Numerator:   numerator,
		Denominator: denominator,
		Percentage:  roundedPercentage(numerator, denominator),
		SampleSize:  denominator,
		WindowDays:  windowDays,
		Truncated:   truncated,
		Confidence:  sampleConfidence(denominator, truncated),
	}
}

// SummarizeCount validates a bounded observed count and attaches the sample
// metadata needed to interpret it.
func SummarizeCount(
	value int,
	sampleSize int,
	windowDays int,
	truncated bool,
) CountAggregate {
	if value < 0 || sampleSize < 0 || value > sampleSize {
		return CountAggregate{
			Status:     AggregateUnavailable,
			WindowDays: windowDays,
			Truncated:  truncated,
			Confidence: ConfidenceLow,
		}
	}
	return CountAggregate{
		Status:     AggregateAvailable,
		Value:      value,
		SampleSize: sampleSize,
		WindowDays: windowDays,
		Truncated:  truncated,
		Confidence: sampleConfidence(sampleSize, truncated),
	}
}

func percentile(sorted []time.Duration, percentage int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := (percentage*len(sorted) + 99) / 100
	return sorted[clamp(index-1, 0, len(sorted)-1)]
}

func sampleConfidence(sampleSize int, truncated bool) Confidence {
	switch {
	case sampleSize >= 10 && !truncated:
		return ConfidenceHigh
	case sampleSize >= 3:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

// RankIssues returns a sorted copy using stable public tie-breakers.
func RankIssues(input []RankedIssue) []RankedIssue {
	ranked := append([]RankedIssue(nil), input...)
	slices.SortStableFunc(ranked, func(left, right RankedIssue) int {
		if result := cmp.Compare(
			right.Recommendation.Score,
			left.Recommendation.Score,
		); result != 0 {
			return result
		}
		if result := cmp.Compare(
			right.Recommendation.SkillMatch.Percentage,
			left.Recommendation.SkillMatch.Percentage,
		); result != 0 {
			return result
		}
		if result := cmp.Compare(
			right.Candidate.Repository.Stars,
			left.Candidate.Repository.Stars,
		); result != 0 {
			return result
		}
		if result := right.Candidate.Issue.UpdatedAt.Compare(
			left.Candidate.Issue.UpdatedAt,
		); result != 0 {
			return result
		}
		if result := cmp.Compare(
			strings.ToLower(left.Candidate.Repository.FullName),
			strings.ToLower(right.Candidate.Repository.FullName),
		); result != 0 {
			return result
		}
		return cmp.Compare(
			left.Candidate.Issue.Number,
			right.Candidate.Issue.Number,
		)
	})
	return ranked
}
