package issue

import "fmt"

// Confidence values order rule certainty from weak to strong.
const (
	// MaximumAnalysisTextBytes bounds the title and body inspected by the rule
	// engine. GitHub content beyond this limit cannot increase complexity
	// without bound.
	MaximumAnalysisTextBytes = 64 << 10
	// MaximumAnalysisLabels bounds label-derived rule evidence.
	MaximumAnalysisLabels = 100
	// MaximumAnalysisDependencies bounds manifest-derived dependency evidence.
	MaximumAnalysisDependencies = 100
)

// Confidence communicates how strongly the available evidence supports a
// rule-derived result.
type Confidence string

// SignalState values distinguish observed states from unavailable evidence.
const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// SignalState distinguishes observed, missing, irrelevant, and unavailable
// evidence. Consumers must not render unknown as a negative result.
type SignalState string

// EvidenceSource values identify each normalized rule input.
const (
	SignalPresent       SignalState = "present"
	SignalAbsent        SignalState = "absent"
	SignalNotApplicable SignalState = "not_applicable"
	SignalUnknown       SignalState = "unknown"
)

// EvidenceSource identifies the normalized input that activated a rule.
type EvidenceSource string

// QualitySignalKey values enumerate supported issue-description signals.
const (
	EvidenceTitle              EvidenceSource = "title"
	EvidenceBody               EvidenceSource = "body"
	EvidenceLabel              EvidenceSource = "label"
	EvidenceRepositoryLanguage EvidenceSource = "repository_language"
	EvidenceDependency         EvidenceSource = "dependency"
	EvidenceIssueMetadata      EvidenceSource = "issue_metadata"
	EvidenceDerived            EvidenceSource = "derived"
)

// Evidence records an explainable, stable rule result without copying
// arbitrary upstream content into diagnostics.
type Evidence struct {
	RuleID      string
	Source      EvidenceSource
	Description string
}

// QualitySignalKey names a concrete issue-description quality signal.
type QualitySignalKey string

// TechnologyKind values classify concrete and cross-cutting requirements.
const (
	QualityProblemDescription     QualitySignalKey = "problem_description"
	QualityExpectedBehavior       QualitySignalKey = "expected_behavior"
	QualityCurrentBehavior        QualitySignalKey = "current_behavior"
	QualityReproductionSteps      QualitySignalKey = "reproduction_steps"
	QualityImplementationGuidance QualitySignalKey = "implementation_guidance"
	QualityRelatedFiles           QualitySignalKey = "related_files"
	QualityScreenshot             QualitySignalKey = "screenshot"
	QualityTestMethod             QualitySignalKey = "test_method"
	QualityAcceptanceCriteria     QualitySignalKey = "acceptance_criteria"
)

// QualitySignal describes one issue-body signal and the evidence behind it.
type QualitySignal struct {
	Key      QualitySignalKey
	State    SignalState
	Evidence []Evidence
}

// QualityAssessment is a zero-to-100 specificity score over applicable,
// observable signals.
type QualityAssessment struct {
	Score      int
	Confidence Confidence
	Signals    []QualitySignal
}

// TechnologyKind separates concrete technologies from cross-cutting skills.
type TechnologyKind string

// Category values are the closed rule-based issue classification vocabulary.
const (
	TechnologyLanguage   TechnologyKind = "language"
	TechnologyFramework  TechnologyKind = "framework"
	TechnologyPlatform   TechnologyKind = "platform"
	TechnologyDatabase   TechnologyKind = "database"
	TechnologyPractice   TechnologyKind = "practice"
	TechnologyCapability TechnologyKind = "capability"
)

// RequiredTechnology is one normalized requirement inferred from explicit
// issue, repository, or manifest evidence.
type RequiredTechnology struct {
	Name       string
	Kind       TechnologyKind
	Confidence Confidence
	Evidence   []Evidence
}

// Category is the primary rule-based issue classification.
type Category string

// ChangeArea values name product surfaces that an issue may affect.
const (
	CategoryBug           Category = "bug"
	CategoryFeature       Category = "feature"
	CategoryDocumentation Category = "documentation"
	CategoryTesting       Category = "testing"
	CategoryRefactoring   Category = "refactoring"
	CategoryPerformance   Category = "performance"
	CategoryAccessibility Category = "accessibility"
	CategoryUI            Category = "ui"
	CategoryBackend       Category = "backend"
	CategoryDevOps        Category = "devops"
	CategoryLocalization  Category = "localization"
)

// CategoryAssessment includes the primary category and every matching rule.
// Matching categories are ordered deterministically by rule priority.
type CategoryAssessment struct {
	Primary    Category
	Matches    []Category
	Confidence Confidence
	Evidence   []Evidence
}

// ChangeArea is a typed, extensible estimate of the affected product surface.
type ChangeArea string

// ChangeArea values name product surfaces that an issue may affect.
const (
	ChangeFrontend       ChangeArea = "frontend"
	ChangeBackend        ChangeArea = "backend"
	ChangeTests          ChangeArea = "tests"
	ChangeMigration      ChangeArea = "migration"
	ChangeDocumentation  ChangeArea = "documentation"
	ChangeInfrastructure ChangeArea = "infrastructure"
)

// FileCountBand avoids claiming an exact implementation plan from issue text.
// Maximum is zero only for the open-ended 9+ band.
type FileCountBand struct {
	Minimum int
	Maximum int
	Label   string
}

// ChangeScope estimates affected areas, file-count range, and database impact.
type ChangeScope struct {
	Areas          []ChangeArea
	FileCount      FileCountBand
	DatabaseChange SignalState
	Confidence     Confidence
	Evidence       []Evidence
}

// DifficultyAssessment is the bounded five-level estimate and its evidence.
type DifficultyAssessment struct {
	Level      Difficulty
	Label      string
	Confidence Confidence
	Evidence   []Evidence
}

// EffortBand is the product's intentionally coarse effort scale.
type EffortBand string

// EffortBand values form the ordered, intentionally coarse effort scale.
const (
	EffortThirtyMinutes EffortBand = "thirty_minutes"
	EffortTwoHours      EffortBand = "two_hours"
	EffortHalfDay       EffortBand = "half_day"
	EffortOneDay        EffortBand = "one_day"
	EffortThreeDays     EffortBand = "three_days"
)

// ParseEffortBand validates the closed, ordered effort vocabulary used by
// search filters and analysis responses.
func ParseEffortBand(value string) (EffortBand, error) {
	band := EffortBand(value)
	switch band {
	case EffortThirtyMinutes,
		EffortTwoHours,
		EffortHalfDay,
		EffortOneDay,
		EffortThreeDays:
		return band, nil
	default:
		return "", fmt.Errorf(
			"%w: maximumEffort must be a supported effort band",
			ErrInvalidSearchCriteria,
		)
	}
}

// IsAtMost reports whether an estimate fits within an inclusive maximum.
func (band EffortBand) IsAtMost(maximum EffortBand) bool {
	return effortBandRank(band) > 0 &&
		effortBandRank(band) <= effortBandRank(maximum)
}

func effortBandRank(band EffortBand) int {
	switch band {
	case EffortThirtyMinutes:
		return 1
	case EffortTwoHours:
		return 2
	case EffortHalfDay:
		return 3
	case EffortOneDay:
		return 4
	case EffortThreeDays:
		return 5
	default:
		return 0
	}
}

// EffortEstimate is a coarse, explicitly estimated duration.
type EffortEstimate struct {
	Band       EffortBand
	Label      string
	Confidence Confidence
	Evidence   []Evidence
}

// AnalysisInput is transport-independent input for issue rule analysis.
// Dependencies should contain normalized manifest identifiers when available.
type AnalysisInput struct {
	Candidate             Candidate
	Dependencies          []string
	HasMaintainerGuidance bool
}

// Analysis is the complete deterministic output used by later ranking and
// transport layers.
type Analysis struct {
	Quality              QualityAssessment
	RequiredTechnologies []RequiredTechnology
	Category             CategoryAssessment
	Difficulty           DifficultyAssessment
	Scope                ChangeScope
	Effort               EffortEstimate
	Confidence           Confidence
}
