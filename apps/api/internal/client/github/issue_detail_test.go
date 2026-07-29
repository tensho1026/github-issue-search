package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestGetIssueDetailPostsBoundedGraphQLAndNormalizesResult(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/graphql" {
			t.Errorf("request = %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		var payload graphQLIssueDetailRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if payload.Variables.Owner != "acme" ||
			payload.Variables.Name != "rocket" ||
			payload.Variables.Number != 42 ||
			payload.Variables.SampleSize != issueDetailSampleSize ||
			payload.Variables.CommentSize != issueDetailCommentSize ||
			!strings.Contains(payload.Query, "history(first: $sampleSize)") {
			t.Errorf("GraphQL request = %+v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, issueDetailFixture(""))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "token")
	client.now = func() time.Time {
		return time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	}
	result, err := client.GetIssueDetail(
		context.Background(),
		"acme",
		"rocket",
		42,
	)
	if err != nil {
		t.Fatalf("GetIssueDetail() error = %v", err)
	}

	if requests.Load() != 1 ||
		result.Candidate.Repository.FullName != "acme/rocket" ||
		result.Candidate.Repository.MainLanguage != "Go" ||
		result.Candidate.Issue.Number != 42 ||
		result.Candidate.Issue.Comments != 2 ||
		result.RateLimit.Remaining != 4980 ||
		result.Incomplete {
		t.Fatalf("GetIssueDetail() result = %+v, requests = %d", result, requests.Load())
	}
	states := make(map[issue.RepositorySignalKey]issue.SignalState)
	for _, signal := range result.RepositorySignals {
		states[signal.Key] = signal.State
	}
	if states[issue.RepositoryREADME] != issue.SignalPresent ||
		states[issue.RepositoryContributing] != issue.SignalPresent ||
		states[issue.RepositoryCI] != issue.SignalPresent ||
		states[issue.RepositoryTests] != issue.SignalPresent ||
		states[issue.RepositoryCodeOfConduct] != issue.SignalAbsent {
		t.Fatalf("repository signals = %+v", states)
	}
	activity := result.Activity
	if activity.CI != issue.CIStateSuccess ||
		activity.Contributors.Status != issue.AggregateAvailable ||
		activity.Contributors.Value != 2 ||
		activity.PullRequestsOpened.Value != 2 ||
		activity.PullRequestMerge.Percentage != 50 ||
		activity.StaleOpenPullRequests.Value != 1 ||
		activity.UnansweredIssues.Value != 1 ||
		activity.IssueResponse.Median != 24*time.Hour ||
		activity.PullRequestReview.Median != 12*time.Hour ||
		activity.PullRequestMergeTime.Median != 48*time.Hour {
		t.Fatalf("activity = %+v", activity)
	}
	if claim := issue.DetectClaim(
		result.Comments,
		result.CommentsTruncated,
	); !claim.Claimed {
		t.Fatalf("claim = %+v", claim)
	}
}

func TestGetIssueDetailPreservesPartialFieldsAsUnknown(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = io.WriteString(writer, issueDetailFixture(`,
  "errors": [{
    "message": "Something went wrong while executing your query",
    "type": "INTERNAL",
    "extensions": {"code": "INTERNAL"}
  }]`))
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "token").GetIssueDetail(
		context.Background(),
		"acme",
		"rocket",
		42,
	)
	if err != nil {
		t.Fatalf("GetIssueDetail() error = %v", err)
	}
	if !result.Incomplete {
		t.Fatal("GetIssueDetail() incomplete = false")
	}
	for _, signal := range result.RepositorySignals {
		if signal.Key == issue.RepositoryCodeOfConduct &&
			signal.State != issue.SignalUnknown {
			t.Fatalf("code of conduct state = %q", signal.State)
		}
	}
}

func TestGetIssueDetailMapsNotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = io.WriteString(writer, `{
  "data": {"repository": null, "rateLimit": null},
  "errors": [{
    "message": "Could not resolve to a Repository",
    "type": "NOT_FOUND",
    "extensions": {"code": "NOT_FOUND"}
  }]
}`)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL, "token").GetIssueDetail(
		context.Background(),
		"missing",
		"repository",
		1,
	)
	if !port.IsGitHubError(err, port.GitHubErrorNotFound) {
		t.Fatalf("GetIssueDetail() error = %v", err)
	}
}

func TestGetIssueDetailRejectsInvalidIdentityBeforeRequest(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		requests.Add(1)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, "token")

	tests := []struct {
		owner  string
		name   string
		number int
	}{
		{owner: "../owner", name: "repo", number: 1},
		{owner: ".", name: "repo", number: 1},
		{owner: "owner", name: "repo/name", number: 1},
		{owner: "owner", name: "..", number: 1},
		{owner: "owner", name: "repo", number: 0},
	}
	for _, test := range tests {
		_, err := client.GetIssueDetail(
			context.Background(),
			test.owner,
			test.name,
			test.number,
		)
		if err == nil {
			t.Fatalf("GetIssueDetail(%q, %q, %d) error = nil", test.owner, test.name, test.number)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func TestGetIssueDetailRejectsMalformedAndOversizedPayloads(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: "{"},
		{
			name: "oversized",
			body: strings.Repeat("x", maxIssueDetailResponseBytes+1),
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				_ *http.Request,
			) {
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			_, err := newTestClient(t, server.URL, "token").GetIssueDetail(
				context.Background(),
				"owner",
				"repo",
				1,
			)
			var gitHubError *port.GitHubError
			if !errors.As(err, &gitHubError) ||
				gitHubError.Kind != port.GitHubErrorUpstream {
				t.Fatalf("GetIssueDetail() error = %v", err)
			}
		})
	}
}

func issueDetailFixture(suffix string) string {
	return `{
  "data": {
    "repository": {
      "databaseId": 7,
      "owner": {"login": "acme"},
      "name": "rocket",
      "nameWithOwner": "acme/rocket",
      "url": "https://github.com/acme/rocket",
      "description": "A rocket",
      "stargazerCount": 250,
      "forkCount": 20,
      "isFork": false,
      "isArchived": false,
      "updatedAt": "2026-07-29T00:00:00Z",
      "pushedAt": "2026-07-29T00:00:00Z",
      "primaryLanguage": {"name": "Go"},
      "defaultBranchRef": {
        "name": "main",
        "target": {
          "committedDate": "2026-07-29T12:00:00Z",
          "statusCheckRollup": {"state": "SUCCESS"},
          "history": {
            "totalCount": 3,
            "pageInfo": {"hasNextPage": false},
            "nodes": [
              {
                "committedDate": "2026-07-29T12:00:00Z",
                "author": {"user": {"login": "alice"}}
              },
              {
                "committedDate": "2026-07-28T12:00:00Z",
                "author": {"user": {"login": "bob"}}
              },
              {
                "committedDate": "2026-07-27T12:00:00Z",
                "author": {"user": {"login": "bot[bot]"}}
              }
            ]
          }
        }
      },
      "issues": {
        "totalCount": 2,
        "pageInfo": {"hasNextPage": false},
        "nodes": [
          {
            "createdAt": "2026-07-01T00:00:00Z",
            "comments": {
              "totalCount": 1,
              "pageInfo": {"hasNextPage": false},
              "nodes": [{
                "createdAt": "2026-07-02T00:00:00Z",
                "authorAssociation": "MEMBER",
                "author": {"login": "maintainer", "__typename": "User"}
              }]
            }
          },
          {
            "createdAt": "2026-07-05T00:00:00Z",
            "comments": {
              "totalCount": 0,
              "pageInfo": {"hasNextPage": false},
              "nodes": []
            }
          }
        ]
      },
      "pullRequests": {
        "totalCount": 3,
        "pageInfo": {"hasNextPage": false},
        "nodes": [
          {
            "createdAt": "2026-07-01T00:00:00Z",
            "updatedAt": "2026-07-03T00:00:00Z",
            "mergedAt": "2026-07-03T00:00:00Z",
            "isDraft": false,
            "reviews": {
              "totalCount": 1,
              "pageInfo": {"hasNextPage": false},
              "nodes": [{
                "createdAt": "2026-07-01T12:00:00Z",
                "authorAssociation": "COLLABORATOR",
                "author": {"login": "reviewer", "__typename": "User"}
              }]
            }
          },
          {
            "createdAt": "2026-05-01T00:00:00Z",
            "updatedAt": "2026-05-15T00:00:00Z",
            "mergedAt": null,
            "isDraft": false,
            "reviews": {
              "totalCount": 1,
              "pageInfo": {"hasNextPage": false},
              "nodes": [{
                "createdAt": "2026-05-01T12:00:00Z",
                "authorAssociation": "CONTRIBUTOR",
                "author": {"login": "reader", "__typename": "User"}
              }]
            }
          },
          {
            "createdAt": "2026-07-15T00:00:00Z",
            "updatedAt": "2026-07-16T00:00:00Z",
            "mergedAt": "2026-07-16T00:00:00Z",
            "isDraft": true,
            "reviews": {
              "totalCount": 0,
              "pageInfo": {"hasNextPage": false},
              "nodes": []
            }
          }
        ]
      },
      "issue": {
        "number": 42,
        "title": "Improve launch validation",
        "body": "The launch validation needs clear expected behavior, implementation guidance, tests, and acceptance criteria.",
        "url": "https://github.com/acme/rocket/issues/42",
        "state": "OPEN",
        "locked": false,
        "createdAt": "2026-07-20T00:00:00Z",
        "updatedAt": "2026-07-29T00:00:00Z",
        "comments": {
          "totalCount": 2,
          "pageInfo": {"hasNextPage": false},
          "nodes": [
            {
              "body": "Can I work on this?",
              "createdAt": "2026-07-21T00:00:00Z",
              "authorAssociation": "NONE",
              "author": {"login": "reader", "__typename": "User"}
            },
            {
              "body": "I'll start working on this tomorrow.",
              "createdAt": "2026-07-22T00:00:00Z",
              "authorAssociation": "CONTRIBUTOR",
              "author": {"login": "helper", "__typename": "User"}
            }
          ]
        },
        "author": {"login": "maintainer", "__typename": "User"},
        "labels": {"nodes": [{"name": "help wanted"}]},
        "assignees": {"nodes": []}
      },
      "readmeRoot": {"__typename": "Blob"},
      "readmeLower": null,
      "contributingRoot": {"__typename": "Blob"},
      "contributingGitHub": null,
      "conductRoot": null,
      "conductGitHub": null,
      "workflows": {"__typename": "Tree"},
      "testsRoot": {"__typename": "Tree"},
      "testRoot": null,
      "specsRoot": null
    },
    "rateLimit": {
      "limit": 5000,
      "remaining": 4980,
      "resetAt": "2026-07-30T01:00:00Z"
    }
  }` + suffix + `
}`
}
