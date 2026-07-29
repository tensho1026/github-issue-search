package issue

import (
	"slices"
)

var changeAreaOrder = []ChangeArea{
	ChangeFrontend,
	ChangeBackend,
	ChangeTests,
	ChangeMigration,
	ChangeDocumentation,
	ChangeInfrastructure,
}

func estimateChangeScope(
	input normalizedAnalysisInput,
	category CategoryAssessment,
) ChangeScope {
	areaSet := make(map[ChangeArea]bool, len(changeAreaOrder))
	evidence := make([]Evidence, 0, len(changeAreaOrder)+2)
	addArea := func(area ChangeArea, ruleID string, description string) {
		if areaSet[area] {
			return
		}
		areaSet[area] = true
		evidence = append(evidence, Evidence{
			RuleID:      ruleID,
			Source:      EvidenceDerived,
			Description: description,
		})
	}

	if containsCategory(category.Matches, CategoryUI) ||
		containsCategory(category.Matches, CategoryAccessibility) ||
		containsAnyTerm(
			input.combined,
			"frontend", "react", "vue", "angular", "component", "css",
		) {
		addArea(
			ChangeFrontend,
			"scope.frontend",
			"UI, accessibility, or frontend evidence indicates frontend changes.",
		)
	}
	if containsCategory(category.Matches, CategoryBackend) ||
		containsAnyTerm(
			input.combined,
			"backend", "api endpoint", "server", "handler", "graphql", "grpc",
		) {
		addArea(
			ChangeBackend,
			"scope.backend",
			"API or server evidence indicates backend changes.",
		)
	}
	if containsCategory(category.Matches, CategoryTesting) ||
		containsAnyTerm(
			input.combined,
			"test", "testing", "regression", "verification",
		) {
		addArea(
			ChangeTests,
			"scope.tests",
			"Testing evidence indicates test changes.",
		)
	}
	explicitNoDatabaseChange := containsAnyTerm(
		input.combined,
		"no database change", "no database changes", "database change none",
		"database change: none", "db change none", "db change: none",
		"without database changes", "no migration", "db変更なし",
		"database変更なし",
	)
	hasMigration := !explicitNoDatabaseChange && containsAnyTerm(
		input.combined,
		"migration", "schema change", "database schema", "ddl",
	)
	if hasMigration {
		addArea(
			ChangeMigration,
			"scope.migration",
			"Migration or schema evidence indicates migration changes.",
		)
	}
	if containsCategory(category.Matches, CategoryDocumentation) ||
		containsAnyTerm(input.combined, "documentation", "readme", "docs") {
		addArea(
			ChangeDocumentation,
			"scope.documentation",
			"Documentation evidence indicates documentation changes.",
		)
	}
	if containsCategory(category.Matches, CategoryDevOps) ||
		containsAnyTerm(
			input.combined,
			"github actions", "ci pipeline", "deployment", "terraform",
			"kubernetes", "dockerfile", "infrastructure",
		) {
		addArea(
			ChangeInfrastructure,
			"scope.infrastructure",
			"Delivery or infrastructure evidence indicates infrastructure changes.",
		)
	}
	if len(areaSet) == 0 {
		switch category.Primary {
		case CategoryDocumentation, CategoryLocalization:
			addArea(
				ChangeDocumentation,
				"scope.category-fallback",
				"The primary category indicates documentation changes.",
			)
		case CategoryUI, CategoryAccessibility:
			addArea(
				ChangeFrontend,
				"scope.category-fallback",
				"The primary category indicates frontend changes.",
			)
		case CategoryDevOps:
			addArea(
				ChangeInfrastructure,
				"scope.category-fallback",
				"The primary category indicates infrastructure changes.",
			)
		default:
			addArea(
				ChangeBackend,
				"scope.category-fallback",
				"The primary category indicates application-code changes.",
			)
		}
	}

	areas := make([]ChangeArea, 0, len(areaSet))
	for _, area := range changeAreaOrder {
		if areaSet[area] {
			areas = append(areas, area)
		}
	}

	databaseChange := SignalAbsent
	if explicitNoDatabaseChange {
		evidence = append(evidence, Evidence{
			RuleID:      "scope.explicit-no-database-change",
			Source:      EvidenceBody,
			Description: "The issue explicitly states that no database change is required.",
		})
	} else if hasMigration || containsAnyTerm(
		input.combined,
		"database", "postgresql", "mysql", "mongodb", "sql table",
	) {
		databaseChange = SignalPresent
		evidence = append(evidence, Evidence{
			RuleID:      "scope.database-change",
			Source:      EvidenceBody,
			Description: "Database or schema evidence indicates a database change.",
		})
	} else if input.bodyRuneCount < 20 || input.textWasTruncated {
		databaseChange = SignalUnknown
	}

	fileCount := estimateFileCount(input, category, areas)
	evidence = append(evidence, Evidence{
		RuleID:      "scope.file-count-band",
		Source:      EvidenceDerived,
		Description: "The file-count band is inferred from category, risk, and affected areas.",
	})
	slices.SortFunc(evidence, compareEvidence)
	confidence := ConfidenceMedium
	if input.bodyRuneCount < 80 || input.textWasTruncated {
		confidence = ConfidenceLow
	}
	if relatedFilePattern.MatchString(input.body) &&
		!input.textWasTruncated {
		confidence = ConfidenceHigh
	}
	return ChangeScope{
		Areas:          areas,
		FileCount:      fileCount,
		DatabaseChange: databaseChange,
		Confidence:     confidence,
		Evidence:       evidence,
	}
}

func estimateFileCount(
	input normalizedAnalysisInput,
	category CategoryAssessment,
	areas []ChangeArea,
) FileCountBand {
	complexity := categoryBaseDifficulty(category.Primary)
	if len(areas) >= 2 {
		complexity++
	}
	if len(areas) >= 4 ||
		containsAnyTerm(
			input.combined,
			"architecture", "breaking change", "multiple packages",
			"multiple services", "large refactor", "large migration",
		) {
		complexity = 5
	}
	switch {
	case complexity <= 1:
		return FileCountBand{Minimum: 1, Maximum: 1, Label: "1 file"}
	case complexity == 2:
		return FileCountBand{Minimum: 1, Maximum: 3, Label: "1–3 files"}
	case complexity == 3:
		return FileCountBand{Minimum: 2, Maximum: 5, Label: "2–5 files"}
	case complexity == 4:
		return FileCountBand{Minimum: 4, Maximum: 8, Label: "4–8 files"}
	default:
		return FileCountBand{Minimum: 9, Maximum: 0, Label: "9+ files"}
	}
}
