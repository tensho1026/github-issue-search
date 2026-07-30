package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

type GitHubProfileAnalysisHandler struct {
	analyze   usecase.AnalyzeGitHubProfile
	responder response.Responder
}

func NewGitHubProfileAnalysisHandler(
	analyze usecase.AnalyzeGitHubProfile,
	responder response.Responder,
) GitHubProfileAnalysisHandler {
	return GitHubProfileAnalysisHandler{
		analyze:   analyze,
		responder: responder,
	}
}

func (h GitHubProfileAnalysisHandler) Get(ctx *gin.Context) {
	username, err := user.ParseUsername(ctx.Param("username"))
	if err != nil {
		h.responder.Error(ctx, apperror.Wrap(
			apperror.CodeInvalidRequest,
			"GitHub username is invalid",
			http.StatusBadRequest,
			err,
		))
		return
	}

	output, err := h.analyze.Execute(ctx.Request.Context(), username)
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	var remaining *int
	if output.RateLimit.Known {
		remaining = &output.RateLimit.Remaining
	}
	h.responder.DataWithMeta(
		ctx,
		http.StatusOK,
		newGitHubProfileAnalysisResponse(output.Analysis),
		response.MetaOptions{RateLimitRemaining: remaining},
	)
}

type githubProfileAnalysisResponse struct {
	Username             string                          `json:"username"`
	Languages            []languageShareResponse         `json:"languages"`
	Frameworks           []string                        `json:"frameworks"`
	RecentTechnologies   []recentTechnologyResponse      `json:"recentTechnologies"`
	Contributions        contributionAnalysisResponse    `json:"contributions"`
	OSSExperience        ossExperienceResponse           `json:"ossExperience"`
	RepositoryEvidence   repositoryEvidenceResponse      `json:"repositoryEvidence"`
	Proficiency          []technologyProficiencyResponse `json:"proficiency"`
	AnalysisWindow       analysisWindowResponse          `json:"analysisWindow"`
	RepositoriesAnalyzed int                             `json:"repositoriesAnalyzed"`
	Warnings             []profileWarningResponse        `json:"warnings"`
}

type languageShareResponse struct {
	Name       string `json:"name"`
	Percentage int    `json:"percentage"`
}

type profileWarningResponse struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Repository string `json:"repository,omitempty"`
}

type recentTechnologyResponse struct {
	Name              string   `json:"name"`
	Kind              string   `json:"kind"`
	LastUsedAt        string   `json:"lastUsedAt"`
	RepositoryCount   int      `json:"repositoryCount"`
	RepositorySources []string `json:"repositorySources"`
	Confidence        string   `json:"confidence"`
}

type countMetricResponse struct {
	Value  int    `json:"value"`
	Status string `json:"status"`
}

type contributionAnalysisResponse struct {
	WindowDays          int                 `json:"windowDays"`
	Commits             countMetricResponse `json:"commits"`
	IssuesOpened        countMetricResponse `json:"issuesOpened"`
	PullRequestsOpened  countMetricResponse `json:"pullRequestsOpened"`
	PullRequestReviews  countMetricResponse `json:"pullRequestReviews"`
	RepositoriesTouched countMetricResponse `json:"repositoriesTouched"`
}

type technologyEvidenceResponse struct {
	Kind   string `json:"kind"`
	Value  int    `json:"value"`
	Status string `json:"status"`
}

type ossExperienceResponse struct {
	Level      string                       `json:"level"`
	Confidence string                       `json:"confidence"`
	PublicOnly bool                         `json:"publicOnly"`
	Evidence   []technologyEvidenceResponse `json:"evidence"`
}

type repositorySampleResponse struct {
	Status              string                  `json:"status"`
	Observed            int                     `json:"observed"`
	Total               *int                    `json:"total"`
	Limit               int                     `json:"limit"`
	ActiveInWindow      int                     `json:"activeInWindow"`
	PrimaryTechnologies []languageShareResponse `json:"primaryTechnologies"`
}

type repositoryEvidenceResponse struct {
	Owned       repositorySampleResponse `json:"owned"`
	Contributed repositorySampleResponse `json:"contributed"`
	Starred     repositorySampleResponse `json:"starred"`
	Forked      repositorySampleResponse `json:"forked"`
}

type technologyProficiencyResponse struct {
	Name       string                       `json:"name"`
	Kind       string                       `json:"kind"`
	Level      int                          `json:"level"`
	Label      string                       `json:"label"`
	Score      int                          `json:"score"`
	Confidence string                       `json:"confidence"`
	Evidence   []technologyEvidenceResponse `json:"evidence"`
}

type analysisWindowResponse struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Days       int    `json:"days"`
	PublicOnly bool   `json:"publicOnly"`
}

func newGitHubProfileAnalysisResponse(
	analysis profile.Analysis,
) githubProfileAnalysisResponse {
	warnings := make([]profileWarningResponse, 0, len(analysis.Warnings))
	for _, warning := range analysis.Warnings {
		warnings = append(warnings, profileWarningResponse{
			Code:       warning.Code,
			Message:    warning.Message,
			Repository: warning.Repository,
		})
	}
	frameworks := make([]string, len(analysis.Frameworks))
	copy(frameworks, analysis.Frameworks)

	return githubProfileAnalysisResponse{
		Username:   analysis.Username.String(),
		Languages:  newLanguageShareResponses(analysis.Languages),
		Frameworks: frameworks,
		RecentTechnologies: newRecentTechnologyResponses(
			analysis.RecentTechnologies,
		),
		Contributions: newContributionAnalysisResponse(
			analysis.Contributions,
		),
		OSSExperience: newOSSExperienceResponse(analysis.OSSExperience),
		RepositoryEvidence: newRepositoryEvidenceResponse(
			analysis.RepositoryEvidence,
		),
		Proficiency: newTechnologyProficiencyResponses(
			analysis.Proficiency,
		),
		AnalysisWindow: analysisWindowResponse{
			From:       analysis.Window.From.Format(time.RFC3339),
			To:         analysis.Window.To.Format(time.RFC3339),
			Days:       analysis.Window.Days,
			PublicOnly: analysis.Window.PublicOnly,
		},
		RepositoriesAnalyzed: analysis.RepositoriesAnalyzed,
		Warnings:             warnings,
	}
}

func newLanguageShareResponses(
	languages []profile.LanguageShare,
) []languageShareResponse {
	result := make([]languageShareResponse, 0, len(languages))
	for _, language := range languages {
		result = append(result, languageShareResponse{
			Name:       language.Name,
			Percentage: language.Percentage,
		})
	}
	return result
}

func newRecentTechnologyResponses(
	technologies []profile.RecentTechnology,
) []recentTechnologyResponse {
	result := make([]recentTechnologyResponse, 0, len(technologies))
	for _, technology := range technologies {
		sources := make(
			[]string,
			0,
			len(technology.RepositorySources),
		)
		for _, source := range technology.RepositorySources {
			sources = append(sources, string(source))
		}
		result = append(result, recentTechnologyResponse{
			Name:              technology.Name,
			Kind:              string(technology.Kind),
			LastUsedAt:        technology.LastUsedAt.Format(time.RFC3339),
			RepositoryCount:   technology.RepositoryCount,
			RepositorySources: sources,
			Confidence:        string(technology.Confidence),
		})
	}
	return result
}

func newCountMetricResponse(metric profile.CountMetric) countMetricResponse {
	return countMetricResponse{
		Value:  metric.Value,
		Status: string(metric.Status),
	}
}

func newContributionAnalysisResponse(
	analysis profile.ContributionAnalysis,
) contributionAnalysisResponse {
	return contributionAnalysisResponse{
		WindowDays:   analysis.WindowDays,
		Commits:      newCountMetricResponse(analysis.Commits),
		IssuesOpened: newCountMetricResponse(analysis.IssuesOpened),
		PullRequestsOpened: newCountMetricResponse(
			analysis.PullRequestsOpened,
		),
		PullRequestReviews: newCountMetricResponse(
			analysis.PullRequestReviews,
		),
		RepositoriesTouched: newCountMetricResponse(
			analysis.RepositoriesTouched,
		),
	}
}

func newTechnologyEvidenceResponses(
	evidence []profile.TechnologyEvidence,
) []technologyEvidenceResponse {
	result := make([]technologyEvidenceResponse, 0, len(evidence))
	for _, item := range evidence {
		result = append(result, technologyEvidenceResponse{
			Kind:   item.Kind,
			Value:  item.Value,
			Status: string(item.Status),
		})
	}
	return result
}

func newOSSExperienceResponse(
	experience profile.OSSExperience,
) ossExperienceResponse {
	return ossExperienceResponse{
		Level:      experience.Level,
		Confidence: string(experience.Confidence),
		PublicOnly: experience.PublicOnly,
		Evidence:   newTechnologyEvidenceResponses(experience.Evidence),
	}
}

func newRepositoryEvidenceResponse(
	evidence profile.RepositoryEvidence,
) repositoryEvidenceResponse {
	return repositoryEvidenceResponse{
		Owned:       newRepositorySampleResponse(evidence.Owned),
		Contributed: newRepositorySampleResponse(evidence.Contributed),
		Starred:     newRepositorySampleResponse(evidence.Starred),
		Forked:      newRepositorySampleResponse(evidence.Forked),
	}
}

func newRepositorySampleResponse(
	sample profile.RepositorySample,
) repositorySampleResponse {
	return repositorySampleResponse{
		Status:         string(sample.Status),
		Observed:       sample.Observed,
		Total:          sample.Total,
		Limit:          sample.Limit,
		ActiveInWindow: sample.ActiveInWindow,
		PrimaryTechnologies: newLanguageShareResponses(
			sample.PrimaryTechnologies,
		),
	}
}

func newTechnologyProficiencyResponses(
	proficiency []profile.TechnologyProficiency,
) []technologyProficiencyResponse {
	result := make(
		[]technologyProficiencyResponse,
		0,
		len(proficiency),
	)
	for _, technology := range proficiency {
		result = append(result, technologyProficiencyResponse{
			Name:       technology.Name,
			Kind:       string(technology.Kind),
			Level:      technology.Level,
			Label:      technology.Label,
			Score:      technology.Score,
			Confidence: string(technology.Confidence),
			Evidence:   newTechnologyEvidenceResponses(technology.Evidence),
		})
	}
	return result
}
