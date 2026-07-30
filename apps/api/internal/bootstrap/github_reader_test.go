package bootstrap

import (
	"io"
	"log/slog"
	"net/url"
	"testing"
	"time"

	githubclient "github.com/tensho1026/github-issue-search/apps/api/internal/client/github"
	"github.com/tensho1026/github-issue-search/apps/api/internal/client/githubmock"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
)

func TestNewGitHubReaderSelectsValidatedAdapter(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	baseURL, err := url.Parse("https://api.github.example")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	mock := NewGitHubReader(config.Config{
		AppEnvironment:   "test",
		UseGitHubAPIMock: true,
	}, logger)
	if _, ok := mock.(*githubmock.Client); !ok {
		t.Fatalf("mock reader type = %T", mock)
	}

	live := NewGitHubReader(config.Config{
		AppEnvironment:       "production",
		GitHubAPIBaseURL:     baseURL,
		GitHubRequestTimeout: time.Second,
	}, logger)
	if _, ok := live.(*githubclient.Client); !ok {
		t.Fatalf("live reader type = %T", live)
	}
}
