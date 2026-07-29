package usecase

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestGetGitHubUserExecute(t *testing.T) {
	reader := githubUserReaderStub{
		result: port.GitHubUserResult{
			Profile: user.Profile{
				Login:       "octocat",
				Name:        "The Octocat",
				PublicRepos: 8,
			},
			RateLimit: port.RateLimit{Remaining: 42},
		},
		repositories: []repository.Summary{
			{Name: "hello-world", FullName: "octocat/hello-world"},
		},
		repositoryRateLimit: port.RateLimit{Known: true, Remaining: 41},
	}

	output, err := NewGetGitHubUser(reader, 20).Execute(
		context.Background(),
		"octocat",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Profile.Login != "octocat" ||
		output.Profile.Name != "The Octocat" ||
		len(output.Repositories) != 1 ||
		output.RateLimit.Remaining != 41 {
		t.Fatalf("Execute() output = %+v", output)
	}
}

func TestGetGitHubUserMapsErrors(t *testing.T) {
	tests := []struct {
		name       string
		readerErr  error
		wantCode   apperror.Code
		wantStatus int
	}{
		{
			name:       "not found",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorNotFound},
			wantCode:   apperror.CodeGitHubUserNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rate limit",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorRateLimited},
			wantCode:   apperror.CodeRateLimit,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "upstream",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorUpstream},
			wantCode:   apperror.CodeGitHubAPI,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "cancelled",
			readerErr:  context.Canceled,
			wantCode:   apperror.CodeRequestTimeout,
			wantStatus: http.StatusGatewayTimeout,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGetGitHubUser(githubUserReaderStub{
				err: test.readerErr,
			}, 20).Execute(context.Background(), "octocat")

			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) {
				t.Fatalf("Execute() error = %v", err)
			}
			if applicationError.Code != test.wantCode ||
				applicationError.HTTPStatus != test.wantStatus {
				t.Fatalf("Execute() error = %+v", applicationError)
			}
			if !errors.Is(applicationError, test.readerErr) {
				t.Fatalf("Execute() did not preserve cause")
			}
		})
	}
}

type githubUserReaderStub struct {
	result              port.GitHubUserResult
	repositories        []repository.Summary
	repositoryRateLimit port.RateLimit
	err                 error
}

func (stub githubUserReaderStub) GetUser(
	context.Context,
	user.Username,
) (port.GitHubUserResult, error) {
	return stub.result, stub.err
}

func (stub githubUserReaderStub) ListRepositories(
	context.Context,
	user.Username,
	int,
) ([]repository.Summary, port.RateLimit, error) {
	return stub.repositories, stub.repositoryRateLimit, stub.err
}
