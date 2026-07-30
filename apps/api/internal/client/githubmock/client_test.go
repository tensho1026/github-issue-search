package githubmock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

var fixtureNow = time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)

func TestClientSupportsCompleteSuccessJourney(t *testing.T) {
	client := New(func() time.Time { return fixtureNow })
	username := user.Username(successUsername)

	profileResult, err := client.GetUser(context.Background(), username)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if profileResult.Profile.Login != username ||
		profileResult.Profile.Name != "The Octocat" ||
		profileResult.RateLimit.Remaining != 4992 {
		t.Fatalf("GetUser() = %+v", profileResult)
	}

	repositories, _, err := client.ListRepositories(
		context.Background(),
		username,
		20,
	)
	if err != nil {
		t.Fatalf("ListRepositories() error = %v", err)
	}
	if len(repositories) != 1 ||
		repositories[0].FullName != "octocat/typed-service" {
		t.Fatalf("ListRepositories() = %+v", repositories)
	}

	analysis, err := client.GetProfileAnalysis(
		context.Background(),
		username,
		20,
		3,
	)
	if err != nil {
		t.Fatalf("GetProfileAnalysis() error = %v", err)
	}
	if len(analysis.Snapshot.Owned.Repositories) != 1 ||
		len(analysis.Snapshot.Contributed.Repositories) != 1 ||
		len(analysis.Snapshot.Starred.Repositories) != 1 ||
		len(analysis.Snapshot.Forked.Repositories) != 1 ||
		analysis.Snapshot.Contributions.PullRequestsOpened.Value != 7 {
		t.Fatalf("GetProfileAnalysis() = %+v", analysis)
	}

	criteria := mustCriteria(t, successUsername)
	search, err := client.SearchIssues(context.Background(), criteria, 20)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}
	if len(search.Candidates) != 1 ||
		search.Candidates[0].Issue.Number != fixtureIssue ||
		search.TotalCount != 1 {
		t.Fatalf("SearchIssues() = %+v", search)
	}

	repositorySearch, err := client.SearchRepositories(
		context.Background(),
		mustRepositoryCriteria(t),
		20,
	)
	if err != nil {
		t.Fatalf("SearchRepositories() error = %v", err)
	}
	if len(repositorySearch.Candidates) != 1 ||
		repositorySearch.Candidates[0].License != "MIT" {
		t.Fatalf("SearchRepositories() = %+v", repositorySearch)
	}
	enrichment, err := client.EnrichRepositories(
		context.Background(),
		[]repository.Summary{repositorySearch.Candidates[0].Repository},
	)
	if err != nil {
		t.Fatalf("EnrichRepositories() error = %v", err)
	}
	if !enrichment.Items["octocat/typed-service"].READMEAvailable ||
		!enrichment.Items["octocat/typed-service"].ContributingAvailable {
		t.Fatalf("EnrichRepositories() = %+v", enrichment)
	}

	detail, err := client.GetIssueDetail(
		context.Background(),
		fixtureOwner,
		fixtureRepository,
		fixtureIssue,
	)
	if err != nil {
		t.Fatalf("GetIssueDetail() error = %v", err)
	}
	if detail.Candidate.Issue.Number != fixtureIssue ||
		len(detail.RepositorySignals) != 5 ||
		detail.Activity.CI != issue.CIStateSuccess ||
		len(detail.Dependencies) != 3 {
		t.Fatalf("GetIssueDetail() = %+v", detail)
	}
}

func TestClientSupportsEmptyAndFailureScenarios(t *testing.T) {
	client := New(func() time.Time { return fixtureNow })

	empty, err := client.SearchIssues(
		context.Background(),
		mustCriteria(t, emptyUsername),
		20,
	)
	if err != nil {
		t.Fatalf("empty SearchIssues() error = %v", err)
	}
	if empty.Candidates == nil || len(empty.Candidates) != 0 ||
		empty.TotalCount != 0 {
		t.Fatalf("empty SearchIssues() = %+v", empty)
	}

	tests := []struct {
		name     string
		username string
		kind     port.GitHubErrorKind
	}{
		{
			name:     "missing profile",
			username: missingUsername,
			kind:     port.GitHubErrorNotFound,
		},
		{
			name:     "rate limited profile",
			username: limitedUsername,
			kind:     port.GitHubErrorRateLimited,
		},
		{
			name:     "unknown profile",
			username: "unknown-user",
			kind:     port.GitHubErrorNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, scenarioErr := client.GetUser(
				context.Background(),
				user.Username(test.username),
			)
			if !port.IsGitHubError(scenarioErr, test.kind) {
				t.Fatalf(
					"GetUser() error = %v, want %s",
					scenarioErr,
					test.kind,
				)
			}
		})
	}

	_, err = client.GetIssueDetail(
		context.Background(),
		fixtureOwner,
		fixtureRepository,
		99,
	)
	if !port.IsGitHubError(err, port.GitHubErrorNotFound) {
		t.Fatalf("GetIssueDetail() error = %v", err)
	}
}

func TestClientHonorsCancellationAndReturnsFreshValues(t *testing.T) {
	client := New(func() time.Time { return fixtureNow })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetUser(ctx, user.Username(successUsername))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetUser() error = %v, want cancellation", err)
	}

	first, err := client.SearchIssues(
		context.Background(),
		mustCriteria(t, successUsername),
		20,
	)
	if err != nil {
		t.Fatalf("first SearchIssues() error = %v", err)
	}
	first.Candidates[0].Issue.Labels[0] = "mutated"
	second, err := client.SearchIssues(
		context.Background(),
		mustCriteria(t, successUsername),
		20,
	)
	if err != nil {
		t.Fatalf("second SearchIssues() error = %v", err)
	}
	if second.Candidates[0].Issue.Labels[0] != "good first issue" {
		t.Fatalf("fixture state leaked: %+v", second.Candidates[0].Issue.Labels)
	}
}

func mustCriteria(t *testing.T, username string) issue.SearchCriteria {
	t.Helper()
	criteria, err := issue.NewSearchCriteria(issue.SearchCriteriaOptions{
		Username: username,
	})
	if err != nil {
		t.Fatalf("NewSearchCriteria() error = %v", err)
	}
	return criteria
}

func mustRepositoryCriteria(t *testing.T) repository.DiscoveryCriteria {
	t.Helper()
	criteria, err := repository.NewDiscoveryCriteria(
		repository.DiscoveryCriteriaOptions{},
	)
	if err != nil {
		t.Fatalf("NewDiscoveryCriteria() error = %v", err)
	}
	return criteria
}
