package issue

import (
	"slices"
	"strconv"
	"strings"
)

func estimateAnalysisDifficulty(
	input normalizedAnalysisInput,
	category CategoryAssessment,
	technologies []RequiredTechnology,
	scope ChangeScope,
	quality QualityAssessment,
) DifficultyAssessment {
	level := categoryBaseDifficulty(category.Primary)
	evidence := []Evidence{{
		RuleID:      "difficulty.category-base",
		Source:      EvidenceDerived,
		Description: "The primary category provides the baseline difficulty.",
	}}

	if hasAnyLabel(input, "good first issue", "first timers only", "starter") {
		level--
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.starter-label",
			Source:      EvidenceLabel,
			Description: "A starter label reduces the estimate by one level.",
		})
	}
	explicitDifficulty := 0
	for _, label := range input.labels {
		explicitDifficulty = max(
			explicitDifficulty,
			explicitAnalysisDifficultyLabel(label),
		)
	}
	if explicitDifficulty > 0 {
		level = max(level, explicitDifficulty)
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.explicit-label",
			Source:      EvidenceLabel,
			Description: "A recognized difficulty label sets a minimum level.",
		})
	}
	if scope.DatabaseChange == SignalPresent ||
		slices.Contains(scope.Areas, ChangeMigration) {
		level++
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.database-or-migration",
			Source:      EvidenceBody,
			Description: "Database or migration changes increase complexity.",
		})
	}
	if slices.Contains(scope.Areas, ChangeFrontend) &&
		slices.Contains(scope.Areas, ChangeBackend) {
		level++
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.cross-layer",
			Source:      EvidenceDerived,
			Description: "Frontend and backend changes create a cross-layer task.",
		})
	}
	if len(scope.Areas) >= 4 {
		level++
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.broad-scope",
			Source:      EvidenceDerived,
			Description: "Four or more affected areas increase complexity.",
		})
	}
	if len(technologies) >= 5 {
		level++
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.many-technologies",
			Source:      EvidenceDerived,
			Description: "Five or more inferred technologies increase complexity.",
		})
	}
	if input.commentCount >= 20 {
		level++
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.high-discussion",
			Source:      EvidenceIssueMetadata,
			Description: "A large discussion may indicate hidden complexity.",
		})
	}

	switch {
	case containsAnyTerm(input.combined, "security", "vulnerability", "cve"):
		level = max(level+1, 5)
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.security",
			Source:      EvidenceBody,
			Description: "Security-sensitive work requires the highest review rigor.",
		})
	case containsAnyTerm(
		input.combined,
		"breaking change", "architecture change", "multiple services",
		"cross-service", "large migration",
	):
		level = max(level+1, 5)
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.systemic-risk",
			Source:      EvidenceBody,
			Description: "Breaking, architectural, or cross-service work is systemic.",
		})
	case containsAnyTerm(
		input.combined,
		"authentication", "authorization", "large refactor",
		"multiple packages",
	):
		level++
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.high-risk-change",
			Source:      EvidenceBody,
			Description: "Authentication, schema, or broad refactoring increases risk.",
		})
	}
	level = min(max(level, 1), 5)

	confidence := ConfidenceMedium
	if quality.Score >= 65 && !input.textWasTruncated {
		confidence = ConfidenceHigh
	}
	if quality.Confidence == ConfidenceLow || input.textWasTruncated {
		confidence = ConfidenceLow
	}
	if input.hasMaintainerGuidance &&
		confidence == ConfidenceMedium {
		confidence = ConfidenceHigh
		evidence = append(evidence, Evidence{
			RuleID:      "difficulty.maintainer-guidance",
			Source:      EvidenceIssueMetadata,
			Description: "Maintainer guidance increases estimate confidence.",
		})
	}
	slices.SortFunc(evidence, compareEvidence)
	difficulty, _ := ParseDifficulty(level)
	return DifficultyAssessment{
		Level:      difficulty,
		Label:      difficultyLabel(level),
		Confidence: confidence,
		Evidence:   evidence,
	}
}

func explicitAnalysisDifficultyLabel(label string) int {
	switch label {
	case "very easy":
		return 1
	case "beginner", "starter", "easy":
		return 2
	case "intermediate", "medium":
		return 3
	case "hard", "complex":
		return 4
	case "very hard", "advanced", "expert":
		return 5
	}
	for _, prefix := range []string{"difficulty ", "difficulty:", "difficulty/"} {
		raw, found := strings.CutPrefix(label, prefix)
		if !found {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil && value >= 1 && value <= 5 {
			return value
		}
	}
	return 0
}

func difficultyLabel(level int) string {
	switch level {
	case 1:
		return "Very Easy"
	case 2:
		return "Easy"
	case 3:
		return "Medium"
	case 4:
		return "Hard"
	default:
		return "Very Hard"
	}
}

func estimateEffort(
	category Category,
	difficulty DifficultyAssessment,
	scope ChangeScope,
) EffortEstimate {
	band := EffortOneDay
	switch difficulty.Level.Int() {
	case 1:
		if category == CategoryDocumentation ||
			category == CategoryLocalization {
			band = EffortThirtyMinutes
		} else {
			band = EffortTwoHours
		}
	case 2:
		if category == CategoryDocumentation ||
			category == CategoryTesting ||
			category == CategoryBug {
			band = EffortTwoHours
		} else {
			band = EffortHalfDay
		}
	case 3:
		if category == CategoryDocumentation ||
			category == CategoryTesting {
			band = EffortHalfDay
		} else {
			band = EffortOneDay
		}
	case 4:
		if scope.DatabaseChange == SignalPresent ||
			slices.Contains(scope.Areas, ChangeMigration) ||
			category == CategoryFeature ||
			category == CategoryRefactoring ||
			category == CategoryDevOps {
			band = EffortThreeDays
		} else {
			band = EffortOneDay
		}
	case 5:
		band = EffortThreeDays
	}
	return EffortEstimate{
		Band:       band,
		Label:      effortLabel(band),
		Confidence: difficulty.Confidence,
		Evidence: []Evidence{{
			RuleID:      "effort.category-difficulty-matrix",
			Source:      EvidenceDerived,
			Description: "The effort band is derived from category, difficulty, and migration scope.",
		}},
	}
}

func effortLabel(band EffortBand) string {
	switch band {
	case EffortThirtyMinutes:
		return "30 minutes"
	case EffortTwoHours:
		return "2 hours"
	case EffortHalfDay:
		return "Half day"
	case EffortOneDay:
		return "1 day"
	default:
		return "3 days"
	}
}
