package issue

import "time"

// Recommendation score limits define the fixed 100-point model invariant.
const (
	MaximumRecommendationScore = 100

	SkillScoreMaximum        = 30
	IssueQualityScoreMaximum = 20
	RepositoryScoreMaximum   = 15
	ActivityScoreMaximum     = 15
	MaintainerScoreMaximum   = 10
	AvailabilityScoreMaximum = 10
)

// MatchStatus records whether a required technology is present in the
// contributor's explicitly supplied skill set. Unknown evidence is excluded
// from the percentage denominator.
type MatchStatus string

// MatchStatus values preserve unavailable evidence separately from mismatch.
const (
	MatchMatched   MatchStatus = "matched"
	MatchUnmatched MatchStatus = "unmatched"
	MatchUnknown   MatchStatus = "unknown"
)

// SkillMatch is one explainable technology comparison.
type SkillMatch struct {
	Technology string
	Status     MatchStatus
	Evidence   []Evidence
}

// SkillMatchAssessment exposes the exact denominator used to calculate the
// percentage so unavailable evidence cannot appear as a mismatch.
type SkillMatchAssessment struct {
	Percentage  int
	Matched     int
	Denominator int
	Skills      []SkillMatch
}

// RepositorySignalKey identifies one independently observed contribution
// readiness signal.
type RepositorySignalKey string

// RepositorySignalKey values enumerate contribution-readiness observations.
const (
	RepositoryREADME        RepositorySignalKey = "readme"
	RepositoryContributing  RepositorySignalKey = "contributing"
	RepositoryCI            RepositorySignalKey = "ci"
	RepositoryTests         RepositorySignalKey = "tests"
	RepositoryCodeOfConduct RepositorySignalKey = "code_of_conduct"
)

// RepositorySignal separates observed absence from incomplete inspection.
type RepositorySignal struct {
	Key      RepositorySignalKey
	State    SignalState
	Evidence []Evidence
}

// CIState is the normalized state of the default branch's latest check rollup.
type CIState string

// CIState values normalize the default branch's latest check rollup.
const (
	CIStateSuccess CIState = "success"
	CIStateFailure CIState = "failure"
	CIStatePending CIState = "pending"
	CIStateUnknown CIState = "unknown"
)

// AggregateStatus prevents missing and empty samples from becoming a
// misleading zero duration.
type AggregateStatus string

// AggregateStatus values distinguish usable samples from absent evidence.
const (
	AggregateAvailable   AggregateStatus = "available"
	AggregateUnavailable AggregateStatus = "unavailable"
)

// DurationAggregate describes a bounded public sample. Median and 90th
// percentile are robust summaries over completed, non-bot observations.
type DurationAggregate struct {
	Status       AggregateStatus
	Median       time.Duration
	Percentile90 time.Duration
	SampleSize   int
	WindowDays   int
	Truncated    bool
	Confidence   Confidence
}

// RatioAggregate describes a bounded numerator/denominator observation.
type RatioAggregate struct {
	Status      AggregateStatus
	Numerator   int
	Denominator int
	Percentage  int
	SampleSize  int
	WindowDays  int
	Truncated   bool
	Confidence  Confidence
}

// CountAggregate is a bounded count observation. Value is meaningful only
// when Status is available.
type CountAggregate struct {
	Status     AggregateStatus
	Value      int
	SampleSize int
	WindowDays int
	Truncated  bool
	Confidence Confidence
}

// ActivityMetrics holds public repository facts and sampled maintenance
// aggregates. A zero value is not meaningful unless the corresponding status
// says it is available.
type ActivityMetrics struct {
	LastMeaningfulUpdate  time.Time
	CI                    CIState
	Contributors          CountAggregate
	PullRequestsOpened    CountAggregate
	StaleOpenPullRequests CountAggregate
	UnansweredIssues      CountAggregate
	PullRequestMerge      RatioAggregate
	IssueResponse         DurationAggregate
	PullRequestReview     DurationAggregate
	PullRequestMergeTime  DurationAggregate
}

// ClaimEvidence is a conservative observation that another contributor has
// explicitly claimed the issue.
type ClaimEvidence struct {
	Claimed    bool
	Confidence Confidence
	Evidence   []Evidence
}

// Severity orders warning presentation and allows clients to style risk.
type Severity string

// Severity values order heuristic warning presentation.
const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Warning is an explainable heuristic risk. Evidence contains normalized
// facts and never arbitrary comment or issue text.
type Warning struct {
	Code     string
	Severity Severity
	Message  string
	Evidence []Evidence
}

// ScoreComponent is one fixed part of the documented 100-point model.
type ScoreComponent struct {
	Name    string
	Score   int
	Maximum int
	Reasons []string
}

// ScoreBreakdown makes the score invariant observable.
type ScoreBreakdown struct {
	SkillMatch        ScoreComponent
	IssueQuality      ScoreComponent
	RepositoryQuality ScoreComponent
	Activity          ScoreComponent
	Maintainer        ScoreComponent
	Availability      ScoreComponent
}

// RecommendationInput is the complete transport-independent scoring input.
type RecommendationInput struct {
	Candidate         Candidate
	Analysis          Analysis
	DesiredSkills     []string
	RepositorySignals []RepositorySignal
	Activity          ActivityMetrics
	Claim             ClaimEvidence
	Now               time.Time
}

// Recommendation is the deterministic ranked result shared by list and
// detail transports.
type Recommendation struct {
	Score             int
	Breakdown         ScoreBreakdown
	SkillMatch        SkillMatchAssessment
	RepositorySignals []RepositorySignal
	Activity          ActivityMetrics
	Claim             ClaimEvidence
	Reasons           []string
	Warnings          []Warning
}

// RankedIssue is the shared evaluated shape used by both list and detail
// application flows.
type RankedIssue struct {
	Candidate      Candidate
	Analysis       Analysis
	Recommendation Recommendation
}

// CommentObservation contains only the bounded public fields needed for
// conservative claim detection.
type CommentObservation struct {
	AuthorLogin       string
	AuthorType        string
	AuthorAssociation string
	Body              string
	CreatedAt         time.Time
}
