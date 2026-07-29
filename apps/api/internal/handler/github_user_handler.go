package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

type GitHubUserHandler struct {
	getUser   usecase.GetGitHubUser
	responder response.Responder
}

func NewGitHubUserHandler(
	getUser usecase.GetGitHubUser,
	responder response.Responder,
) GitHubUserHandler {
	return GitHubUserHandler{
		getUser:   getUser,
		responder: responder,
	}
}

func (h GitHubUserHandler) Get(ctx *gin.Context) {
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

	output, err := h.getUser.Execute(ctx.Request.Context(), username)
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
		newGitHubUserResponse(output.Profile, output.Repositories),
		response.MetaOptions{RateLimitRemaining: remaining},
	)
}

type githubUserResponse struct {
	Login        string                      `json:"login"`
	Name         string                      `json:"name"`
	AvatarURL    string                      `json:"avatarUrl"`
	Bio          string                      `json:"bio"`
	PublicRepos  int                         `json:"publicRepos"`
	Followers    int                         `json:"followers"`
	Following    int                         `json:"following"`
	Repositories []repositorySummaryResponse `json:"repositories"`
}

type repositorySummaryResponse struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Description   string `json:"description"`
	URL           string `json:"url"`
	MainLanguage  string `json:"mainLanguage"`
	Stars         int    `json:"stars"`
	Forks         int    `json:"forks"`
	OpenIssues    int    `json:"openIssues"`
	IsFork        bool   `json:"isFork"`
	IsArchived    bool   `json:"isArchived"`
	DefaultBranch string `json:"defaultBranch"`
	UpdatedAt     string `json:"updatedAt"`
	PushedAt      string `json:"pushedAt"`
}

func newGitHubUserResponse(
	profile user.Profile,
	repositories []repository.Summary,
) githubUserResponse {
	repositoryResponses := make(
		[]repositorySummaryResponse,
		0,
		len(repositories),
	)
	for _, item := range repositories {
		repositoryResponses = append(
			repositoryResponses,
			newRepositorySummaryResponse(item),
		)
	}
	return githubUserResponse{
		Login:        profile.Login.String(),
		Name:         profile.Name,
		AvatarURL:    profile.AvatarURL,
		Bio:          profile.Bio,
		PublicRepos:  profile.PublicRepos,
		Followers:    profile.Followers,
		Following:    profile.Following,
		Repositories: repositoryResponses,
	}
}

func newRepositorySummaryResponse(
	item repository.Summary,
) repositorySummaryResponse {
	return repositorySummaryResponse{
		Owner:         item.Owner,
		Name:          item.Name,
		FullName:      item.FullName,
		Description:   item.Description,
		URL:           item.URL,
		MainLanguage:  item.MainLanguage,
		Stars:         item.Stars,
		Forks:         item.Forks,
		OpenIssues:    item.OpenIssues,
		IsFork:        item.IsFork,
		IsArchived:    item.IsArchived,
		DefaultBranch: item.DefaultBranch,
		UpdatedAt:     formatOptionalTime(item.UpdatedAt),
		PushedAt:      formatOptionalTime(item.PushedAt),
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
