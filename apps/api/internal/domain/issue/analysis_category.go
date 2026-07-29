package issue

import (
	"slices"
)

type categoryRule struct {
	category Category
	labels   []string
	terms    []string
}

var categoryRules = []categoryRule{
	{
		category: CategoryAccessibility,
		labels:   []string{"accessibility", "a11y"},
		terms: []string{
			"accessibility", "a11y", "screen reader", "keyboard navigation",
			"aria", "wcag",
		},
	},
	{
		category: CategoryLocalization,
		labels:   []string{"localization", "translation", "i18n", "l10n"},
		terms: []string{
			"localization", "translation", "translate", "i18n", "l10n",
			"locale",
		},
	},
	{
		category: CategoryPerformance,
		labels:   []string{"performance", "optimization"},
		terms: []string{
			"performance", "optimize", "optimization", "latency",
			"memory allocation", "slow query", "benchmark",
		},
	},
	{
		category: CategoryRefactoring,
		labels:   []string{"refactoring", "refactor", "technical debt"},
		terms: []string{
			"refactor", "technical debt", "cleanup", "restructure",
			"simplify architecture",
		},
	},
	{
		category: CategoryDocumentation,
		labels:   []string{"documentation", "docs"},
		terms: []string{
			"documentation", "readme", "docs", "typo", "godoc",
			"openapi description",
		},
	},
	{
		category: CategoryTesting,
		labels:   []string{"testing", "tests", "test"},
		terms: []string{
			"add tests", "test coverage", "regression test", "unit test",
			"integration test", "end-to-end test",
		},
	},
	{
		category: CategoryBug,
		labels:   []string{"bug", "defect"},
		terms: []string{
			"bug", "incorrect", "broken", "fails", "failure", "regression",
			"unexpected behavior", "crash",
		},
	},
	{
		category: CategoryFeature,
		labels:   []string{"feature", "enhancement"},
		terms: []string{
			"feature", "enhancement", "add support", "implement", "introduce",
			"new capability",
		},
	},
	{
		category: CategoryDevOps,
		labels:   []string{"devops", "ci", "deployment", "infrastructure"},
		terms: []string{
			"ci pipeline", "github actions", "deployment", "infrastructure",
			"terraform", "kubernetes", "dockerfile", "release workflow",
		},
	},
	{
		category: CategoryUI,
		labels:   []string{"ui", "frontend", "design"},
		terms: []string{
			"user interface", "frontend", "component", "css", "responsive",
			"layout", "dialog", "button",
		},
	},
	{
		category: CategoryBackend,
		labels:   []string{"backend", "api", "server"},
		terms: []string{
			"backend", "api endpoint", "http handler", "server", "database",
			"graphql", "grpc",
		},
	},
}

type scoredCategory struct {
	category Category
	score    int
	priority int
	evidence []Evidence
}

func classifyCategory(
	input normalizedAnalysisInput,
) CategoryAssessment {
	scored := make([]scoredCategory, 0, len(categoryRules))
	for priority, rule := range categoryRules {
		result := scoredCategory{
			category: rule.category,
			priority: priority,
			evidence: []Evidence{},
		}
		for _, label := range rule.labels {
			if hasAnyLabel(input, label) {
				result.score += 100
				result.evidence = append(result.evidence, Evidence{
					RuleID:      "category.explicit-label",
					Source:      EvidenceLabel,
					Description: "An issue label explicitly identifies this category.",
				})
				break
			}
		}
		if containsAnyTerm(input.title, rule.terms...) {
			result.score += 40
			result.evidence = append(result.evidence, Evidence{
				RuleID:      "category.title-keyword",
				Source:      EvidenceTitle,
				Description: "The issue title contains a category keyword.",
			})
		}
		if containsAnyTerm(input.body, rule.terms...) {
			result.score += 20
			result.evidence = append(result.evidence, Evidence{
				RuleID:      "category.body-keyword",
				Source:      EvidenceBody,
				Description: "The issue body contains a category keyword.",
			})
		}
		if result.score > 0 {
			slices.SortFunc(result.evidence, compareEvidence)
			scored = append(scored, result)
		}
	}

	if len(scored) == 0 {
		return CategoryAssessment{
			Primary:    CategoryFeature,
			Matches:    []Category{CategoryFeature},
			Confidence: ConfidenceLow,
			Evidence: []Evidence{{
				RuleID:      "category.default",
				Source:      EvidenceDerived,
				Description: "No explicit category evidence was available; feature is the neutral fallback.",
			}},
		}
	}
	slices.SortFunc(scored, func(left scoredCategory, right scoredCategory) int {
		if left.score != right.score {
			return right.score - left.score
		}
		return left.priority - right.priority
	})

	matches := make([]Category, 0, len(scored))
	for _, result := range scored {
		matches = append(matches, result.category)
	}
	confidence := ConfidenceMedium
	if scored[0].score >= 100 {
		confidence = ConfidenceHigh
	}
	return CategoryAssessment{
		Primary:    scored[0].category,
		Matches:    matches,
		Confidence: confidence,
		Evidence:   scored[0].evidence,
	}
}

func categoryBaseDifficulty(category Category) int {
	switch category {
	case CategoryDocumentation, CategoryLocalization:
		return 1
	case CategoryBug, CategoryTesting, CategoryAccessibility, CategoryUI:
		return 2
	case CategoryFeature, CategoryBackend, CategoryDevOps,
		CategoryPerformance, CategoryRefactoring:
		return 3
	default:
		return 3
	}
}

func containsCategory(categories []Category, expected Category) bool {
	return slices.Contains(categories, expected)
}
