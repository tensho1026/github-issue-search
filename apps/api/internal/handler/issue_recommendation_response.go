package handler

import (
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
)

type evidenceResponse struct {
	RuleID      string               `json:"ruleId"`
	Source      issue.EvidenceSource `json:"source"`
	Description string               `json:"description"`
}

type recommendationResponse struct {
	Score      int                             `json:"score"`
	Breakdown  []scoreComponentResponse        `json:"breakdown"`
	SkillMatch skillMatchResponse              `json:"skillMatch"`
	Reasons    []string                        `json:"reasons"`
	Warnings   []recommendationWarningResponse `json:"warnings"`
	Claim      claimEvidenceResponse           `json:"claim"`
}

type scoreComponentResponse struct {
	Name    string   `json:"name"`
	Score   int      `json:"score"`
	Maximum int      `json:"maximum"`
	Reasons []string `json:"reasons"`
}

type skillMatchResponse struct {
	Percentage  int                      `json:"percentage"`
	Matched     int                      `json:"matched"`
	Denominator int                      `json:"denominator"`
	Skills      []skillMatchItemResponse `json:"skills"`
}

type skillMatchItemResponse struct {
	Technology string             `json:"technology"`
	Status     issue.MatchStatus  `json:"status"`
	Evidence   []evidenceResponse `json:"evidence"`
}

type recommendationWarningResponse struct {
	Code     string             `json:"code"`
	Severity issue.Severity     `json:"severity"`
	Message  string             `json:"message"`
	Evidence []evidenceResponse `json:"evidence"`
}

type claimEvidenceResponse struct {
	Claimed    bool               `json:"claimed"`
	Confidence issue.Confidence   `json:"confidence"`
	Evidence   []evidenceResponse `json:"evidence"`
}

type issueAnalysisResponse struct {
	Quality              qualityAssessmentResponse    `json:"quality"`
	RequiredTechnologies []requiredTechnologyResponse `json:"requiredTechnologies"`
	Category             categoryAssessmentResponse   `json:"category"`
	Difficulty           difficultyAssessmentResponse `json:"difficulty"`
	Scope                changeScopeResponse          `json:"scope"`
	Effort               effortEstimateResponse       `json:"effort"`
	Confidence           issue.Confidence             `json:"confidence"`
}

type qualityAssessmentResponse struct {
	Score      int                     `json:"score"`
	Confidence issue.Confidence        `json:"confidence"`
	Signals    []qualitySignalResponse `json:"signals"`
}

type qualitySignalResponse struct {
	Key      issue.QualitySignalKey `json:"key"`
	State    issue.SignalState      `json:"state"`
	Evidence []evidenceResponse     `json:"evidence"`
}

type requiredTechnologyResponse struct {
	Name       string               `json:"name"`
	Kind       issue.TechnologyKind `json:"kind"`
	Confidence issue.Confidence     `json:"confidence"`
	Evidence   []evidenceResponse   `json:"evidence"`
}

type categoryAssessmentResponse struct {
	Primary    issue.Category     `json:"primary"`
	Matches    []issue.Category   `json:"matches"`
	Confidence issue.Confidence   `json:"confidence"`
	Evidence   []evidenceResponse `json:"evidence"`
}

type difficultyAssessmentResponse struct {
	Level      int                `json:"level"`
	Label      string             `json:"label"`
	Confidence issue.Confidence   `json:"confidence"`
	Evidence   []evidenceResponse `json:"evidence"`
}

type changeScopeResponse struct {
	Areas          []issue.ChangeArea    `json:"areas"`
	FileCount      fileCountBandResponse `json:"fileCount"`
	DatabaseChange issue.SignalState     `json:"databaseChange"`
	Confidence     issue.Confidence      `json:"confidence"`
	Evidence       []evidenceResponse    `json:"evidence"`
}

type fileCountBandResponse struct {
	Minimum int    `json:"minimum"`
	Maximum *int   `json:"maximum"`
	Label   string `json:"label"`
}

type effortEstimateResponse struct {
	Band       issue.EffortBand   `json:"band"`
	Label      string             `json:"label"`
	Confidence issue.Confidence   `json:"confidence"`
	Evidence   []evidenceResponse `json:"evidence"`
}

type repositorySignalResponse struct {
	Key      issue.RepositorySignalKey `json:"key"`
	State    issue.SignalState         `json:"state"`
	Evidence []evidenceResponse        `json:"evidence"`
}

type activityMetricsResponse struct {
	LastMeaningfulUpdate  *time.Time                `json:"lastMeaningfulUpdate"`
	CI                    issue.CIState             `json:"ci"`
	Contributors          countAggregateResponse    `json:"contributors"`
	PullRequestsOpened    countAggregateResponse    `json:"pullRequestsOpened"`
	StaleOpenPullRequests countAggregateResponse    `json:"staleOpenPullRequests"`
	UnansweredIssues      countAggregateResponse    `json:"unansweredIssues"`
	PullRequestMerge      ratioAggregateResponse    `json:"pullRequestMerge"`
	IssueResponse         durationAggregateResponse `json:"issueResponse"`
	PullRequestReview     durationAggregateResponse `json:"pullRequestReview"`
	PullRequestMergeTime  durationAggregateResponse `json:"pullRequestMergeTime"`
}

type aggregateMetadataResponse struct {
	Status     issue.AggregateStatus `json:"status"`
	SampleSize int                   `json:"sampleSize"`
	WindowDays int                   `json:"windowDays"`
	Truncated  bool                  `json:"truncated"`
	Confidence issue.Confidence      `json:"confidence"`
}

type countAggregateResponse struct {
	aggregateMetadataResponse
	Value *int `json:"value"`
}

type ratioAggregateResponse struct {
	aggregateMetadataResponse
	Numerator   *int `json:"numerator"`
	Denominator *int `json:"denominator"`
	Percentage  *int `json:"percentage"`
}

type durationAggregateResponse struct {
	aggregateMetadataResponse
	MedianSeconds       *int64 `json:"medianSeconds"`
	Percentile90Seconds *int64 `json:"percentile90Seconds"`
}

func newRecommendationResponse(
	recommendation issue.Recommendation,
) recommendationResponse {
	components := []issue.ScoreComponent{
		recommendation.Breakdown.SkillMatch,
		recommendation.Breakdown.IssueQuality,
		recommendation.Breakdown.RepositoryQuality,
		recommendation.Breakdown.Activity,
		recommendation.Breakdown.Maintainer,
		recommendation.Breakdown.Availability,
	}
	breakdown := make([]scoreComponentResponse, 0, len(components))
	for _, component := range components {
		breakdown = append(breakdown, scoreComponentResponse{
			Name:    component.Name,
			Score:   component.Score,
			Maximum: component.Maximum,
			Reasons: append([]string(nil), component.Reasons...),
		})
	}
	skills := make(
		[]skillMatchItemResponse,
		0,
		len(recommendation.SkillMatch.Skills),
	)
	for _, skill := range recommendation.SkillMatch.Skills {
		skills = append(skills, skillMatchItemResponse{
			Technology: skill.Technology,
			Status:     skill.Status,
			Evidence:   newEvidenceResponses(skill.Evidence),
		})
	}
	warnings := make(
		[]recommendationWarningResponse,
		0,
		len(recommendation.Warnings),
	)
	for _, warning := range recommendation.Warnings {
		warnings = append(warnings, recommendationWarningResponse{
			Code:     warning.Code,
			Severity: warning.Severity,
			Message:  warning.Message,
			Evidence: newEvidenceResponses(warning.Evidence),
		})
	}
	return recommendationResponse{
		Score:     recommendation.Score,
		Breakdown: breakdown,
		SkillMatch: skillMatchResponse{
			Percentage:  recommendation.SkillMatch.Percentage,
			Matched:     recommendation.SkillMatch.Matched,
			Denominator: recommendation.SkillMatch.Denominator,
			Skills:      skills,
		},
		Reasons:  append([]string(nil), recommendation.Reasons...),
		Warnings: warnings,
		Claim: claimEvidenceResponse{
			Claimed:    recommendation.Claim.Claimed,
			Confidence: recommendation.Claim.Confidence,
			Evidence:   newEvidenceResponses(recommendation.Claim.Evidence),
		},
	}
}

func newIssueAnalysisResponse(
	analysis issue.Analysis,
) issueAnalysisResponse {
	qualitySignals := make(
		[]qualitySignalResponse,
		0,
		len(analysis.Quality.Signals),
	)
	for _, signal := range analysis.Quality.Signals {
		qualitySignals = append(qualitySignals, qualitySignalResponse{
			Key:      signal.Key,
			State:    signal.State,
			Evidence: newEvidenceResponses(signal.Evidence),
		})
	}
	technologies := make(
		[]requiredTechnologyResponse,
		0,
		len(analysis.RequiredTechnologies),
	)
	for _, technology := range analysis.RequiredTechnologies {
		technologies = append(technologies, requiredTechnologyResponse{
			Name:       technology.Name,
			Kind:       technology.Kind,
			Confidence: technology.Confidence,
			Evidence:   newEvidenceResponses(technology.Evidence),
		})
	}
	maximum := analysis.Scope.FileCount.Maximum
	var maximumPointer *int
	if maximum > 0 {
		maximumPointer = &maximum
	}
	return issueAnalysisResponse{
		Quality: qualityAssessmentResponse{
			Score:      analysis.Quality.Score,
			Confidence: analysis.Quality.Confidence,
			Signals:    qualitySignals,
		},
		RequiredTechnologies: technologies,
		Category: categoryAssessmentResponse{
			Primary:    analysis.Category.Primary,
			Matches:    append([]issue.Category(nil), analysis.Category.Matches...),
			Confidence: analysis.Category.Confidence,
			Evidence:   newEvidenceResponses(analysis.Category.Evidence),
		},
		Difficulty: difficultyAssessmentResponse{
			Level:      analysis.Difficulty.Level.Int(),
			Label:      analysis.Difficulty.Label,
			Confidence: analysis.Difficulty.Confidence,
			Evidence:   newEvidenceResponses(analysis.Difficulty.Evidence),
		},
		Scope: changeScopeResponse{
			Areas: append([]issue.ChangeArea(nil), analysis.Scope.Areas...),
			FileCount: fileCountBandResponse{
				Minimum: analysis.Scope.FileCount.Minimum,
				Maximum: maximumPointer,
				Label:   analysis.Scope.FileCount.Label,
			},
			DatabaseChange: analysis.Scope.DatabaseChange,
			Confidence:     analysis.Scope.Confidence,
			Evidence:       newEvidenceResponses(analysis.Scope.Evidence),
		},
		Effort: effortEstimateResponse{
			Band:       analysis.Effort.Band,
			Label:      analysis.Effort.Label,
			Confidence: analysis.Effort.Confidence,
			Evidence:   newEvidenceResponses(analysis.Effort.Evidence),
		},
		Confidence: analysis.Confidence,
	}
}

func newRepositorySignalResponses(
	signals []issue.RepositorySignal,
) []repositorySignalResponse {
	responses := make([]repositorySignalResponse, 0, len(signals))
	for _, signal := range signals {
		responses = append(responses, repositorySignalResponse{
			Key:      signal.Key,
			State:    signal.State,
			Evidence: newEvidenceResponses(signal.Evidence),
		})
	}
	return responses
}

func newActivityMetricsResponse(
	activity issue.ActivityMetrics,
) activityMetricsResponse {
	var lastMeaningfulUpdate *time.Time
	if !activity.LastMeaningfulUpdate.IsZero() {
		value := activity.LastMeaningfulUpdate.UTC()
		lastMeaningfulUpdate = &value
	}
	return activityMetricsResponse{
		LastMeaningfulUpdate: lastMeaningfulUpdate,
		CI:                   activity.CI,
		Contributors:         newCountAggregateResponse(activity.Contributors),
		PullRequestsOpened: newCountAggregateResponse(
			activity.PullRequestsOpened,
		),
		StaleOpenPullRequests: newCountAggregateResponse(
			activity.StaleOpenPullRequests,
		),
		UnansweredIssues: newCountAggregateResponse(
			activity.UnansweredIssues,
		),
		PullRequestMerge: newRatioAggregateResponse(
			activity.PullRequestMerge,
		),
		IssueResponse: newDurationAggregateResponse(
			activity.IssueResponse,
		),
		PullRequestReview: newDurationAggregateResponse(
			activity.PullRequestReview,
		),
		PullRequestMergeTime: newDurationAggregateResponse(
			activity.PullRequestMergeTime,
		),
	}
}

func newCountAggregateResponse(
	aggregate issue.CountAggregate,
) countAggregateResponse {
	response := countAggregateResponse{
		aggregateMetadataResponse: newAggregateMetadata(
			aggregate.Status,
			aggregate.SampleSize,
			aggregate.WindowDays,
			aggregate.Truncated,
			aggregate.Confidence,
		),
	}
	if aggregate.Status == issue.AggregateAvailable {
		value := aggregate.Value
		response.Value = &value
	}
	return response
}

func newRatioAggregateResponse(
	aggregate issue.RatioAggregate,
) ratioAggregateResponse {
	response := ratioAggregateResponse{
		aggregateMetadataResponse: newAggregateMetadata(
			aggregate.Status,
			aggregate.SampleSize,
			aggregate.WindowDays,
			aggregate.Truncated,
			aggregate.Confidence,
		),
	}
	if aggregate.Status == issue.AggregateAvailable {
		numerator := aggregate.Numerator
		denominator := aggregate.Denominator
		percentage := aggregate.Percentage
		response.Numerator = &numerator
		response.Denominator = &denominator
		response.Percentage = &percentage
	}
	return response
}

func newDurationAggregateResponse(
	aggregate issue.DurationAggregate,
) durationAggregateResponse {
	response := durationAggregateResponse{
		aggregateMetadataResponse: newAggregateMetadata(
			aggregate.Status,
			aggregate.SampleSize,
			aggregate.WindowDays,
			aggregate.Truncated,
			aggregate.Confidence,
		),
	}
	if aggregate.Status == issue.AggregateAvailable {
		median := int64(aggregate.Median / time.Second)
		percentile90 := int64(aggregate.Percentile90 / time.Second)
		response.MedianSeconds = &median
		response.Percentile90Seconds = &percentile90
	}
	return response
}

func newAggregateMetadata(
	status issue.AggregateStatus,
	sampleSize int,
	windowDays int,
	truncated bool,
	confidence issue.Confidence,
) aggregateMetadataResponse {
	if status == "" {
		status = issue.AggregateUnavailable
	}
	if confidence == "" {
		confidence = issue.ConfidenceLow
	}
	return aggregateMetadataResponse{
		Status:     status,
		SampleSize: sampleSize,
		WindowDays: windowDays,
		Truncated:  truncated,
		Confidence: confidence,
	}
}

func newEvidenceResponses(
	evidence []issue.Evidence,
) []evidenceResponse {
	responses := make([]evidenceResponse, 0, len(evidence))
	for _, item := range evidence {
		responses = append(responses, evidenceResponse{
			RuleID:      item.RuleID,
			Source:      item.Source,
			Description: item.Description,
		})
	}
	return responses
}
