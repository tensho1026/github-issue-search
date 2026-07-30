// Package githubmock provides the deterministic, network-free GitHub adapter
// used only by test environments and built-stack end-to-end validation.
package githubmock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	successUsername   = "octocat"
	emptyUsername     = "no-results"
	missingUsername   = "missing-user"
	limitedUsername   = "rate-limited"
	fixtureOwner      = "octocat"
	fixtureRepository = "typed-service"
	fixtureIssue      = 42
)

// Client returns fresh domain values for a small, explicit scenario catalog.
// It never opens a socket, reads a secret, or falls back to the live adapter.
type Client struct {
	now func() time.Time
}

// New constructs a deterministic adapter. The injected clock keeps fixture
// recency stable without hard-coding dates that eventually become stale.
func New(now func() time.Time) *Client {
	if now == nil {
		now = time.Now
	}
	return &Client{now: now}
}

// GetUser returns a public test profile or one explicit error scenario.
func (client *Client) GetUser(
	ctx context.Context,
	username user.Username,
) (port.GitHubUserResult, error) {
	if err := ctx.Err(); err != nil {
		return port.GitHubUserResult{}, err
	}
	if err := scenarioError(username.String(), client.now()); err != nil {
		return port.GitHubUserResult{}, err
	}
	if !isFixtureProfile(username.String()) {
		return port.GitHubUserResult{}, notFound()
	}

	profileName := "The Octocat"
	if username.String() == emptyUsername {
		profileName = "No Results"
	}
	return port.GitHubUserResult{
		Profile: user.Profile{
			Login:       username,
			Name:        profileName,
			AvatarURL:   "https://avatars.githubusercontent.com/u/1?v=4",
			Bio:         "Builds accessible, typed developer tools.",
			PublicRepos: 1,
			Followers:   1250,
			Following:   42,
		},
		RateLimit: client.rateLimit(),
	}, nil
}

// ListRepositories returns one bounded repository for supported profiles.
func (client *Client) ListRepositories(
	ctx context.Context,
	username user.Username,
	limit int,
) ([]repository.Summary, port.RateLimit, error) {
	if err := ctx.Err(); err != nil {
		return nil, port.RateLimit{}, err
	}
	if err := scenarioError(username.String(), client.now()); err != nil {
		return nil, port.RateLimit{}, err
	}
	if !isFixtureProfile(username.String()) {
		return nil, port.RateLimit{}, notFound()
	}
	if limit <= 0 {
		return []repository.Summary{}, client.rateLimit(), nil
	}

	item := client.repository(username.String())
	return []repository.Summary{item}, client.rateLimit(), nil
}

// GetProfileAnalysis returns one fresh, public-only extended profile snapshot.
func (client *Client) GetProfileAnalysis(
	ctx context.Context,
	username user.Username,
	repositoryLimit int,
	manifestLimit int,
) (port.GitHubProfileAnalysisResult, error) {
	if err := ctx.Err(); err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	if err := scenarioError(username.String(), client.now()); err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	if !isFixtureProfile(username.String()) {
		return port.GitHubProfileAnalysisResult{}, notFound()
	}
	if repositoryLimit < 1 || repositoryLimit > 20 ||
		manifestLimit < 1 || manifestLimit > 10 {
		return port.GitHubProfileAnalysisResult{}, fmt.Errorf(
			"mock profile analysis limits are invalid",
		)
	}

	now := client.now().UTC()
	emptyCollection := profile.RepositoryCollection{
		Available:  true,
		TotalKnown: true,
		Limit:      repositoryLimit,
	}
	snapshot := profile.ProfileSnapshot{
		Username:    username,
		WindowFrom:  now.AddDate(0, 0, -profile.AnalysisWindowDays),
		WindowTo:    now,
		Owned:       emptyCollection,
		Contributed: emptyCollection,
		Forked:      emptyCollection,
		Starred: profile.RepositoryCollection{
			Available: true,
			Limit:     repositoryLimit,
		},
		Contributions: profile.ContributionSnapshot{
			Available: true,
			Commits: profile.CountEvidence{
				Available: true,
			},
			IssuesOpened: profile.CountEvidence{
				Available: true,
				Complete:  true,
			},
			PullRequestsOpened: profile.CountEvidence{
				Available: true,
				Complete:  true,
			},
			PullRequestReviews: profile.CountEvidence{
				Available: true,
			},
			RepositoriesTouched: profile.CountEvidence{
				Available: true,
			},
		},
	}
	if username.String() == successUsername {
		snapshot.Owned.Repositories = []profile.RepositoryObservation{{
			Repository: client.repository(successUsername),
			Languages: map[string]int64{
				"TypeScript": 650,
				"Go":         350,
			},
			LanguagesComplete: true,
			Manifests: []profile.Manifest{{
				Path: "package.json",
				Content: []byte(
					`{"dependencies":{"react":"19.2.0","typescript":"6.0.0"}}`,
				),
			}},
		}}
		snapshot.Owned.Total = 1
		snapshot.Contributed.Repositories = []profile.RepositoryObservation{{
			Repository: client.profileRepository(
				"community",
				"accessible-tools",
				"TypeScript",
				false,
				24*time.Hour,
			),
		}}
		snapshot.Contributed.Total = 1
		snapshot.Forked.Repositories = []profile.RepositoryObservation{{
			Repository: client.profileRepository(
				successUsername,
				"go-tooling-fork",
				"Go",
				true,
				48*time.Hour,
			),
		}}
		snapshot.Forked.Total = 1
		snapshot.Starred.Repositories = []profile.RepositoryObservation{{
			Repository: client.profileRepository(
				"community",
				"rust-cli",
				"Rust",
				false,
				72*time.Hour,
			),
		}}
		snapshot.Contributions.Commits.Value = 18
		snapshot.Contributions.IssuesOpened.Value = 3
		snapshot.Contributions.PullRequestsOpened.Value = 7
		snapshot.Contributions.PullRequestReviews.Value = 4
		snapshot.Contributions.RepositoriesTouched.Value = 1
	}

	return port.GitHubProfileAnalysisResult{
		Snapshot:  snapshot,
		RateLimit: client.rateLimit(),
	}, nil
}

// SearchIssues returns an eligible candidate, an explicit empty result, or an
// explicit error without inspecting live GitHub state.
func (client *Client) SearchIssues(
	ctx context.Context,
	criteria issue.SearchCriteria,
	limit int,
) (port.GitHubIssueSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return port.GitHubIssueSearchResult{}, err
	}
	username := criteria.Username().String()
	if err := scenarioError(username, client.now()); err != nil {
		return port.GitHubIssueSearchResult{}, err
	}
	if username == emptyUsername {
		return port.GitHubIssueSearchResult{
			Candidates: []issue.Candidate{},
			RateLimit:  client.rateLimit(),
		}, nil
	}
	if username != successUsername {
		return port.GitHubIssueSearchResult{}, notFound()
	}
	if limit <= 0 {
		return port.GitHubIssueSearchResult{}, fmt.Errorf(
			"mock issue search limit must be positive",
		)
	}
	return port.GitHubIssueSearchResult{
		Candidates: []issue.Candidate{client.candidate()},
		TotalCount: 1,
		RateLimit:  client.rateLimit(),
	}, nil
}

// GetIssueDetail returns the complete bounded fixture used by the real detail
// handler, analyzer, recommendation usecase, and response mapper.
func (client *Client) GetIssueDetail(
	ctx context.Context,
	owner string,
	repositoryName string,
	issueNumber int,
) (port.GitHubIssueDetailResult, error) {
	if err := ctx.Err(); err != nil {
		return port.GitHubIssueDetailResult{}, err
	}
	if !isFixtureRepository(owner, repositoryName) ||
		issueNumber != fixtureIssue {
		return port.GitHubIssueDetailResult{}, notFound()
	}
	now := client.now().UTC()
	return port.GitHubIssueDetailResult{
		Candidate:    client.candidate(),
		Dependencies: []string{"react", "typescript", "vitest"},
		RepositorySignals: []issue.RepositorySignal{
			signal(issue.RepositoryREADME, issue.SignalPresent),
			signal(issue.RepositoryContributing, issue.SignalPresent),
			signal(issue.RepositoryCI, issue.SignalPresent),
			signal(issue.RepositoryTests, issue.SignalPresent),
			signal(issue.RepositoryCodeOfConduct, issue.SignalAbsent),
		},
		Activity: issue.ActivityMetrics{
			LastMeaningfulUpdate:  now.Add(-12 * time.Hour),
			CI:                    issue.CIStateSuccess,
			Contributors:          issue.SummarizeCount(8, 8, 180, false),
			PullRequestsOpened:    issue.SummarizeCount(20, 20, 180, false),
			StaleOpenPullRequests: issue.SummarizeCount(2, 7, 180, false),
			UnansweredIssues:      issue.SummarizeCount(3, 28, 180, false),
			PullRequestMerge:      issue.SummarizeRatio(15, 20, 180, false),
			IssueResponse: issue.SummarizeDurations(
				[]time.Duration{2 * time.Hour, 4 * time.Hour, 24 * time.Hour},
				180,
				false,
			),
			PullRequestReview: issue.SummarizeDurations(
				[]time.Duration{4 * time.Hour, 8 * time.Hour, 48 * time.Hour},
				180,
				false,
			),
			PullRequestMergeTime: issue.SummarizeDurations(
				[]time.Duration{48 * time.Hour, 72 * time.Hour, 120 * time.Hour},
				180,
				false,
			),
		},
		Comments: []issue.CommentObservation{{
			AuthorLogin:       "maintainer",
			AuthorType:        issue.AuthorHuman,
			AuthorAssociation: "MEMBER",
			Body:              "Thanks for the detailed report.",
			CreatedAt:         now.Add(-18 * time.Hour),
		}},
		RateLimit: client.rateLimit(),
	}, nil
}

func (client *Client) candidate() issue.Candidate {
	now := client.now().UTC()
	return issue.Candidate{
		Repository: client.repository(successUsername),
		Issue: issue.Summary{
			Number: fixtureIssue,
			Title:  "Improve keyboard navigation in the command palette",
			Body: `## Problem
The React command palette traps keyboard focus after closing.

## Expected behavior
Focus returns to the trigger and arrow-key navigation remains available.

## Implementation guidance
Update the TypeScript dialog component and its accessibility tests.

## Acceptance criteria
- Escape closes the palette and restores focus.
- Add Vitest coverage for keyboard navigation.
- Document how to verify the change.`,
			URL:         "https://github.com/octocat/typed-service/issues/42",
			State:       issue.StateOpen,
			Labels:      []string{"good first issue", "accessibility"},
			AuthorLogin: "maintainer",
			AuthorType:  issue.AuthorHuman,
			Comments:    1,
			CreatedAt:   now.Add(-14 * 24 * time.Hour),
			UpdatedAt:   now.Add(-18 * time.Hour),
		},
	}
}

func (client *Client) repository(owner string) repository.Summary {
	now := client.now().UTC()
	name := fixtureRepository
	if owner == emptyUsername {
		name = "empty-project"
	}
	return repository.Summary{
		ID:            1,
		Owner:         owner,
		Name:          name,
		FullName:      owner + "/" + name,
		Description:   "A typed service with accessible contributor workflows.",
		URL:           "https://github.com/" + owner + "/" + name,
		MainLanguage:  "TypeScript",
		Stars:         1250,
		Forks:         32,
		OpenIssues:    4,
		DefaultBranch: "main",
		UpdatedAt:     now.Add(-12 * time.Hour),
		PushedAt:      now.Add(-14 * time.Hour),
	}
}

func (client *Client) profileRepository(
	owner string,
	name string,
	language string,
	isFork bool,
	age time.Duration,
) repository.Summary {
	return repository.Summary{
		ID:            10,
		Owner:         owner,
		Name:          name,
		FullName:      owner + "/" + name,
		Description:   "Deterministic public profile evidence.",
		URL:           "https://github.com/" + owner + "/" + name,
		MainLanguage:  language,
		Stars:         100,
		Forks:         10,
		OpenIssues:    5,
		IsFork:        isFork,
		DefaultBranch: "main",
		UpdatedAt:     client.now().UTC().Add(-age),
		PushedAt:      client.now().UTC().Add(-age),
	}
}

func (client *Client) rateLimit() port.RateLimit {
	return port.RateLimit{
		Known:     true,
		Limit:     5000,
		Remaining: 4992,
		Reset:     client.now().UTC().Add(time.Hour),
	}
}

func scenarioError(username string, now time.Time) error {
	switch username {
	case missingUsername:
		return notFound()
	case limitedUsername:
		return &port.GitHubError{
			Kind:  port.GitHubErrorRateLimited,
			Reset: now.UTC().Add(time.Hour),
		}
	default:
		return nil
	}
}

func isFixtureProfile(username string) bool {
	return username == successUsername || username == emptyUsername
}

func isFixtureRepository(owner, name string) bool {
	return strings.EqualFold(owner, fixtureOwner) &&
		strings.EqualFold(name, fixtureRepository)
}

func signal(
	key issue.RepositorySignalKey,
	state issue.SignalState,
) issue.RepositorySignal {
	return issue.RepositorySignal{
		Key:   key,
		State: state,
		Evidence: []issue.Evidence{{
			RuleID:      "mock.repository." + string(key),
			Source:      issue.EvidenceDerived,
			Description: "deterministic mock repository inspection",
		}},
	}
}

func notFound() error {
	return &port.GitHubError{Kind: port.GitHubErrorNotFound}
}

var _ port.GitHubReader = (*Client)(nil)
