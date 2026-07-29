package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
		newGitHubUserResponse(output.Profile),
		response.MetaOptions{RateLimitRemaining: remaining},
	)
}

type githubUserResponse struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatarUrl"`
	Bio         string `json:"bio"`
	PublicRepos int    `json:"publicRepos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
}

func newGitHubUserResponse(profile user.Profile) githubUserResponse {
	return githubUserResponse{
		Login:       profile.Login.String(),
		Name:        profile.Name,
		AvatarURL:   profile.AvatarURL,
		Bio:         profile.Bio,
		PublicRepos: profile.PublicRepos,
		Followers:   profile.Followers,
		Following:   profile.Following,
	}
}
