package github_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	githubclient "github.com/tensho1026/github-issue-search/apps/api/internal/client/github"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

func ExampleNewClient() {
	endpoint, err := url.Parse("https://api.github.com")
	if err != nil {
		panic(err)
	}
	client := githubclient.NewClient(
		endpoint,
		"",
		5*time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	username, err := user.ParseUsername("octocat")
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.GetUser(ctx, username)
	fmt.Println("cancellation propagated:", errors.Is(err, context.Canceled))

	// Output:
	// cancellation propagated: true
}
