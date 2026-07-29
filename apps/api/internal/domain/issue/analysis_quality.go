package issue

import (
	"regexp"
	"strings"
)

var relatedFilePattern = regexp.MustCompile(
	`(?i)(?:^|[\s("'])` +
		`(?:[\w.-]+/)+[\w.-]+\.(?:go|tsx?|jsx?|vue|svelte|py|java|rs|php|cs|sql|ya?ml|json|md)` +
		`(?:$|[\s:),."'` + "`" + `])`,
)

var qualitySignalOrder = []QualitySignalKey{
	QualityProblemDescription,
	QualityExpectedBehavior,
	QualityCurrentBehavior,
	QualityReproductionSteps,
	QualityImplementationGuidance,
	QualityRelatedFiles,
	QualityScreenshot,
	QualityTestMethod,
	QualityAcceptanceCriteria,
}

type qualityRule struct {
	key         QualitySignalKey
	ruleID      string
	terms       []string
	customMatch func(normalizedAnalysisInput) bool
}

var qualityRules = []qualityRule{
	{
		key:    QualityProblemDescription,
		ruleID: "quality.problem-description",
		terms: []string{
			"problem", "description", "summary", "overview",
			"背景", "問題", "概要",
		},
		customMatch: func(input normalizedAnalysisInput) bool {
			return input.bodyRuneCount >= 120
		},
	},
	{
		key:    QualityExpectedBehavior,
		ruleID: "quality.expected-behavior",
		terms: []string{
			"expected behavior", "expected result", "expected outcome",
			"should behave", "期待する動作", "期待結果",
		},
	},
	{
		key:    QualityCurrentBehavior,
		ruleID: "quality.current-behavior",
		terms: []string{
			"actual behavior", "current behavior", "observed behavior",
			"currently", "現状", "現在の動作", "実際の動作",
		},
	},
	{
		key:    QualityReproductionSteps,
		ruleID: "quality.reproduction-steps",
		terms: []string{
			"steps to reproduce", "reproduction", "minimal reproduction",
			"how to reproduce", "再現手順", "再現方法",
		},
	},
	{
		key:    QualityImplementationGuidance,
		ruleID: "quality.implementation-guidance",
		terms: []string{
			"implementation plan", "proposed solution", "suggested fix",
			"implementation notes", "approach", "実装方針", "対応方針",
		},
	},
	{
		key:    QualityRelatedFiles,
		ruleID: "quality.related-files",
		terms: []string{
			"related files", "affected files", "relevant files",
			"関連ファイル", "対象ファイル",
		},
		customMatch: func(input normalizedAnalysisInput) bool {
			return relatedFilePattern.MatchString(input.body)
		},
	},
	{
		key:    QualityScreenshot,
		ruleID: "quality.screenshot",
		terms: []string{
			"screenshot", "screen shot", "attached image", "image below",
			"visual evidence", "screen recording", "attached video",
			"スクリーンショット", "画像", "動画",
		},
		customMatch: func(input normalizedAnalysisInput) bool {
			return strings.Contains(input.body, "![") ||
				strings.Contains(input.body, "/assets/")
		},
	},
	{
		key:    QualityTestMethod,
		ruleID: "quality.test-method",
		terms: []string{
			"test plan", "how to test", "testing steps", "verification",
			"steps to verify", "test case", "テスト方法", "確認方法",
		},
	},
	{
		key:    QualityAcceptanceCriteria,
		ruleID: "quality.acceptance-criteria",
		terms: []string{
			"acceptance criteria", "definition of done", "done when",
			"completion criteria", "完了条件", "受け入れ条件",
		},
		customMatch: func(input normalizedAnalysisInput) bool {
			return strings.Contains(input.body, "- [ ]")
		},
	},
}

func assessQuality(
	input normalizedAnalysisInput,
	category CategoryAssessment,
) QualityAssessment {
	if input.bodyRuneCount < 20 {
		signals := make([]QualitySignal, 0, len(qualitySignalOrder))
		for _, key := range qualitySignalOrder {
			signals = append(signals, QualitySignal{
				Key:      key,
				State:    SignalUnknown,
				Evidence: []Evidence{},
			})
		}
		return QualityAssessment{
			Score:      0,
			Confidence: ConfidenceLow,
			Signals:    signals,
		}
	}

	applicable := make(map[QualitySignalKey]bool, len(qualitySignalOrder))
	for _, key := range qualitySignalOrder {
		applicable[key] = qualitySignalApplicable(key, category.Matches)
	}
	signals := make([]QualitySignal, 0, len(qualityRules))
	presentCount := 0
	applicableCount := 0
	for _, rule := range qualityRules {
		if !applicable[rule.key] {
			signals = append(signals, QualitySignal{
				Key:      rule.key,
				State:    SignalNotApplicable,
				Evidence: []Evidence{},
			})
			continue
		}
		applicableCount++
		matched := containsAnyTerm(input.body, rule.terms...)
		if !matched && rule.customMatch != nil {
			matched = rule.customMatch(input)
		}
		state := SignalAbsent
		evidence := []Evidence{}
		if matched {
			state = SignalPresent
			presentCount++
			evidence = append(evidence, Evidence{
				RuleID:      rule.ruleID,
				Source:      EvidenceBody,
				Description: "The issue body contains this quality signal.",
			})
		}
		signals = append(signals, QualitySignal{
			Key:      rule.key,
			State:    state,
			Evidence: evidence,
		})
	}

	score := 0
	if applicableCount > 0 {
		score = (presentCount*100 + applicableCount/2) / applicableCount
	}
	confidence := ConfidenceMedium
	if input.bodyRuneCount >= 300 && !input.textWasTruncated {
		confidence = ConfidenceHigh
	}
	if input.textWasTruncated {
		confidence = ConfidenceLow
	}
	return QualityAssessment{
		Score:      score,
		Confidence: confidence,
		Signals:    signals,
	}
}

func qualitySignalApplicable(
	key QualitySignalKey,
	categories []Category,
) bool {
	switch key {
	case QualityExpectedBehavior,
		QualityCurrentBehavior,
		QualityReproductionSteps:
		return containsCategory(categories, CategoryBug)
	case QualityScreenshot:
		return containsCategory(categories, CategoryBug) ||
			containsCategory(categories, CategoryUI) ||
			containsCategory(categories, CategoryAccessibility)
	default:
		return true
	}
}
