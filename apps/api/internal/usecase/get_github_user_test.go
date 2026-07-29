package usecase

import (
	"context"
	"errors"
	"net/http"
	"testing"

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
	}

	output, err := NewGetGitHubUser(reader).Execute(
		context.Background(),
		"octocat",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Profile.Login != "octocat" ||
		output.Profile.Name != "The Octocat" ||
		output.RateLimit.Remaining != 42 {
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
			}).Execute(context.Background(), "octocat")

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
	result port.GitHubUserResult
	err    error
}

func (stub githubUserReaderStub) GetUser(
	context.Context,
	user.Username,
) (port.GitHubUserResult, error) {
	return stub.result, stub.err
}
