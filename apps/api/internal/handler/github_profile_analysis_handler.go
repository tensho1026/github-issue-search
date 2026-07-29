package handler

import (
	"net/http"

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
	Username             string                   `json:"username"`
	Languages            []languageShareResponse  `json:"languages"`
	Frameworks           []string                 `json:"frameworks"`
	RepositoriesAnalyzed int                      `json:"repositoriesAnalyzed"`
	Warnings             []profileWarningResponse `json:"warnings"`
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

func newGitHubProfileAnalysisResponse(
	analysis profile.Analysis,
) githubProfileAnalysisResponse {
	languages := make([]languageShareResponse, 0, len(analysis.Languages))
	for _, language := range analysis.Languages {
		languages = append(languages, languageShareResponse{
			Name:       language.Name,
			Percentage: language.Percentage,
		})
	}
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
		Username:             analysis.Username.String(),
		Languages:            languages,
		Frameworks:           frameworks,
		RepositoriesAnalyzed: analysis.RepositoriesAnalyzed,
		Warnings:             warnings,
	}
}
