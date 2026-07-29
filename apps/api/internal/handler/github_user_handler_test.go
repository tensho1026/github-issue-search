package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestGitHubUserHandlerGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	getUser := &getGitHubUserStub{
		output: usecase.GetGitHubUserOutput{
			Profile: user.Profile{
				Login:       "octocat",
				Name:        "The Octocat",
				AvatarURL:   "https://avatars.example/octocat.png",
				Bio:         "GitHub mascot",
				PublicRepos: 8,
				Followers:   10,
				Following:   2,
			},
			Repositories: []repository.Summary{
				{
					Owner:        "octocat",
					Name:         "hello-world",
					FullName:     "octocat/hello-world",
					MainLanguage: "Go",
					Stars:        10,
					UpdatedAt: time.Date(
						2026,
						time.July,
						30,
						1,
						0,
						0,
						0,
						time.UTC,
					),
				},
			},
			RateLimit: port.RateLimit{Known: true, Remaining: 42},
		},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "username", Value: "octocat"}}
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/github/users/octocat",
		nil,
	)

	NewGitHubUserHandler(getUser, response.NewResponder()).Get(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, value := range []string{
		`"login":"octocat"`,
		`"name":"The Octocat"`,
		`"publicRepos":8`,
		`"rateLimitRemaining":42`,
		`"fullName":"octocat/hello-world"`,
		`"mainLanguage":"Go"`,
		`"updatedAt":"2026-07-30T01:00:00Z"`,
	} {
		if !strings.Contains(body, value) {
			t.Errorf("body missing %s: %s", value, body)
		}
	}
	if getUser.username != "octocat" {
		t.Fatalf("usecase username = %q", getUser.username)
	}
}

func TestGitHubUserHandlerRejectsInvalidUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	getUser := &getGitHubUserStub{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "username", Value: "invalid--username"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewGitHubUserHandler(getUser, response.NewResponder()).Get(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	if getUser.called {
		t.Fatal("usecase was called for invalid input")
	}
	if !strings.Contains(recorder.Body.String(), `"code":"INVALID_REQUEST"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestGitHubUserHandlerWritesUsecaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	getUser := &getGitHubUserStub{
		err: apperror.New(
			apperror.CodeGitHubUserNotFound,
			"GitHub user was not found",
			http.StatusNotFound,
		),
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "username", Value: "missing"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewGitHubUserHandler(getUser, response.NewResponder()).Get(ctx)

	if recorder.Code != http.StatusNotFound ||
		!strings.Contains(recorder.Body.String(), "GITHUB_USER_NOT_FOUND") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

type getGitHubUserStub struct {
	output   usecase.GetGitHubUserOutput
	err      error
	username user.Username
	called   bool
}

func (stub *getGitHubUserStub) Execute(
	_ context.Context,
	username user.Username,
) (usecase.GetGitHubUserOutput, error) {
	stub.called = true
	stub.username = username
	return stub.output, stub.err
}

var _ usecase.GetGitHubUser = (*getGitHubUserStub)(nil)
