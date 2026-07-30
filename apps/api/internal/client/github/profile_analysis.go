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
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	maxProfileAnalysisResponseBytes = 12 << 20
	maxProfileRepositoryLimit       = 20

	graphQLProfileAnalysisDocument = `query IssueScoutProfileAnalysis(
  $login: String!
  $repositoryLimit: Int!
  $windowFrom: DateTime!
  $windowTo: DateTime!
  $pullRequestQuery: String!
  $issueQuery: String!
) {
  repositoryOwner(login: $login) {
    __typename
    login
    ... on User {
      ownedRepositories: repositories(
        first: $repositoryLimit
        privacy: PUBLIC
        ownerAffiliations: [OWNER]
        isFork: false
        isArchived: false
        orderBy: { field: PUSHED_AT, direction: DESC }
      ) {
        totalCount
        pageInfo { hasNextPage }
        nodes { ...OwnedProfileRepository }
      }
      forkedRepositories: repositories(
        first: $repositoryLimit
        privacy: PUBLIC
        ownerAffiliations: [OWNER]
        isFork: true
        isArchived: false
        orderBy: { field: PUSHED_AT, direction: DESC }
      ) {
        totalCount
        pageInfo { hasNextPage }
        nodes { ...ProfileRepository }
      }
      contributedRepositories: repositoriesContributedTo(
        first: $repositoryLimit
        privacy: PUBLIC
        includeUserRepositories: false
        contributionTypes: [COMMIT, ISSUE, PULL_REQUEST, PULL_REQUEST_REVIEW]
        orderBy: { field: PUSHED_AT, direction: DESC }
      ) {
        totalCount
        pageInfo { hasNextPage }
        nodes { ...ProfileRepository }
      }
      starredRepositories(
        first: $repositoryLimit
        orderBy: { field: STARRED_AT, direction: DESC }
      ) {
        pageInfo { hasNextPage }
        nodes { ...ProfileRepository }
      }
      contributionsCollection(from: $windowFrom, to: $windowTo) {
      commitContributionsByRepository(maxRepositories: $repositoryLimit) {
        repository { id visibility }
        contributions { totalCount }
      }
      issueContributionsByRepository(maxRepositories: $repositoryLimit) {
        repository { id visibility }
        contributions { totalCount }
      }
      pullRequestContributionsByRepository(
        maxRepositories: $repositoryLimit
      ) {
        repository { id visibility }
        contributions { totalCount }
      }
      pullRequestReviewContributionsByRepository(
        maxRepositories: $repositoryLimit
      ) {
        repository { id visibility }
        contributions { totalCount }
        }
      }
    }
    ... on Organization {
      ownedRepositories: repositories(
        first: $repositoryLimit
        privacy: PUBLIC
        isFork: false
        isArchived: false
        orderBy: { field: PUSHED_AT, direction: DESC }
      ) {
        totalCount
        pageInfo { hasNextPage }
        nodes { ...OwnedProfileRepository }
      }
      forkedRepositories: repositories(
        first: $repositoryLimit
        privacy: PUBLIC
        isFork: true
        isArchived: false
        orderBy: { field: PUSHED_AT, direction: DESC }
      ) {
        totalCount
        pageInfo { hasNextPage }
        nodes { ...ProfileRepository }
      }
    }
  }
  authoredPullRequests: search(
    query: $pullRequestQuery
    type: ISSUE
    first: 1
  ) {
    issueCount
  }
  authoredIssues: search(query: $issueQuery, type: ISSUE, first: 1) {
    issueCount
  }
  rateLimit {
    limit
    remaining
    resetAt
  }
}

fragment ProfileRepository on Repository {
  databaseId
  owner { login }
  name
  nameWithOwner
  description
  url
  stargazerCount
  forkCount
  issues(states: OPEN) { totalCount }
  isFork
  isArchived
  defaultBranchRef { name }
  primaryLanguage { name }
  updatedAt
  pushedAt
  visibility
}

fragment OwnedProfileRepository on Repository {
  ...ProfileRepository
  languages(first: 10, orderBy: { field: SIZE, direction: DESC }) {
    totalSize
    edges {
      size
      node { name }
    }
  }
  packageManifest: object(expression: "HEAD:package.json") {
    ...ProfileManifest
  }
  goManifest: object(expression: "HEAD:go.mod") {
    ...ProfileManifest
  }
  mavenManifest: object(expression: "HEAD:pom.xml") {
    ...ProfileManifest
  }
  gradleManifest: object(expression: "HEAD:build.gradle") {
    ...ProfileManifest
  }
  pythonProject: object(expression: "HEAD:pyproject.toml") {
    ...ProfileManifest
  }
  pythonRequirements: object(expression: "HEAD:requirements.txt") {
    ...ProfileManifest
  }
  rustManifest: object(expression: "HEAD:Cargo.toml") {
    ...ProfileManifest
  }
  composerManifest: object(expression: "HEAD:composer.json") {
    ...ProfileManifest
  }
}

fragment ProfileManifest on Blob {
  __typename
  byteSize
  isBinary
  text
}`
)

// GetProfileAnalysis retrieves one bounded public GraphQL snapshot. The query
// filters repository connections to PUBLIC and discards non-public
// contribution or star nodes before they cross the adapter boundary.
func (c *Client) GetProfileAnalysis(
	ctx context.Context,
	username user.Username,
	repositoryLimit int,
	manifestLimit int,
) (port.GitHubProfileAnalysisResult, error) {
	if repositoryLimit < 1 || repositoryLimit > maxProfileRepositoryLimit {
		return port.GitHubProfileAnalysisResult{}, fmt.Errorf(
			"profile repository limit must be between 1 and %d",
			maxProfileRepositoryLimit,
		)
	}
	if manifestLimit < 1 || manifestLimit > 10 {
		return port.GitHubProfileAnalysisResult{}, errors.New(
			"profile manifest limit must be between 1 and 10",
		)
	}

	windowTo := c.now().UTC()
	windowFrom := windowTo.AddDate(0, 0, -profile.AnalysisWindowDays)
	variables := graphQLProfileAnalysisVariables{
		Login:           username.String(),
		RepositoryLimit: repositoryLimit,
		WindowFrom:      windowFrom,
		WindowTo:        windowTo,
		PullRequestQuery: publicAuthoredQuery(
			username,
			true,
			windowFrom,
			windowTo,
		),
		IssueQuery: publicAuthoredQuery(
			username,
			false,
			windowFrom,
			windowTo,
		),
	}
	requestPayload, err := json.Marshal(graphQLProfileAnalysisRequest{
		Query:     graphQLProfileAnalysisDocument,
		Variables: variables,
	})
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis request",
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
			bytes.NewReader(requestPayload),
		)
		if requestErr != nil {
			return nil, requestErr
		}
		request.Header.Set("Content-Type", "application/json")
		return request, nil
	})
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	defer response.Body.Close()

	headerRateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, headerRateLimit); statusErr != nil {
		return port.GitHubProfileAnalysisResult{}, statusErr
	}
	payload, err := decodeGraphQLProfileAnalysis(response.Body)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	rateLimit, err := normalizeGraphQLRateLimit(
		payload.Data.RateLimit,
		headerRateLimit,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	result, err := normalizeGraphQLProfileAnalysis(
		payload,
		username,
		repositoryLimit,
		manifestLimit,
		windowFrom,
		windowTo,
		rateLimit,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	c.logger.Debug(
		"GitHub GraphQL public profile analysis received",
		"status", response.StatusCode,
		"ownedObserved", len(result.Snapshot.Owned.Repositories),
		"contributedObserved", len(result.Snapshot.Contributed.Repositories),
		"starredObserved", len(result.Snapshot.Starred.Repositories),
		"forkedObserved", len(result.Snapshot.Forked.Repositories),
		"partial", len(result.Snapshot.Warnings) > 0,
		"rateLimitKnown", result.RateLimit.Known,
		"rateLimitRemaining", result.RateLimit.Remaining,
	)
	return result, nil
}

func publicAuthoredQuery(
	username user.Username,
	pullRequests bool,
	from time.Time,
	to time.Time,
) string {
	kind := "is:issue"
	if pullRequests {
		kind = "is:pr"
	}
	const searchTimestamp = "2006-01-02T15:04:05-07:00"
	return strings.Join([]string{
		kind,
		"is:public",
		"author:" + username.String(),
		"created:>=" + from.UTC().Format(searchTimestamp),
		"created:<=" + to.UTC().Format(searchTimestamp),
	}, " ")
}

func decodeGraphQLProfileAnalysis(
	body io.Reader,
) (graphQLProfileAnalysisEnvelope, error) {
	raw, err := io.ReadAll(io.LimitReader(
		body,
		maxProfileAnalysisResponseBytes+1,
	))
	if err != nil {
		return graphQLProfileAnalysisEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis response",
			err,
		)
	}
	if len(raw) > maxProfileAnalysisResponseBytes {
		return graphQLProfileAnalysisEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis response",
			fmt.Errorf("exceeds %d bytes", maxProfileAnalysisResponseBytes),
		)
	}
	var payload graphQLProfileAnalysisEnvelope
	if err := json.Unmarshal(raw, &payload); err != nil {
		return graphQLProfileAnalysisEnvelope{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis response",
			err,
		)
	}
	return payload, nil
}

func normalizeGraphQLProfileAnalysis(
	payload graphQLProfileAnalysisEnvelope,
	requestedUsername user.Username,
	repositoryLimit int,
	manifestLimit int,
	windowFrom time.Time,
	windowTo time.Time,
	rateLimit port.RateLimit,
) (port.GitHubProfileAnalysisResult, error) {
	if payload.Data.Owner == nil {
		if profileErrorsContainNotFound(payload.Errors) {
			return port.GitHubProfileAnalysisResult{}, &port.GitHubError{
				Kind:  port.GitHubErrorNotFound,
				Cause: errors.New("GitHub GraphQL profile user was not found"),
			}
		}
		if len(payload.Errors) > 0 {
			return port.GitHubProfileAnalysisResult{}, graphQLIssueSearchError(
				payload.Errors,
				rateLimit,
			)
		}
		return port.GitHubProfileAnalysisResult{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis response",
			errors.New("does not contain user data"),
		)
	}
	normalizedUsername, err := user.ParseUsername(payload.Data.Owner.Login)
	if err != nil ||
		!strings.EqualFold(
			normalizedUsername.String(),
			requestedUsername.String(),
		) {
		return port.GitHubProfileAnalysisResult{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis response",
			errors.New("contains an invalid or mismatched user login"),
		)
	}

	if payload.Data.Owner.TypeName != "User" &&
		payload.Data.Owner.TypeName != "Organization" {
		return port.GitHubProfileAnalysisResult{}, upstreamDecodeError(
			"GitHub GraphQL profile analysis response",
			fmt.Errorf(
				"contains unsupported repository owner type %q",
				payload.Data.Owner.TypeName,
			),
		)
	}
	warnings := make([]profile.Warning, 0, 9)
	if payload.Data.Owner.TypeName == "Organization" {
		warnings = append(warnings, profile.Warning{
			Code: "organization_activity_unavailable",
			Message: "Personal stars and contribution activity do not apply " +
				"to organization profiles",
		})
	}
	owned, collectionWarnings, err := normalizeProfileRepositoryConnection(
		payload.Data.Owner.Owned,
		profile.RepositoryOwned,
		repositoryLimit,
		manifestLimit,
		true,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	warnings = append(warnings, collectionWarnings...)
	contributed, collectionWarnings, err := normalizeProfileRepositoryConnection(
		payload.Data.Owner.Contributed,
		profile.RepositoryContributed,
		repositoryLimit,
		manifestLimit,
		true,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	warnings = append(warnings, collectionWarnings...)
	starred, collectionWarnings, err := normalizeProfileRepositoryConnection(
		payload.Data.Owner.Starred,
		profile.RepositoryStarred,
		repositoryLimit,
		manifestLimit,
		false,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	warnings = append(warnings, collectionWarnings...)
	forked, collectionWarnings, err := normalizeProfileRepositoryConnection(
		payload.Data.Owner.Forked,
		profile.RepositoryForked,
		repositoryLimit,
		manifestLimit,
		true,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	warnings = append(warnings, collectionWarnings...)

	pullRequestSearch := payload.Data.AuthoredPullRequests
	issueSearch := payload.Data.AuthoredIssues
	if payload.Data.Owner.TypeName == "Organization" {
		pullRequestSearch = nil
		issueSearch = nil
	}
	contributions, contributionWarnings, err := normalizeProfileContributions(
		payload.Data.Owner.Contributions,
		pullRequestSearch,
		issueSearch,
		repositoryLimit,
	)
	if err != nil {
		return port.GitHubProfileAnalysisResult{}, err
	}
	warnings = append(warnings, contributionWarnings...)
	if len(payload.Errors) > 0 {
		warnings = append(warnings, profile.Warning{
			Code:    "github_partial_response",
			Message: "GitHub returned partial public profile data",
		})
	}

	return port.GitHubProfileAnalysisResult{
		Snapshot: profile.ProfileSnapshot{
			Username:      normalizedUsername,
			WindowFrom:    windowFrom,
			WindowTo:      windowTo,
			Owned:         owned,
			Contributed:   contributed,
			Starred:       starred,
			Forked:        forked,
			Contributions: contributions,
			Warnings:      warnings,
		},
		RateLimit: rateLimit,
	}, nil
}

func normalizeProfileRepositoryConnection(
	connection *graphQLProfileRepositoryConnection,
	source profile.RepositorySource,
	repositoryLimit int,
	manifestLimit int,
	strictPublic bool,
) (profile.RepositoryCollection, []profile.Warning, error) {
	if connection == nil {
		return profile.RepositoryCollection{
				Limit: repositoryLimit,
			}, []profile.Warning{{
				Code:    string(source) + "_repositories_unavailable",
				Message: "A public repository analysis segment is unavailable",
			}}, nil
	}
	if len(connection.Nodes) > repositoryLimit ||
		connection.TotalCount < 0 ||
		(strictPublic && connection.TotalCount < len(connection.Nodes)) {
		return profile.RepositoryCollection{}, nil, upstreamDecodeError(
			"GitHub GraphQL profile repository response",
			errors.New("contains invalid repository result counts"),
		)
	}

	observations := make(
		[]profile.RepositoryObservation,
		0,
		len(connection.Nodes),
	)
	warnings := make([]profile.Warning, 0)
	privateExcluded := false
	for index, node := range connection.Nodes {
		if node == nil {
			return profile.RepositoryCollection{}, nil, upstreamDecodeError(
				"GitHub GraphQL profile repository response",
				fmt.Errorf("contains null node %d", index),
			)
		}
		if node.Visibility != "PUBLIC" {
			if strictPublic {
				return profile.RepositoryCollection{}, nil, upstreamDecodeError(
					"GitHub GraphQL profile repository response",
					errors.New("contains a non-public repository"),
				)
			}
			privateExcluded = true
			continue
		}
		observation, observationWarnings, err := node.toObservation(
			source == profile.RepositoryOwned,
			manifestLimit,
		)
		if err != nil {
			return profile.RepositoryCollection{}, nil, upstreamDecodeError(
				"GitHub GraphQL profile repository response",
				fmt.Errorf("node %d: %w", index, err),
			)
		}
		observations = append(observations, observation)
		warnings = append(warnings, observationWarnings...)
	}
	if privateExcluded {
		warnings = append(warnings, profile.Warning{
			Code:    "private_starred_repositories_excluded",
			Message: "Non-public starred repositories were excluded",
		})
	}

	totalKnown := strictPublic
	total := connection.TotalCount
	hasMore := connection.PageInfo.HasNextPage || privateExcluded
	return profile.RepositoryCollection{
		Available:    true,
		Repositories: observations,
		Total:        total,
		TotalKnown:   totalKnown,
		Limit:        repositoryLimit,
		HasMore:      hasMore,
	}, warnings, nil
}

func normalizeProfileContributions(
	contributions *graphQLProfileContributions,
	pullRequestSearch *graphQLProfileSearch,
	issueSearch *graphQLProfileSearch,
	repositoryLimit int,
) (profile.ContributionSnapshot, []profile.Warning, error) {
	warnings := make([]profile.Warning, 0, 3)
	result := profile.ContributionSnapshot{}
	if contributions == nil {
		warnings = append(warnings, profile.Warning{
			Code:    "contribution_activity_unavailable",
			Message: "Public commit and review activity is unavailable",
		})
	} else {
		commits, err := sumPublicContributionGroups(
			contributions.Commits,
			repositoryLimit,
		)
		if err != nil {
			return profile.ContributionSnapshot{}, nil, err
		}
		reviews, err := sumPublicContributionGroups(
			contributions.Reviews,
			repositoryLimit,
		)
		if err != nil {
			return profile.ContributionSnapshot{}, nil, err
		}
		result.Commits = profile.CountEvidence{
			Available: true,
			Value:     commits,
			Complete:  false,
		}
		result.PullRequestReviews = profile.CountEvidence{
			Available: true,
			Value:     reviews,
			Complete:  false,
		}
		repositoriesTouched, err := countPublicContributionRepositories(
			repositoryLimit,
			contributions.Commits,
			contributions.Issues,
			contributions.PullRequests,
			contributions.Reviews,
		)
		if err != nil {
			return profile.ContributionSnapshot{}, nil, err
		}
		result.RepositoriesTouched = profile.CountEvidence{
			Available: true,
			Value:     repositoriesTouched,
			Complete:  false,
		}
		result.Available = true
	}
	if pullRequestSearch == nil {
		warnings = append(warnings, profile.Warning{
			Code:    "authored_pull_requests_unavailable",
			Message: "Public authored pull-request count is unavailable",
		})
	} else if pullRequestSearch.IssueCount < 0 {
		return profile.ContributionSnapshot{}, nil, upstreamDecodeError(
			"GitHub GraphQL profile contribution response",
			errors.New("contains a negative pull-request count"),
		)
	} else {
		result.PullRequestsOpened = profile.CountEvidence{
			Available: true,
			Value:     pullRequestSearch.IssueCount,
			Complete:  true,
		}
		result.Available = true
	}
	if issueSearch == nil {
		warnings = append(warnings, profile.Warning{
			Code:    "authored_issues_unavailable",
			Message: "Public authored issue count is unavailable",
		})
	} else if issueSearch.IssueCount < 0 {
		return profile.ContributionSnapshot{}, nil, upstreamDecodeError(
			"GitHub GraphQL profile contribution response",
			errors.New("contains a negative issue count"),
		)
	} else {
		result.IssuesOpened = profile.CountEvidence{
			Available: true,
			Value:     issueSearch.IssueCount,
			Complete:  true,
		}
		result.Available = true
	}
	return result, warnings, nil
}

func sumPublicContributionGroups(
	groups []graphQLProfileContributionGroup,
	repositoryLimit int,
) (int, error) {
	if len(groups) > repositoryLimit {
		return 0, upstreamDecodeError(
			"GitHub GraphQL profile contribution response",
			errors.New("contains too many contribution repository groups"),
		)
	}
	total := 0
	for _, group := range groups {
		if group.Contributions.TotalCount < 0 {
			return 0, upstreamDecodeError(
				"GitHub GraphQL profile contribution response",
				errors.New("contains a negative contribution count"),
			)
		}
		if group.Repository.Visibility == "PUBLIC" {
			total += group.Contributions.TotalCount
		}
	}
	return total, nil
}

func countPublicContributionRepositories(
	repositoryLimit int,
	groupCollections ...[]graphQLProfileContributionGroup,
) (int, error) {
	repositoryIDs := make(map[string]struct{})
	for _, groups := range groupCollections {
		if len(groups) > repositoryLimit {
			return 0, upstreamDecodeError(
				"GitHub GraphQL profile contribution response",
				errors.New("contains too many contribution repository groups"),
			)
		}
		for _, group := range groups {
			if group.Repository.Visibility != "PUBLIC" {
				continue
			}
			repositoryID := strings.TrimSpace(group.Repository.ID)
			if repositoryID == "" {
				return 0, upstreamDecodeError(
					"GitHub GraphQL profile contribution response",
					errors.New(
						"contains public contribution without repository identity",
					),
				)
			}
			repositoryIDs[repositoryID] = struct{}{}
		}
	}
	return len(repositoryIDs), nil
}

func profileErrorsContainNotFound(errors []graphQLError) bool {
	for _, graphQLError := range errors {
		classification := strings.ToUpper(strings.Join([]string{
			graphQLError.Type,
			graphQLError.Extensions.Code,
			graphQLError.Message,
		}, " "))
		if strings.Contains(classification, "NOT_FOUND") ||
			strings.Contains(classification, "NOT FOUND") ||
			strings.Contains(classification, "COULD NOT RESOLVE TO A USER") {
			return true
		}
	}
	return false
}

type graphQLProfileAnalysisRequest struct {
	Query     string                          `json:"query"`
	Variables graphQLProfileAnalysisVariables `json:"variables"`
}

type graphQLProfileAnalysisVariables struct {
	Login            string    `json:"login"`
	RepositoryLimit  int       `json:"repositoryLimit"`
	WindowFrom       time.Time `json:"windowFrom"`
	WindowTo         time.Time `json:"windowTo"`
	PullRequestQuery string    `json:"pullRequestQuery"`
	IssueQuery       string    `json:"issueQuery"`
}

type graphQLProfileAnalysisEnvelope struct {
	Data struct {
		Owner                *graphQLProfileOwner  `json:"repositoryOwner"`
		AuthoredPullRequests *graphQLProfileSearch `json:"authoredPullRequests"`
		AuthoredIssues       *graphQLProfileSearch `json:"authoredIssues"`
		RateLimit            *graphQLRateLimit     `json:"rateLimit"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLProfileOwner struct {
	TypeName      string                              `json:"__typename"`
	Login         string                              `json:"login"`
	Owned         *graphQLProfileRepositoryConnection `json:"ownedRepositories"`
	Contributed   *graphQLProfileRepositoryConnection `json:"contributedRepositories"`
	Starred       *graphQLProfileRepositoryConnection `json:"starredRepositories"`
	Forked        *graphQLProfileRepositoryConnection `json:"forkedRepositories"`
	Contributions *graphQLProfileContributions        `json:"contributionsCollection"`
}

type graphQLProfileRepositoryConnection struct {
	TotalCount int                         `json:"totalCount"`
	PageInfo   graphQLPageInfo             `json:"pageInfo"`
	Nodes      []*graphQLProfileRepository `json:"nodes"`
}

type graphQLProfileRepository struct {
	DatabaseID         *int64                           `json:"databaseId"`
	Owner              graphQLActor                     `json:"owner"`
	Name               string                           `json:"name"`
	NameWithOwner      string                           `json:"nameWithOwner"`
	URL                string                           `json:"url"`
	Description        *string                          `json:"description"`
	StargazerCount     int                              `json:"stargazerCount"`
	ForkCount          int                              `json:"forkCount"`
	OpenIssues         graphQLTotalCount                `json:"issues"`
	IsFork             bool                             `json:"isFork"`
	IsArchived         bool                             `json:"isArchived"`
	DefaultBranch      *graphQLRepositoryName           `json:"defaultBranchRef"`
	PrimaryLanguage    *graphQLRepositoryName           `json:"primaryLanguage"`
	UpdatedAt          time.Time                        `json:"updatedAt"`
	PushedAt           *time.Time                       `json:"pushedAt"`
	Visibility         string                           `json:"visibility"`
	Languages          graphQLProfileLanguageConnection `json:"languages"`
	PackageManifest    *graphQLProfileManifest          `json:"packageManifest"`
	GoManifest         *graphQLProfileManifest          `json:"goManifest"`
	MavenManifest      *graphQLProfileManifest          `json:"mavenManifest"`
	GradleManifest     *graphQLProfileManifest          `json:"gradleManifest"`
	PythonProject      *graphQLProfileManifest          `json:"pythonProject"`
	PythonRequirements *graphQLProfileManifest          `json:"pythonRequirements"`
	RustManifest       *graphQLProfileManifest          `json:"rustManifest"`
	ComposerManifest   *graphQLProfileManifest          `json:"composerManifest"`
}

type graphQLProfileLanguageConnection struct {
	TotalSize int                          `json:"totalSize"`
	Edges     []graphQLProfileLanguageEdge `json:"edges"`
}

type graphQLProfileLanguageEdge struct {
	Size int64                 `json:"size"`
	Node graphQLRepositoryName `json:"node"`
}

type graphQLProfileManifest struct {
	TypeName string  `json:"__typename"`
	ByteSize int     `json:"byteSize"`
	IsBinary bool    `json:"isBinary"`
	Text     *string `json:"text"`
}

type graphQLProfileContributions struct {
	Commits      []graphQLProfileContributionGroup `json:"commitContributionsByRepository"`
	Issues       []graphQLProfileContributionGroup `json:"issueContributionsByRepository"`
	PullRequests []graphQLProfileContributionGroup `json:"pullRequestContributionsByRepository"`
	Reviews      []graphQLProfileContributionGroup `json:"pullRequestReviewContributionsByRepository"`
}

type graphQLProfileContributionGroup struct {
	Repository struct {
		ID         string `json:"id"`
		Visibility string `json:"visibility"`
	} `json:"repository"`
	Contributions graphQLTotalCount `json:"contributions"`
}

type graphQLProfileSearch struct {
	IssueCount int `json:"issueCount"`
}

func (node graphQLProfileRepository) toObservation(
	includeDetails bool,
	manifestLimit int,
) (profile.RepositoryObservation, []profile.Warning, error) {
	summary, err := node.toSummary()
	if err != nil {
		return profile.RepositoryObservation{}, nil, err
	}
	observation := profile.RepositoryObservation{
		Repository: summary,
		Languages:  make(map[string]int64),
		Manifests:  make([]profile.Manifest, 0),
	}
	if !includeDetails {
		return observation, nil, nil
	}
	if node.Languages.TotalSize < 0 || len(node.Languages.Edges) > 10 {
		return profile.RepositoryObservation{}, nil, errors.New(
			"contains invalid language collection bounds",
		)
	}
	var observedBytes int64
	for _, edge := range node.Languages.Edges {
		name := strings.TrimSpace(edge.Node.Name)
		if name == "" || edge.Size <= 0 {
			return profile.RepositoryObservation{}, nil, errors.New(
				"contains invalid language evidence",
			)
		}
		observation.Languages[name] += edge.Size
		observedBytes += edge.Size
	}
	if observedBytes > int64(node.Languages.TotalSize) {
		return profile.RepositoryObservation{}, nil, errors.New(
			"language sample exceeds total bytes",
		)
	}

	warnings := make([]profile.Warning, 0)
	for _, manifestPath := range profile.ManifestCandidates(
		summary.MainLanguage,
		manifestLimit,
	) {
		manifest := node.manifest(manifestPath)
		if manifest == nil {
			continue
		}
		if manifest.TypeName != "Blob" ||
			manifest.IsBinary ||
			manifest.Text == nil ||
			manifest.ByteSize < 0 ||
			manifest.ByteSize > maxManifestBytes ||
			len(*manifest.Text) > maxManifestBytes {
			warnings = append(warnings, profile.Warning{
				Code:       "manifest_data_unavailable",
				Message:    "A framework manifest could not be analyzed safely",
				Repository: summary.FullName,
			})
			continue
		}
		observation.Manifests = append(
			observation.Manifests,
			profile.Manifest{
				Path:    manifestPath,
				Content: []byte(*manifest.Text),
			},
		)
	}
	return observation, warnings, nil
}

func (node graphQLProfileRepository) toSummary() (repository.Summary, error) {
	var id int64
	if node.DatabaseID != nil {
		id = *node.DatabaseID
	}
	var pushedAt time.Time
	if node.PushedAt != nil {
		pushedAt = node.PushedAt.UTC()
	}
	var mainLanguage string
	if node.PrimaryLanguage != nil {
		mainLanguage = strings.TrimSpace(node.PrimaryLanguage.Name)
	}
	var defaultBranch string
	if node.DefaultBranch != nil {
		defaultBranch = strings.TrimSpace(node.DefaultBranch.Name)
	}
	summary := repository.Summary{
		ID:            id,
		Owner:         strings.TrimSpace(node.Owner.Login),
		Name:          strings.TrimSpace(node.Name),
		FullName:      strings.TrimSpace(node.NameWithOwner),
		Description:   stringValue(node.Description),
		URL:           node.URL,
		MainLanguage:  mainLanguage,
		Stars:         node.StargazerCount,
		Forks:         node.ForkCount,
		OpenIssues:    node.OpenIssues.TotalCount,
		IsFork:        node.IsFork,
		IsArchived:    node.IsArchived,
		DefaultBranch: defaultBranch,
		UpdatedAt:     node.UpdatedAt.UTC(),
		PushedAt:      pushedAt,
	}
	if summary.Owner == "" ||
		summary.Name == "" ||
		summary.FullName == "" ||
		summary.URL == "" ||
		summary.Stars < 0 ||
		summary.Forks < 0 ||
		summary.OpenIssues < 0 ||
		summary.UpdatedAt.IsZero() {
		return repository.Summary{}, errors.New(
			"contains invalid repository fields",
		)
	}
	if err := validateAbsoluteHTTPURL(summary.URL); err != nil {
		return repository.Summary{}, fmt.Errorf("invalid repository URL: %w", err)
	}
	return summary, nil
}

func (node graphQLProfileRepository) manifest(
	manifestPath string,
) *graphQLProfileManifest {
	switch manifestPath {
	case "package.json":
		return node.PackageManifest
	case "go.mod":
		return node.GoManifest
	case "pom.xml":
		return node.MavenManifest
	case "build.gradle":
		return node.GradleManifest
	case "pyproject.toml":
		return node.PythonProject
	case "requirements.txt":
		return node.PythonRequirements
	case "Cargo.toml":
		return node.RustManifest
	case "composer.json":
		return node.ComposerManifest
	default:
		return nil
	}
}

var _ port.GitHubProfileAnalysisReader = (*Client)(nil)
