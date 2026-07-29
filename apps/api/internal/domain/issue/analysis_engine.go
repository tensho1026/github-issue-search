package issue

// AnalyzeIssue applies the bounded deterministic rule engine. It performs no
// I/O, does not mutate input slices, and returns stable slice ordering.
func AnalyzeIssue(input AnalysisInput) Analysis {
	normalized := normalizeAnalysisInput(input)
	category := classifyCategory(normalized)
	quality := assessQuality(normalized, category)
	technologies := inferRequiredTechnologies(normalized)
	scope := estimateChangeScope(normalized, category)
	difficulty := estimateAnalysisDifficulty(
		normalized,
		category,
		technologies,
		scope,
		quality,
	)
	effort := estimateEffort(category.Primary, difficulty, scope)

	return Analysis{
		Quality:              quality,
		RequiredTechnologies: technologies,
		Category:             category,
		Difficulty:           difficulty,
		Scope:                scope,
		Effort:               effort,
		Confidence: overallAnalysisConfidence(
			quality.Confidence,
			category.Confidence,
			difficulty.Confidence,
			scope.Confidence,
		),
	}
}

func overallAnalysisConfidence(values ...Confidence) Confidence {
	result := ConfidenceHigh
	for _, value := range values {
		if confidenceRank(value) < confidenceRank(result) {
			result = value
		}
	}
	return result
}
