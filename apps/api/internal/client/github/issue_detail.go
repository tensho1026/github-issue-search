package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	issueDetailSampleSize       = 50
	issueDetailCommentSize      = 50
	issueDetailMetricWindowDays = 180
	maxIssueDetailResponseBytes = 8 << 20

	graphQLIssueDetailDocument = `query IssueScoutIssueDetail(
  $owner: String!
  $name: String!
  $number: Int!
  $sampleSize: Int!
  $commentSize: Int!
) {
  repository(owner: $owner, name: $name) {
    databaseId
    name
    nameWithOwner
    url
    owner { login }
    description
    stargazerCount
    forkCount
    isFork
    isArchived
    updatedAt
    pushedAt
    primaryLanguage { name }
    defaultBranchRef {
      name
      target {
        ... on Commit {
          committedDate
          statusCheckRollup { state }
          history(first: $sampleSize) {
            totalCount
            pageInfo { hasNextPage }
            nodes {
              committedDate
              author { user { login } }
            }
          }
        }
      }
    }
    issues(first: $sampleSize, orderBy: {field: UPDATED_AT, direction: DESC}) {
      totalCount
      pageInfo { hasNextPage }
      nodes {
        createdAt
        comments(first: 10) {
          totalCount
          pageInfo { hasNextPage }
          nodes {
            createdAt
            authorAssociation
            author { login __typename }
          }
        }
      }
    }
    pullRequests(first: $sampleSize, orderBy: {field: UPDATED_AT, direction: DESC}) {
      totalCount
      pageInfo { hasNextPage }
      nodes {
        createdAt
        updatedAt
        mergedAt
        isDraft
        reviews(first: 10) {
          totalCount
          pageInfo { hasNextPage }
          nodes {
            createdAt
            authorAssociation
            author { login __typename }
          }
        }
      }
    }
    issue(number: $number) {
      number
      title
      body
      url
      state
      locked
      createdAt
      updatedAt
      comments(first: $commentSize) {
        totalCount
        pageInfo { hasNextPage }
        nodes {
          body
          createdAt
          authorAssociation
          author { login __typename }
        }
      }
      author { login __typename }
      labels(first: 100) { nodes { name } }
      assignees(first: 10) { nodes { login } }
    }
    readmeRoot: object(expression: "HEAD:README.md") { __typename }
    readmePlain: object(expression: "HEAD:README") { __typename }
    readmeLower: object(expression: "HEAD:readme.md") { __typename }
    readmeRST: object(expression: "HEAD:README.rst") { __typename }
    readmeText: object(expression: "HEAD:README.txt") { __typename }
    readmeGitHub: object(expression: "HEAD:.github/README.md") { __typename }
    readmeDocs: object(expression: "HEAD:docs/README.md") { __typename }
    contributingRoot: object(expression: "HEAD:CONTRIBUTING.md") { __typename }
    contributingPlain: object(expression: "HEAD:CONTRIBUTING") { __typename }
    contributingRST: object(expression: "HEAD:CONTRIBUTING.rst") { __typename }
    contributingGitHub: object(expression: "HEAD:.github/CONTRIBUTING.md") { __typename }
    contributingDocs: object(expression: "HEAD:docs/CONTRIBUTING.md") { __typename }
    conductRoot: object(expression: "HEAD:CODE_OF_CONDUCT.md") { __typename }
    conductGitHub: object(expression: "HEAD:.github/CODE_OF_CONDUCT.md") { __typename }
    conductDocs: object(expression: "HEAD:docs/CODE_OF_CONDUCT.md") { __typename }
    workflows: object(expression: "HEAD:.github/workflows") { __typename }
    testsRoot: object(expression: "HEAD:tests") { __typename }
    testRoot: object(expression: "HEAD:test") { __typename }
    specsRoot: object(expression: "HEAD:spec") { __typename }
    packageManifest: object(expression: "HEAD:package.json") {
      __typename
      ... on Blob { byteSize isBinary text }
    }
    goManifest: object(expression: "HEAD:go.mod") {
      __typename
      ... on Blob { byteSize isBinary text }
    }
  }
  rateLimit { limit remaining resetAt }
}`
)

// GetIssueDetail retrieves a single bounded GraphQL snapshot. Repository
// history connections never paginate beyond their documented first window.
func (c *Client) GetIssueDetail(
	ctx context.Context,
	owner string,
	repositoryName string,
	issueNumber int,
) (port.GitHubIssueDetailResult, error) {
	reference, err := issue.NewReference(owner, repositoryName, issueNumber)
	if err != nil {
		return port.GitHubIssueDetailResult{}, err
	}
	requestBody, err := json.Marshal(graphQLIssueDetailRequest{
		Query: graphQLIssueDetailDocument,
		Variables: graphQLIssueDetailVariables{
			Owner:       reference.Owner(),
			Name:        reference.RepositoryName(),
			Number:      reference.Number(),
			SampleSize:  issueDetailSampleSize,
			CommentSize: issueDetailCommentSize,
		},
	})
	if err != nil {
		return port.GitHubIssueDetailResult{}, upstreamDecodeError(
			"GitHub GraphQL issue detail request",
			err,
		)
	}

	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "graphql")
	endpoint.RawQuery = ""
	response, err := c.doRequest(ctx, func() (*http.Request, error) {
		request, requestErr := c.newRequest(
			ctx,
			http.MethodPost,
			endpoint.String(),
			bytes.NewReader(requestBody),
		)
		if requestErr != nil {
			return nil, requestErr
		}
		request.Header.Set("Content-Type", "application/json")
		return request, nil
	})
	if err != nil {
		return port.GitHubIssueDetailResult{}, err
	}
	defer response.Body.Close()

	headerRateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, headerRateLimit); statusErr != nil {
		return port.GitHubIssueDetailResult{}, statusErr
	}
	payload, err := decodeGraphQLIssueDetailResponse(response.Body)
	if err != nil {
		return port.GitHubIssueDetailResult{}, err
	}
	rateLimit, err := normalizeGraphQLRateLimit(
		payload.Data.RateLimit,
		headerRateLimit,
	)
	if err != nil {
		return port.GitHubIssueDetailResult{}, err
	}
	result, err := normalizeGraphQLIssueDetail(payload, rateLimit, c.now())
	if err != nil {
		return port.GitHubIssueDetailResult{}, err
	}
	c.logger.Debug(
		"GitHub GraphQL issue detail response received",
		"status", response.StatusCode,
		"repository", result.Candidate.Repository.FullName,
		"issueNumber", result.Candidate.Issue.Number,
		"incomplete", result.Incomplete,
		"rateLimitKnown", result.RateLimit.Known,
		"rateLimitRemaining", result.RateLimit.Remaining,
	)
	return result, nil
}

func decodeGraphQLIssueDetailResponse(
	body io.Reader,
) (graphQLIssueDetailEnvelope, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxIssueDetailResponseBytes+1))
	if err != nil {
		return graphQLIssueDetailEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			err,
		)
	}
	if len(raw) > maxIssueDetailResponseBytes {
		return graphQLIssueDetailEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			fmt.Errorf("exceeds %d bytes", maxIssueDetailResponseBytes),
		)
	}
	var payload graphQLIssueDetailEnvelope
	if err := json.Unmarshal(raw, &payload); err != nil {
		return graphQLIssueDetailEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			err,
		)
	}
	return payload, nil
}

func normalizeGraphQLIssueDetail(
	payload graphQLIssueDetailEnvelope,
	rateLimit port.RateLimit,
	now time.Time,
) (port.GitHubIssueDetailResult, error) {
	if payload.Data.Repository == nil {
		if isGraphQLNotFound(payload.Errors) {
			return port.GitHubIssueDetailResult{}, &port.GitHubError{
				Kind: port.GitHubErrorNotFound,
			}
		}
		if len(payload.Errors) > 0 {
			return port.GitHubIssueDetailResult{}, graphQLIssueSearchError(
				payload.Errors,
				rateLimit,
			)
		}
		return port.GitHubIssueDetailResult{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			errors.New("does not contain repository data"),
		)
	}
	repository := payload.Data.Repository
	if repository.Issue == nil {
		return port.GitHubIssueDetailResult{}, &port.GitHubError{
			Kind: port.GitHubErrorNotFound,
		}
	}
	repositorySummary := repository.Repository.toDomain()
	if strings.TrimSpace(repositorySummary.Owner) == "" ||
		strings.TrimSpace(repositorySummary.Name) == "" ||
		strings.TrimSpace(repositorySummary.FullName) == "" ||
		repositorySummary.Stars < 0 ||
		repositorySummary.Forks < 0 ||
		repositorySummary.OpenIssues < 0 ||
		repositorySummary.UpdatedAt.IsZero() {
		return port.GitHubIssueDetailResult{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			errors.New("contains invalid repository fields"),
		)
	}
	if err := validateAbsoluteHTTPURL(repositorySummary.URL); err != nil {
		return port.GitHubIssueDetailResult{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			fmt.Errorf("repository URL: %w", err),
		)
	}
	issueSummary, err := repository.Issue.toDomain()
	if err != nil {
		return port.GitHubIssueDetailResult{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			err,
		)
	}
	if err := validateDetailCollections(*repository); err != nil {
		return port.GitHubIssueDetailResult{}, upstreamDecodeError(
			"GitHub GraphQL issue detail response",
			err,
		)
	}
	incomplete := len(payload.Errors) > 0
	signals := repository.repositorySignals(incomplete)
	activity := repository.activity(now)
	return port.GitHubIssueDetailResult{
		Candidate: issue.Candidate{
			Repository: repositorySummary,
			Issue:      issueSummary,
		},
		Dependencies:      repository.manifestDependencies(),
		RepositorySignals: signals,
		Activity:          activity,
		Comments:          repository.Issue.commentObservations(),
		CommentsTruncated: repository.Issue.Comments.PageInfo.HasNextPage ||
			repository.Issue.Comments.TotalCount >
				len(repository.Issue.Comments.Nodes),
		RateLimit:  rateLimit,
		Incomplete: incomplete,
	}, nil
}

func validateDetailCollections(
	repository graphQLIssueDetailRepository,
) error {
	if repository.Issues.TotalCount < len(repository.Issues.Nodes) ||
		len(repository.Issues.Nodes) > issueDetailSampleSize ||
		repository.PullRequests.TotalCount <
			len(repository.PullRequests.Nodes) ||
		len(repository.PullRequests.Nodes) > issueDetailSampleSize {
		return errors.New("contains invalid repository sample counts")
	}
	if repository.DefaultBranch != nil &&
		repository.DefaultBranch.Target != nil {
		history := repository.DefaultBranch.Target.History
		if history.TotalCount < len(history.Nodes) ||
			len(history.Nodes) > issueDetailSampleSize {
			return errors.New("contains invalid commit sample counts")
		}
	}
	for _, issueNode := range repository.Issues.Nodes {
		if issueNode.CreatedAt.IsZero() ||
			issueNode.Comments.TotalCount < len(issueNode.Comments.Nodes) ||
			len(issueNode.Comments.Nodes) > 10 {
			return errors.New("contains invalid issue-response sample")
		}
	}
	for _, pullRequest := range repository.PullRequests.Nodes {
		if pullRequest.CreatedAt.IsZero() ||
			pullRequest.UpdatedAt.IsZero() ||
			pullRequest.Reviews.TotalCount < len(pullRequest.Reviews.Nodes) ||
			len(pullRequest.Reviews.Nodes) > 10 {
			return errors.New("contains invalid pull-request sample")
		}
		if pullRequest.MergedAt != nil &&
			pullRequest.MergedAt.Before(pullRequest.CreatedAt) {
			return errors.New("contains invalid pull-request merge time")
		}
	}
	return nil
}

func isGraphQLNotFound(graphQLErrors []graphQLError) bool {
	for _, graphQLError := range graphQLErrors {
		classification := strings.ToUpper(strings.Join([]string{
			graphQLError.Type,
			graphQLError.Extensions.Code,
			graphQLError.Message,
		}, " "))
		if strings.Contains(classification, "NOT_FOUND") ||
			strings.Contains(classification, "COULD NOT RESOLVE") {
			return true
		}
	}
	return false
}

type graphQLIssueDetailRequest struct {
	Query     string                      `json:"query"`
	Variables graphQLIssueDetailVariables `json:"variables"`
}

type graphQLIssueDetailVariables struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Number      int    `json:"number"`
	SampleSize  int    `json:"sampleSize"`
	CommentSize int    `json:"commentSize"`
}

type graphQLIssueDetailEnvelope struct {
	Data struct {
		Repository *graphQLIssueDetailRepository `json:"repository"`
		RateLimit  *graphQLRateLimit             `json:"rateLimit"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLIssueDetailRepository struct {
	Repository         graphQLIssueRepository    `json:"-"`
	Issue              *graphQLDetailIssue       `json:"issue"`
	DefaultBranch      *graphQLDetailBranch      `json:"defaultBranchRef"`
	Issues             graphQLDetailIssueHistory `json:"issues"`
	PullRequests       graphQLPullRequestHistory `json:"pullRequests"`
	ReadmeRoot         *graphQLGitObject         `json:"readmeRoot"`
	ReadmePlain        *graphQLGitObject         `json:"readmePlain"`
	ReadmeLower        *graphQLGitObject         `json:"readmeLower"`
	ReadmeRST          *graphQLGitObject         `json:"readmeRST"`
	ReadmeText         *graphQLGitObject         `json:"readmeText"`
	ReadmeGitHub       *graphQLGitObject         `json:"readmeGitHub"`
	ReadmeDocs         *graphQLGitObject         `json:"readmeDocs"`
	ContributingRoot   *graphQLGitObject         `json:"contributingRoot"`
	ContributingPlain  *graphQLGitObject         `json:"contributingPlain"`
	ContributingRST    *graphQLGitObject         `json:"contributingRST"`
	ContributingGitHub *graphQLGitObject         `json:"contributingGitHub"`
	ContributingDocs   *graphQLGitObject         `json:"contributingDocs"`
	ConductRoot        *graphQLGitObject         `json:"conductRoot"`
	ConductGitHub      *graphQLGitObject         `json:"conductGitHub"`
	ConductDocs        *graphQLGitObject         `json:"conductDocs"`
	Workflows          *graphQLGitObject         `json:"workflows"`
	TestsRoot          *graphQLGitObject         `json:"testsRoot"`
	TestRoot           *graphQLGitObject         `json:"testRoot"`
	SpecsRoot          *graphQLGitObject         `json:"specsRoot"`
	PackageManifest    *graphQLBlob              `json:"packageManifest"`
	GoManifest         *graphQLBlob              `json:"goManifest"`
}

func (repository *graphQLIssueDetailRepository) UnmarshalJSON(
	data []byte,
) error {
	type detailAlias graphQLIssueDetailRepository
	var detail detailAlias
	if err := json.Unmarshal(data, &detail); err != nil {
		return err
	}
	var summary graphQLIssueRepository
	if err := json.Unmarshal(data, &summary); err != nil {
		return err
	}
	*repository = graphQLIssueDetailRepository(detail)
	repository.Repository = summary
	return nil
}

type graphQLGitObject struct {
	TypeName string `json:"__typename"`
}

type graphQLBlob struct {
	TypeName string  `json:"__typename"`
	ByteSize int     `json:"byteSize"`
	IsBinary bool    `json:"isBinary"`
	Text     *string `json:"text"`
}

type graphQLDetailBranch struct {
	Name   string               `json:"name"`
	Target *graphQLDetailCommit `json:"target"`
}

type graphQLDetailCommit struct {
	CommittedAt time.Time                 `json:"committedDate"`
	CheckRollup *graphQLStatusCheckRollup `json:"statusCheckRollup"`
	History     graphQLCommitHistory      `json:"history"`
}

type graphQLStatusCheckRollup struct {
	State string `json:"state"`
}

type graphQLCommitHistory struct {
	TotalCount int                   `json:"totalCount"`
	PageInfo   graphQLPageInfo       `json:"pageInfo"`
	Nodes      []graphQLCommitSample `json:"nodes"`
}

type graphQLCommitSample struct {
	CommittedAt time.Time          `json:"committedDate"`
	Author      graphQLCommitActor `json:"author"`
}

type graphQLCommitActor struct {
	User *struct {
		Login string `json:"login"`
	} `json:"user"`
}

type graphQLDetailIssueHistory struct {
	TotalCount int                       `json:"totalCount"`
	PageInfo   graphQLPageInfo           `json:"pageInfo"`
	Nodes      []graphQLIssueHistoryNode `json:"nodes"`
}

type graphQLIssueHistoryNode struct {
	CreatedAt time.Time                      `json:"createdAt"`
	Comments  graphQLMaintainerCommentWindow `json:"comments"`
}

type graphQLMaintainerCommentWindow struct {
	TotalCount int                        `json:"totalCount"`
	PageInfo   graphQLPageInfo            `json:"pageInfo"`
	Nodes      []graphQLMaintainerComment `json:"nodes"`
}

type graphQLMaintainerComment struct {
	CreatedAt         time.Time     `json:"createdAt"`
	AuthorAssociation string        `json:"authorAssociation"`
	Author            *graphQLActor `json:"author"`
}

type graphQLPullRequestHistory struct {
	TotalCount int                      `json:"totalCount"`
	PageInfo   graphQLPageInfo          `json:"pageInfo"`
	Nodes      []graphQLPullRequestNode `json:"nodes"`
}

type graphQLPullRequestNode struct {
	CreatedAt time.Time                     `json:"createdAt"`
	UpdatedAt time.Time                     `json:"updatedAt"`
	MergedAt  *time.Time                    `json:"mergedAt"`
	IsDraft   bool                          `json:"isDraft"`
	Reviews   graphQLMaintainerReviewWindow `json:"reviews"`
}

type graphQLMaintainerReviewWindow struct {
	TotalCount int                       `json:"totalCount"`
	PageInfo   graphQLPageInfo           `json:"pageInfo"`
	Nodes      []graphQLMaintainerReview `json:"nodes"`
}

type graphQLMaintainerReview struct {
	CreatedAt         time.Time     `json:"createdAt"`
	AuthorAssociation string        `json:"authorAssociation"`
	Author            *graphQLActor `json:"author"`
}

type graphQLDetailIssue struct {
	Number    int                        `json:"number"`
	Title     string                     `json:"title"`
	Body      *string                    `json:"body"`
	URL       string                     `json:"url"`
	State     string                     `json:"state"`
	Locked    bool                       `json:"locked"`
	CreatedAt time.Time                  `json:"createdAt"`
	UpdatedAt time.Time                  `json:"updatedAt"`
	Comments  graphQLDetailCommentWindow `json:"comments"`
	Author    *graphQLActor              `json:"author"`
	Labels    graphQLLabelConnection     `json:"labels"`
	Assignees graphQLAssigneeConnection  `json:"assignees"`
}

type graphQLDetailCommentWindow struct {
	TotalCount int                    `json:"totalCount"`
	PageInfo   graphQLPageInfo        `json:"pageInfo"`
	Nodes      []graphQLDetailComment `json:"nodes"`
}

type graphQLDetailComment struct {
	Body              string        `json:"body"`
	CreatedAt         time.Time     `json:"createdAt"`
	AuthorAssociation string        `json:"authorAssociation"`
	Author            *graphQLActor `json:"author"`
}

func (detail graphQLDetailIssue) toDomain() (issue.Summary, error) {
	if detail.Number < 1 ||
		strings.TrimSpace(detail.Title) == "" ||
		strings.TrimSpace(detail.State) == "" ||
		detail.CreatedAt.IsZero() ||
		detail.UpdatedAt.IsZero() ||
		detail.Comments.TotalCount < 0 ||
		detail.Comments.TotalCount < len(detail.Comments.Nodes) ||
		len(detail.Labels.Nodes) > 100 ||
		len(detail.Assignees.Nodes) > 10 {
		return issue.Summary{}, errors.New("contains invalid issue fields")
	}
	if err := validateAbsoluteHTTPURL(detail.URL); err != nil {
		return issue.Summary{}, fmt.Errorf("issue URL: %w", err)
	}
	labels := make([]string, 0, len(detail.Labels.Nodes))
	for _, label := range detail.Labels.Nodes {
		if strings.TrimSpace(label.Name) == "" {
			return issue.Summary{}, errors.New("contains invalid issue label")
		}
		labels = append(labels, label.Name)
	}
	assignees := make([]string, 0, len(detail.Assignees.Nodes))
	for _, assignee := range detail.Assignees.Nodes {
		if strings.TrimSpace(assignee.Login) == "" {
			return issue.Summary{}, errors.New("contains invalid issue assignee")
		}
		assignees = append(assignees, assignee.Login)
	}
	authorLogin := ""
	authorType := ""
	if detail.Author != nil {
		authorLogin = strings.TrimSpace(detail.Author.Login)
		authorType = strings.TrimSpace(detail.Author.TypeName)
		if authorLogin == "" || authorType == "" {
			return issue.Summary{}, errors.New("contains invalid issue author")
		}
	}
	return issue.Summary{
		Number:      detail.Number,
		Title:       detail.Title,
		Body:        stringValue(detail.Body),
		URL:         detail.URL,
		State:       strings.ToLower(detail.State),
		Labels:      labels,
		Assignees:   assignees,
		AuthorLogin: authorLogin,
		AuthorType:  authorType,
		Comments:    detail.Comments.TotalCount,
		Locked:      detail.Locked,
		CreatedAt:   detail.CreatedAt.UTC(),
		UpdatedAt:   detail.UpdatedAt.UTC(),
	}, nil
}

func (detail graphQLDetailIssue) commentObservations() []issue.CommentObservation {
	comments := make([]issue.CommentObservation, 0, len(detail.Comments.Nodes))
	for _, comment := range detail.Comments.Nodes {
		login := ""
		actorType := ""
		if comment.Author != nil {
			login = comment.Author.Login
			actorType = comment.Author.TypeName
		}
		comments = append(comments, issue.CommentObservation{
			AuthorLogin:       login,
			AuthorType:        actorType,
			AuthorAssociation: comment.AuthorAssociation,
			Body:              comment.Body,
			CreatedAt:         comment.CreatedAt.UTC(),
		})
	}
	return comments
}

func (repository graphQLIssueDetailRepository) repositorySignals(
	incomplete bool,
) []issue.RepositorySignal {
	state := func(objects ...*graphQLGitObject) issue.SignalState {
		for _, object := range objects {
			if object != nil {
				return issue.SignalPresent
			}
		}
		if incomplete {
			return issue.SignalUnknown
		}
		return issue.SignalAbsent
	}
	return []issue.RepositorySignal{
		repositorySignal(issue.RepositoryREADME, state(
			repository.ReadmeRoot,
			repository.ReadmePlain,
			repository.ReadmeLower,
			repository.ReadmeRST,
			repository.ReadmeText,
			repository.ReadmeGitHub,
			repository.ReadmeDocs,
		)),
		repositorySignal(issue.RepositoryContributing, state(
			repository.ContributingRoot,
			repository.ContributingPlain,
			repository.ContributingRST,
			repository.ContributingGitHub,
			repository.ContributingDocs,
		)),
		repositorySignal(issue.RepositoryCI, state(repository.Workflows)),
		repositorySignal(
			issue.RepositoryTests,
			repository.testSignalState(),
		),
		repositorySignal(issue.RepositoryCodeOfConduct, state(
			repository.ConductRoot,
			repository.ConductGitHub,
			repository.ConductDocs,
		)),
	}
}

func (repository graphQLIssueDetailRepository) testSignalState() issue.SignalState {
	for _, object := range []*graphQLGitObject{
		repository.TestsRoot,
		repository.TestRoot,
		repository.SpecsRoot,
	} {
		if object != nil {
			return issue.SignalPresent
		}
	}
	if packageManifestHasTests(repository.PackageManifest) {
		return issue.SignalPresent
	}
	// Test files can be colocated with source files (notably Go and Rust).
	// The bounded path probes are therefore insufficient proof of absence.
	return issue.SignalUnknown
}

func packageManifestHasTests(manifest *graphQLBlob) bool {
	payload, valid := decodePackageManifest(manifest)
	if !valid {
		return false
	}
	for name, command := range payload.Scripts {
		normalizedName := strings.ToLower(strings.TrimSpace(name))
		if (normalizedName == "test" ||
			strings.HasPrefix(normalizedName, "test:")) &&
			strings.TrimSpace(command) != "" {
			return true
		}
	}
	return false
}

type packageManifestPayload struct {
	Scripts              map[string]string `json:"scripts"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

func decodePackageManifest(
	manifest *graphQLBlob,
) (packageManifestPayload, bool) {
	text, valid := manifestText(manifest)
	if !valid {
		return packageManifestPayload{}, false
	}
	var payload packageManifestPayload
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return packageManifestPayload{}, false
	}
	return payload, true
}

func manifestText(manifest *graphQLBlob) (string, bool) {
	if manifest == nil ||
		manifest.TypeName != "Blob" ||
		manifest.IsBinary ||
		manifest.Text == nil ||
		manifest.ByteSize < 0 ||
		manifest.ByteSize > maxManifestBytes ||
		len(*manifest.Text) > maxManifestBytes {
		return "", false
	}
	return *manifest.Text, true
}

func (repository graphQLIssueDetailRepository) manifestDependencies() []string {
	found := make(map[string]struct{}, issue.MaximumAnalysisDependencies)
	if payload, valid := decodePackageManifest(
		repository.PackageManifest,
	); valid {
		for _, dependencies := range []map[string]string{
			payload.Dependencies,
			payload.DevDependencies,
			payload.PeerDependencies,
			payload.OptionalDependencies,
		} {
			for _, dependency := range sortedManifestDependencyKeys(
				dependencies,
			) {
				addManifestDependency(found, dependency)
			}
		}
	}
	if content, valid := manifestText(repository.GoManifest); valid {
		for _, dependency := range parseGoModDependencies(content) {
			addManifestDependency(found, dependency)
		}
	}
	dependencies := make([]string, 0, len(found))
	for dependency := range found {
		dependencies = append(dependencies, dependency)
	}
	slices.Sort(dependencies)
	if len(dependencies) > issue.MaximumAnalysisDependencies {
		dependencies = dependencies[:issue.MaximumAnalysisDependencies]
	}
	return dependencies
}

func sortedManifestDependencyKeys(dependencies map[string]string) []string {
	keys := make([]string, 0, len(dependencies))
	for dependency := range dependencies {
		keys = append(keys, dependency)
	}
	slices.Sort(keys)
	return keys
}

func addManifestDependency(found map[string]struct{}, raw string) {
	dependency := strings.TrimSpace(strings.ToLower(raw))
	if dependency == "" || len(dependency) > 256 {
		return
	}
	if _, exists := found[dependency]; !exists &&
		len(found) >= issue.MaximumAnalysisDependencies {
		return
	}
	found[dependency] = struct{}{}
}

func parseGoModDependencies(content string) []string {
	dependencies := make([]string, 0)
	inRequireBlock := false
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "require (":
			inRequireBlock = true
			continue
		case inRequireBlock && line == ")":
			inRequireBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		case !inRequireBlock:
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "//") {
			continue
		}
		dependencies = append(dependencies, fields[0])
		if len(dependencies) >= issue.MaximumAnalysisDependencies {
			break
		}
	}
	return dependencies
}

func repositorySignal(
	key issue.RepositorySignalKey,
	state issue.SignalState,
) issue.RepositorySignal {
	return issue.RepositorySignal{
		Key:   key,
		State: state,
		Evidence: []issue.Evidence{{
			RuleID:      "repository.signal." + string(key),
			Source:      issue.EvidenceDerived,
			Description: "bounded conventional repository path inspection",
		}},
	}
}

func (repository graphQLIssueDetailRepository) activity(
	now time.Time,
) issue.ActivityMetrics {
	windowStart := now.UTC().
		AddDate(0, 0, -issueDetailMetricWindowDays)
	activity := issue.ActivityMetrics{
		LastMeaningfulUpdate: repository.Repository.UpdatedAt.UTC(),
		CI:                   issue.CIStateUnknown,
	}
	if repository.Repository.PushedAt != nil {
		activity.LastMeaningfulUpdate = maxTime(
			activity.LastMeaningfulUpdate,
			*repository.Repository.PushedAt,
		)
	}
	if repository.DefaultBranch != nil &&
		repository.DefaultBranch.Target != nil {
		commit := repository.DefaultBranch.Target
		activity.LastMeaningfulUpdate = maxTime(
			activity.LastMeaningfulUpdate,
			commit.CommittedAt,
		)
		activity.CI = normalizeCIState(commit.CheckRollup)
		activity.Contributors = contributorAggregate(
			commit.History,
			windowStart,
		)
	}
	activity.PullRequestsOpened,
		activity.PullRequestMerge,
		activity.PullRequestReview,
		activity.PullRequestMergeTime,
		activity.StaleOpenPullRequests = repository.PullRequests.aggregates(
		windowStart,
		now.UTC(),
	)
	activity.IssueResponse,
		activity.UnansweredIssues = repository.Issues.responseAggregates(
		windowStart,
		now.UTC(),
	)
	return activity
}

func normalizeCIState(rollup *graphQLStatusCheckRollup) issue.CIState {
	if rollup == nil {
		return issue.CIStateUnknown
	}
	switch strings.ToUpper(rollup.State) {
	case "SUCCESS":
		return issue.CIStateSuccess
	case "FAILURE", "ERROR":
		return issue.CIStateFailure
	case "PENDING", "EXPECTED":
		return issue.CIStatePending
	default:
		return issue.CIStateUnknown
	}
}

func contributorAggregate(
	history graphQLCommitHistory,
	windowStart time.Time,
) issue.CountAggregate {
	seen := make(map[string]struct{})
	observed := 0
	for _, commit := range history.Nodes {
		if commit.CommittedAt.Before(windowStart) {
			continue
		}
		observed++
		if commit.Author.User == nil {
			continue
		}
		login := strings.ToLower(strings.TrimSpace(commit.Author.User.Login))
		if login != "" && !issue.IsBotIdentity(login, "") {
			seen[login] = struct{}{}
		}
	}
	return issue.SummarizeCount(
		len(seen),
		observed,
		issueDetailMetricWindowDays,
		history.PageInfo.HasNextPage || history.TotalCount > len(history.Nodes),
	)
}

func (history graphQLPullRequestHistory) aggregates(
	windowStart time.Time,
	now time.Time,
) (
	issue.CountAggregate,
	issue.RatioAggregate,
	issue.DurationAggregate,
	issue.DurationAggregate,
	issue.CountAggregate,
) {
	opened := 0
	merged := 0
	staleOpen := 0
	reviewDurations := make([]time.Duration, 0, len(history.Nodes))
	mergeDurations := make([]time.Duration, 0, len(history.Nodes))
	truncated := history.PageInfo.HasNextPage ||
		history.TotalCount > len(history.Nodes)
	for _, pullRequest := range history.Nodes {
		if pullRequest.IsDraft || pullRequest.CreatedAt.Before(windowStart) {
			continue
		}
		opened++
		if pullRequest.MergedAt != nil {
			merged++
			mergeDurations = append(
				mergeDurations,
				pullRequest.MergedAt.Sub(pullRequest.CreatedAt),
			)
		}
		if pullRequest.MergedAt == nil &&
			now.Sub(pullRequest.CreatedAt) > 60*24*time.Hour &&
			now.Sub(pullRequest.UpdatedAt) > 30*24*time.Hour {
			staleOpen++
		}
		if duration, found := firstMaintainerReview(
			pullRequest.CreatedAt,
			pullRequest.Reviews,
		); found {
			reviewDurations = append(reviewDurations, duration)
		}
		if pullRequest.Reviews.PageInfo.HasNextPage ||
			pullRequest.Reviews.TotalCount > len(pullRequest.Reviews.Nodes) {
			truncated = true
		}
	}
	return issue.SummarizeCount(
			opened,
			len(history.Nodes),
			issueDetailMetricWindowDays,
			truncated,
		),
		issue.SummarizeRatio(
			merged,
			opened,
			issueDetailMetricWindowDays,
			truncated,
		),
		issue.SummarizeDurations(
			reviewDurations,
			issueDetailMetricWindowDays,
			truncated,
		),
		issue.SummarizeDurations(
			mergeDurations,
			issueDetailMetricWindowDays,
			truncated,
		),
		issue.SummarizeCount(
			staleOpen,
			opened,
			issueDetailMetricWindowDays,
			truncated,
		)
}

func firstMaintainerReview(
	createdAt time.Time,
	reviews graphQLMaintainerReviewWindow,
) (time.Duration, bool) {
	times := make([]time.Time, 0, len(reviews.Nodes))
	for _, review := range reviews.Nodes {
		if !isMaintainerAssociation(review.AuthorAssociation) ||
			review.Author == nil ||
			issue.IsBotIdentity(review.Author.Login, review.Author.TypeName) ||
			review.CreatedAt.Before(createdAt) {
			continue
		}
		times = append(times, review.CreatedAt)
	}
	if len(times) == 0 {
		return 0, false
	}
	slices.SortFunc(times, func(left, right time.Time) int {
		return left.Compare(right)
	})
	return times[0].Sub(createdAt), true
}

func (history graphQLDetailIssueHistory) responseAggregates(
	windowStart time.Time,
	now time.Time,
) (issue.DurationAggregate, issue.CountAggregate) {
	durations := make([]time.Duration, 0, len(history.Nodes))
	observed := 0
	unanswered := 0
	truncated := history.PageInfo.HasNextPage ||
		history.TotalCount > len(history.Nodes)
	for _, issueNode := range history.Nodes {
		if issueNode.CreatedAt.Before(windowStart) {
			continue
		}
		observed++
		if duration, found := firstMaintainerComment(
			issueNode.CreatedAt,
			issueNode.Comments,
		); found {
			durations = append(durations, duration)
		} else if now.Sub(issueNode.CreatedAt) > 14*24*time.Hour {
			unanswered++
		}
		if issueNode.Comments.PageInfo.HasNextPage ||
			issueNode.Comments.TotalCount > len(issueNode.Comments.Nodes) {
			truncated = true
		}
	}
	return issue.SummarizeDurations(
			durations,
			issueDetailMetricWindowDays,
			truncated,
		),
		issue.SummarizeCount(
			unanswered,
			observed,
			issueDetailMetricWindowDays,
			truncated,
		)
}

func firstMaintainerComment(
	createdAt time.Time,
	comments graphQLMaintainerCommentWindow,
) (time.Duration, bool) {
	times := make([]time.Time, 0, len(comments.Nodes))
	for _, comment := range comments.Nodes {
		if !isMaintainerAssociation(comment.AuthorAssociation) ||
			comment.Author == nil ||
			issue.IsBotIdentity(comment.Author.Login, comment.Author.TypeName) ||
			comment.CreatedAt.Before(createdAt) {
			continue
		}
		times = append(times, comment.CreatedAt)
	}
	if len(times) == 0 {
		return 0, false
	}
	slices.SortFunc(times, func(left, right time.Time) int {
		return left.Compare(right)
	})
	return times[0].Sub(createdAt), true
}

func isMaintainerAssociation(association string) bool {
	switch strings.ToUpper(strings.TrimSpace(association)) {
	case "OWNER", "MEMBER", "COLLABORATOR":
		return true
	default:
		return false
	}
}

func maxTime(left, right time.Time) time.Time {
	if right.After(left) {
		return right.UTC()
	}
	return left.UTC()
}

var _ port.GitHubIssueDetailReader = (*Client)(nil)
