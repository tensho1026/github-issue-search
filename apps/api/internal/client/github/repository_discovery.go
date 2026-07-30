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
	"strconv"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	maxRepositorySearchQueryBytes        = 1024
	maxRepositorySearchResponseBytes     = 8 << 20
	maxRepositoryEnrichmentQueryBytes    = 64 << 10
	maxRepositoryEnrichmentResponseBytes = 8 << 20

	graphQLRepositorySearchDocument = `query IssueScoutRepositoryDiscovery($searchQuery: String!, $first: Int!) {
  search(query: $searchQuery, type: REPOSITORY, first: $first) {
    repositoryCount
    pageInfo {
      hasNextPage
    }
    nodes {
      __typename
      ... on Repository {
        databaseId
        owner {
          login
        }
        name
        nameWithOwner
        description
        url
        stargazerCount
        forkCount
        watchers {
          totalCount
        }
        issues(states: OPEN) {
          totalCount
        }
        goodFirstIssues: issues(states: OPEN, labels: ["good first issue"]) {
          totalCount
        }
        helpWantedIssues: issues(states: OPEN, labels: ["help wanted"]) {
          totalCount
        }
        isFork
        isArchived
        hasIssuesEnabled
        hasDiscussionsEnabled
        primaryLanguage {
          name
        }
        licenseInfo {
          name
          spdxId
        }
        repositoryTopics(first: 20) {
          nodes {
            topic {
              name
            }
          }
        }
        defaultBranchRef {
          name
        }
        updatedAt
        pushedAt
        codeOfConduct {
          key
        }
        securityPolicyUrl
      }
    }
  }
  rateLimit {
    limit
    remaining
    resetAt
  }
}`
)

// SearchRepositories retrieves exactly one bounded GraphQL repository window.
// README content is intentionally absent and is loaded only for the shortlist.
func (c *Client) SearchRepositories(
	ctx context.Context,
	criteria repository.DiscoveryCriteria,
	limit int,
) (port.GitHubRepositoryDiscoveryResult, error) {
	if limit < 1 || limit > repository.MaximumDiscoveryCandidateResults {
		return port.GitHubRepositoryDiscoveryResult{}, fmt.Errorf(
			"GitHub repository search limit must be between 1 and %d",
			repository.MaximumDiscoveryCandidateResults,
		)
	}
	searchQuery, err := buildRepositorySearchQuery(criteria, c.now())
	if err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, err
	}
	requestPayload, err := json.Marshal(graphQLRepositorySearchRequest{
		Query: graphQLRepositorySearchDocument,
		Variables: graphQLRepositorySearchVariables{
			SearchQuery: searchQuery,
			First:       limit,
		},
	})
	if err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, upstreamDecodeError(
			"GitHub GraphQL repository search request",
			err,
		)
	}

	response, err := c.graphQLRequest(ctx, requestPayload)
	if err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, err
	}
	defer response.Body.Close()
	headerRateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, headerRateLimit); statusErr != nil {
		return port.GitHubRepositoryDiscoveryResult{}, statusErr
	}

	var payload graphQLRepositorySearchEnvelope
	if err := decodeBoundedJSON(
		response.Body,
		maxRepositorySearchResponseBytes,
		"GitHub GraphQL repository search response",
		&payload,
	); err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, err
	}
	rateLimit, err := normalizeGraphQLRateLimit(
		payload.Data.RateLimit,
		headerRateLimit,
	)
	if err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, err
	}
	result, err := normalizeGraphQLRepositorySearch(payload, limit, rateLimit)
	if err != nil {
		return port.GitHubRepositoryDiscoveryResult{}, err
	}
	c.logger.Debug(
		"GitHub GraphQL repository search response received",
		"status", response.StatusCode,
		"candidateCount", len(result.Candidates),
		"upstreamTotal", result.TotalCount,
		"incompleteResults", result.IncompleteResults,
		"rateLimitKnown", result.RateLimit.Known,
		"rateLimitRemaining", result.RateLimit.Remaining,
	)
	return result, nil
}

// EnrichRepositories loads README and contribution-file evidence in one
// dynamically bounded GraphQL request. It never executes repository code.
func (c *Client) EnrichRepositories(
	ctx context.Context,
	repositories []repository.Summary,
) (port.GitHubRepositoryEnrichmentResult, error) {
	if len(repositories) == 0 {
		return port.GitHubRepositoryEnrichmentResult{
			Items: map[string]repository.DiscoveryEnrichment{},
		}, nil
	}
	if len(repositories) > repository.MaximumDiscoveryEnrichmentResults {
		return port.GitHubRepositoryEnrichmentResult{}, fmt.Errorf(
			"GitHub repository enrichment cannot contain more than %d repositories",
			repository.MaximumDiscoveryEnrichmentResults,
		)
	}
	query, aliases, unavailable, err := buildRepositoryEnrichmentQuery(
		repositories,
	)
	if err != nil {
		return port.GitHubRepositoryEnrichmentResult{}, err
	}
	if len(aliases) == 0 {
		return port.GitHubRepositoryEnrichmentResult{
			Items:             unavailable,
			IncompleteResults: true,
		}, nil
	}
	requestPayload, err := json.Marshal(struct {
		Query string `json:"query"`
	}{Query: query})
	if err != nil {
		return port.GitHubRepositoryEnrichmentResult{}, upstreamDecodeError(
			"GitHub GraphQL repository enrichment request",
			err,
		)
	}

	response, err := c.graphQLRequest(ctx, requestPayload)
	if err != nil {
		return port.GitHubRepositoryEnrichmentResult{}, err
	}
	defer response.Body.Close()
	headerRateLimit := parseRateLimit(response.Header)
	if statusErr := responseError(response.StatusCode, headerRateLimit); statusErr != nil {
		return port.GitHubRepositoryEnrichmentResult{}, statusErr
	}

	var payload graphQLRepositoryEnrichmentEnvelope
	if err := decodeBoundedJSON(
		response.Body,
		maxRepositoryEnrichmentResponseBytes,
		"GitHub GraphQL repository enrichment response",
		&payload,
	); err != nil {
		return port.GitHubRepositoryEnrichmentResult{}, err
	}
	rateLimit, err := enrichmentRateLimit(payload.Data, headerRateLimit)
	if err != nil {
		return port.GitHubRepositoryEnrichmentResult{}, err
	}
	if payload.Data == nil {
		if len(payload.Errors) > 0 {
			return port.GitHubRepositoryEnrichmentResult{},
				graphQLRequestError(
					"GitHub GraphQL repository enrichment",
					payload.Errors,
					rateLimit,
				)
		}
		return port.GitHubRepositoryEnrichmentResult{}, upstreamDecodeError(
			"GitHub GraphQL repository enrichment response",
			errors.New("does not contain data"),
		)
	}

	items := unavailable
	incomplete := len(unavailable) > 0 || len(payload.Errors) > 0
	for alias, fullName := range aliases {
		raw, present := payload.Data[alias]
		if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			items[fullName] = repository.DiscoveryEnrichment{}
			incomplete = true
			continue
		}
		var node graphQLRepositoryEnrichmentNode
		if err := json.Unmarshal(raw, &node); err != nil {
			items[fullName] = repository.DiscoveryEnrichment{}
			incomplete = true
			continue
		}
		enrichment, err := node.toDomain(fullName)
		if err != nil {
			items[fullName] = repository.DiscoveryEnrichment{}
			incomplete = true
			continue
		}
		items[fullName] = enrichment
	}
	return port.GitHubRepositoryEnrichmentResult{
		Items:             items,
		IncompleteResults: incomplete,
		RateLimit:         rateLimit,
	}, nil
}

func (c *Client) graphQLRequest(
	ctx context.Context,
	payload []byte,
) (*http.Response, error) {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, "graphql")
	endpoint.RawQuery = ""
	return c.doRequest(ctx, func() (*http.Request, error) {
		request, err := c.newRequest(
			ctx,
			http.MethodPost,
			endpoint.String(),
			bytes.NewReader(payload),
		)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		return request, nil
	})
}

func buildRepositorySearchQuery(
	criteria repository.DiscoveryCriteria,
	now time.Time,
) (string, error) {
	parts := []string{"is:public"}
	if criteria.ExcludesArchived() {
		parts = append(parts, "archived:false")
	}
	switch criteria.ForkPolicy() {
	case repository.ForkPolicyInclude:
		parts = append(parts, "fork:true")
	case repository.ForkPolicyOnly:
		parts = append(parts, "fork:only")
	}
	parts = append(
		parts,
		"stars:>="+strconv.Itoa(criteria.MinimumStars()),
		"forks:>="+strconv.Itoa(criteria.MinimumForks()),
		"pushed:>="+now.UTC().
			AddDate(0, 0, -criteria.UpdatedWithinDays()).
			Format(time.DateOnly),
	)
	if languages := criteria.Languages(); len(languages) > 0 {
		qualifiers := make([]string, len(languages))
		for index, language := range languages {
			qualifiers[index] = `language:"` + language.String() + `"`
		}
		parts = append(parts, "("+strings.Join(qualifiers, " OR ")+")")
	}
	if licenses := criteria.Licenses(); len(licenses) > 0 {
		qualifiers := make([]string, len(licenses))
		for index, license := range licenses {
			qualifiers[index] = "license:" + strings.ToLower(license.String())
		}
		parts = append(parts, "("+strings.Join(qualifiers, " OR ")+")")
	}
	parts = append(parts, "sort:updated-desc")

	query := strings.Join(parts, " ")
	if len(query) > maxRepositorySearchQueryBytes {
		return "", fmt.Errorf(
			"%w: the encoded GitHub repository query exceeds %d bytes",
			repository.ErrInvalidDiscoveryCriteria,
			maxRepositorySearchQueryBytes,
		)
	}
	return query, nil
}

func normalizeGraphQLRepositorySearch(
	payload graphQLRepositorySearchEnvelope,
	limit int,
	rateLimit port.RateLimit,
) (port.GitHubRepositoryDiscoveryResult, error) {
	if payload.Data.Search == nil {
		if len(payload.Errors) > 0 {
			return port.GitHubRepositoryDiscoveryResult{},
				graphQLRequestError(
					"GitHub GraphQL repository search",
					payload.Errors,
					rateLimit,
				)
		}
		return port.GitHubRepositoryDiscoveryResult{}, upstreamDecodeError(
			"GitHub GraphQL repository search response",
			errors.New("does not contain search data"),
		)
	}
	search := payload.Data.Search
	if search.RepositoryCount < 0 ||
		search.RepositoryCount < len(search.Nodes) ||
		len(search.Nodes) > limit {
		return port.GitHubRepositoryDiscoveryResult{}, upstreamDecodeError(
			"GitHub GraphQL repository search response",
			errors.New("contains invalid result counts"),
		)
	}

	candidates := make([]repository.DiscoveryCandidate, 0, len(search.Nodes))
	for index, node := range search.Nodes {
		var candidate repository.DiscoveryCandidate
		var mappingErr error
		if node == nil {
			mappingErr = errors.New("contains a null repository node")
		} else {
			candidate, mappingErr = node.toDomain()
		}
		if mappingErr != nil {
			if len(payload.Errors) > 0 {
				continue
			}
			return port.GitHubRepositoryDiscoveryResult{}, upstreamDecodeError(
				"GitHub GraphQL repository search response",
				fmt.Errorf("node %d: %w", index, mappingErr),
			)
		}
		candidates = append(candidates, candidate)
	}
	if len(payload.Errors) > 0 && len(candidates) == 0 {
		return port.GitHubRepositoryDiscoveryResult{},
			graphQLRequestError(
				"GitHub GraphQL repository search",
				payload.Errors,
				rateLimit,
			)
	}
	return port.GitHubRepositoryDiscoveryResult{
		Candidates: candidates,
		TotalCount: search.RepositoryCount,
		IncompleteResults: len(payload.Errors) > 0 ||
			search.PageInfo.HasNextPage ||
			search.RepositoryCount > len(search.Nodes),
		RateLimit: rateLimit,
	}, nil
}

func buildRepositoryEnrichmentQuery(
	repositories []repository.Summary,
) (
	string,
	map[string]string,
	map[string]repository.DiscoveryEnrichment,
	error,
) {
	var query strings.Builder
	query.WriteString("query IssueScoutRepositoryEnrichment {\n")
	aliases := make(map[string]string, len(repositories))
	unavailable := make(
		map[string]repository.DiscoveryEnrichment,
		len(repositories),
	)
	seen := make(map[string]struct{}, len(repositories))
	aliasIndex := 0
	for _, summary := range repositories {
		fullName := strings.ToLower(strings.TrimSpace(summary.FullName))
		if fullName == "" ||
			strings.TrimSpace(summary.Owner) == "" ||
			strings.TrimSpace(summary.Name) == "" {
			return "", nil, nil, fmt.Errorf(
				"repository enrichment identity is invalid",
			)
		}
		if _, duplicate := seen[fullName]; duplicate {
			continue
		}
		seen[fullName] = struct{}{}
		if strings.TrimSpace(summary.DefaultBranch) == "" {
			unavailable[fullName] = repository.DiscoveryEnrichment{}
			continue
		}
		alias := fmt.Sprintf("repo%d", aliasIndex)
		aliasIndex++
		aliases[alias] = fullName
		writeRepositoryEnrichmentSelection(&query, alias, summary)
	}
	query.WriteString(`  rateLimit {
    limit
    remaining
    resetAt
  }
}`)
	if query.Len() > maxRepositoryEnrichmentQueryBytes {
		return "", nil, nil, fmt.Errorf(
			"repository enrichment query exceeds %d bytes",
			maxRepositoryEnrichmentQueryBytes,
		)
	}
	return query.String(), aliases, unavailable, nil
}

func writeRepositoryEnrichmentSelection(
	query *strings.Builder,
	alias string,
	summary repository.Summary,
) {
	owner := quoteGraphQLString(summary.Owner)
	name := quoteGraphQLString(summary.Name)
	branch := summary.DefaultBranch
	expressions := map[string]string{
		"readmeMarkdown":       branch + ":README.md",
		"readmePlain":          branch + ":README",
		"readmeRST":            branch + ":README.rst",
		"readmeAsciidoc":       branch + ":README.adoc",
		"readmeJapanese":       branch + ":README.ja.md",
		"readmeJapaneseLocale": branch + ":README.ja-JP.md",
		"contributingRoot":     branch + ":CONTRIBUTING.md",
		"contributingGitHub":   branch + ":.github/CONTRIBUTING.md",
	}
	query.WriteString("  ")
	query.WriteString(alias)
	query.WriteString(": repository(owner: ")
	query.WriteString(owner)
	query.WriteString(", name: ")
	query.WriteString(name)
	query.WriteString(") {\n    nameWithOwner\n")
	for _, field := range []string{
		"readmeMarkdown",
		"readmePlain",
		"readmeRST",
		"readmeAsciidoc",
		"readmeJapanese",
		"readmeJapaneseLocale",
	} {
		query.WriteString("    ")
		query.WriteString(field)
		query.WriteString(": object(expression: ")
		query.WriteString(quoteGraphQLString(expressions[field]))
		query.WriteString(`) {
      ... on Blob {
        byteSize
        isBinary
        text
      }
    }
`)
	}
	for _, field := range []string{"contributingRoot", "contributingGitHub"} {
		query.WriteString("    ")
		query.WriteString(field)
		query.WriteString(": object(expression: ")
		query.WriteString(quoteGraphQLString(expressions[field]))
		query.WriteString(`) {
      ... on Blob {
        byteSize
        isBinary
      }
    }
`)
	}
	query.WriteString("  }\n")
}

func quoteGraphQLString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func enrichmentRateLimit(
	data map[string]json.RawMessage,
	fallback port.RateLimit,
) (port.RateLimit, error) {
	if data == nil {
		return fallback, nil
	}
	raw, exists := data["rateLimit"]
	if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fallback, nil
	}
	var graphQL graphQLRateLimit
	if err := json.Unmarshal(raw, &graphQL); err != nil {
		return port.RateLimit{}, upstreamDecodeError(
			"GitHub GraphQL repository enrichment rate limit",
			err,
		)
	}
	delete(data, "rateLimit")
	return normalizeGraphQLRateLimit(&graphQL, fallback)
}

func decodeBoundedJSON(
	body io.Reader,
	maximum int64,
	description string,
	target any,
) error {
	raw, err := io.ReadAll(io.LimitReader(body, maximum+1))
	if err != nil {
		return upstreamDecodeError(description, err)
	}
	if int64(len(raw)) > maximum {
		return upstreamDecodeError(
			description,
			fmt.Errorf("exceeds %d bytes", maximum),
		)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return upstreamDecodeError(description, err)
	}
	return nil
}

type graphQLRepositorySearchRequest struct {
	Query     string                           `json:"query"`
	Variables graphQLRepositorySearchVariables `json:"variables"`
}

type graphQLRepositorySearchVariables struct {
	SearchQuery string `json:"searchQuery"`
	First       int    `json:"first"`
}

type graphQLRepositorySearchEnvelope struct {
	Data struct {
		Search    *graphQLRepositorySearchConnection `json:"search"`
		RateLimit *graphQLRateLimit                  `json:"rateLimit"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLRepositorySearchConnection struct {
	RepositoryCount int                      `json:"repositoryCount"`
	PageInfo        graphQLPageInfo          `json:"pageInfo"`
	Nodes           []*graphQLRepositoryNode `json:"nodes"`
}

type graphQLRepositoryNode struct {
	TypeName              string                    `json:"__typename"`
	DatabaseID            *int64                    `json:"databaseId"`
	Owner                 graphQLActor              `json:"owner"`
	Name                  string                    `json:"name"`
	NameWithOwner         string                    `json:"nameWithOwner"`
	Description           *string                   `json:"description"`
	URL                   string                    `json:"url"`
	StargazerCount        int                       `json:"stargazerCount"`
	ForkCount             int                       `json:"forkCount"`
	Watchers              graphQLTotalCount         `json:"watchers"`
	OpenIssues            graphQLTotalCount         `json:"issues"`
	GoodFirstIssues       graphQLTotalCount         `json:"goodFirstIssues"`
	HelpWantedIssues      graphQLTotalCount         `json:"helpWantedIssues"`
	IsFork                bool                      `json:"isFork"`
	IsArchived            bool                      `json:"isArchived"`
	HasIssuesEnabled      bool                      `json:"hasIssuesEnabled"`
	HasDiscussionsEnabled bool                      `json:"hasDiscussionsEnabled"`
	PrimaryLanguage       *graphQLRepositoryName    `json:"primaryLanguage"`
	LicenseInfo           *graphQLRepositoryLicense `json:"licenseInfo"`
	RepositoryTopics      graphQLRepositoryTopics   `json:"repositoryTopics"`
	DefaultBranch         *graphQLRepositoryName    `json:"defaultBranchRef"`
	UpdatedAt             time.Time                 `json:"updatedAt"`
	PushedAt              *time.Time                `json:"pushedAt"`
	CodeOfConduct         *graphQLCodeOfConduct     `json:"codeOfConduct"`
	SecurityPolicyURL     *string                   `json:"securityPolicyUrl"`
}

type graphQLRepositoryLicense struct {
	Name   string `json:"name"`
	SPDXID string `json:"spdxId"`
}

type graphQLRepositoryTopics struct {
	Nodes []struct {
		Topic struct {
			Name string `json:"name"`
		} `json:"topic"`
	} `json:"nodes"`
}

type graphQLCodeOfConduct struct {
	Key string `json:"key"`
}

func (node graphQLRepositoryNode) toDomain() (
	repository.DiscoveryCandidate,
	error,
) {
	if node.TypeName != "Repository" {
		return repository.DiscoveryCandidate{}, fmt.Errorf(
			"contains unexpected node type %q",
			node.TypeName,
		)
	}
	if strings.TrimSpace(node.Owner.Login) == "" ||
		strings.TrimSpace(node.Name) == "" ||
		strings.TrimSpace(node.NameWithOwner) == "" ||
		strings.TrimSpace(node.URL) == "" ||
		node.StargazerCount < 0 ||
		node.ForkCount < 0 ||
		node.Watchers.TotalCount < 0 ||
		node.OpenIssues.TotalCount < 0 ||
		node.GoodFirstIssues.TotalCount < 0 ||
		node.HelpWantedIssues.TotalCount < 0 ||
		node.UpdatedAt.IsZero() {
		return repository.DiscoveryCandidate{}, errors.New(
			"contains invalid repository fields",
		)
	}
	if err := validateAbsoluteHTTPURL(node.URL); err != nil {
		return repository.DiscoveryCandidate{}, fmt.Errorf(
			"invalid repository URL: %w",
			err,
		)
	}

	topics := make([]string, 0, len(node.RepositoryTopics.Nodes))
	seenTopics := make(map[string]struct{}, len(node.RepositoryTopics.Nodes))
	for _, topicNode := range node.RepositoryTopics.Nodes {
		topic := strings.TrimSpace(topicNode.Topic.Name)
		if topic == "" {
			return repository.DiscoveryCandidate{}, errors.New(
				"contains a blank repository topic",
			)
		}
		key := strings.ToLower(topic)
		if _, duplicate := seenTopics[key]; duplicate {
			continue
		}
		seenTopics[key] = struct{}{}
		topics = append(topics, topic)
	}
	slices.SortFunc(topics, func(left, right string) int {
		return strings.Compare(strings.ToLower(left), strings.ToLower(right))
	})

	var id int64
	if node.DatabaseID != nil {
		id = *node.DatabaseID
	}
	var mainLanguage string
	if node.PrimaryLanguage != nil {
		mainLanguage = strings.TrimSpace(node.PrimaryLanguage.Name)
	}
	var defaultBranch string
	if node.DefaultBranch != nil {
		defaultBranch = strings.TrimSpace(node.DefaultBranch.Name)
	}
	var pushedAt time.Time
	if node.PushedAt != nil {
		pushedAt = node.PushedAt.UTC()
	}
	var license repository.SPDXLicense
	var licenseName string
	licenseKnown := false
	if node.LicenseInfo != nil {
		licenseName = strings.TrimSpace(node.LicenseInfo.Name)
		parsed, err := repository.ParseSPDXLicense(node.LicenseInfo.SPDXID)
		if err == nil {
			license = parsed
			licenseKnown = true
		}
	}
	return repository.DiscoveryCandidate{
		Repository: repository.Summary{
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
		},
		Topics:           topics,
		License:          license,
		LicenseName:      licenseName,
		LicenseKnown:     licenseKnown,
		Watchers:         node.Watchers.TotalCount,
		GoodFirstIssues:  node.GoodFirstIssues.TotalCount,
		HelpWantedIssues: node.HelpWantedIssues.TotalCount,
		HasIssuesEnabled: node.HasIssuesEnabled,
		HasDiscussions:   node.HasDiscussionsEnabled,
		HasCodeOfConduct: node.CodeOfConduct != nil,
		HasSecurityPolicy: node.SecurityPolicyURL != nil &&
			strings.TrimSpace(*node.SecurityPolicyURL) != "",
	}, nil
}

type graphQLRepositoryEnrichmentEnvelope struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []graphQLError             `json:"errors"`
}

type graphQLRepositoryEnrichmentNode struct {
	NameWithOwner        string                          `json:"nameWithOwner"`
	ReadmeMarkdown       *graphQLRepositoryDiscoveryBlob `json:"readmeMarkdown"`
	ReadmePlain          *graphQLRepositoryDiscoveryBlob `json:"readmePlain"`
	ReadmeRST            *graphQLRepositoryDiscoveryBlob `json:"readmeRST"`
	ReadmeAsciidoc       *graphQLRepositoryDiscoveryBlob `json:"readmeAsciidoc"`
	ReadmeJapanese       *graphQLRepositoryDiscoveryBlob `json:"readmeJapanese"`
	ReadmeJapaneseLocale *graphQLRepositoryDiscoveryBlob `json:"readmeJapaneseLocale"`
	ContributingRoot     *graphQLRepositoryDiscoveryBlob `json:"contributingRoot"`
	ContributingGitHub   *graphQLRepositoryDiscoveryBlob `json:"contributingGitHub"`
}

type graphQLRepositoryDiscoveryBlob struct {
	ByteSize int     `json:"byteSize"`
	IsBinary bool    `json:"isBinary"`
	Text     *string `json:"text"`
}

func (node graphQLRepositoryEnrichmentNode) toDomain(
	expectedFullName string,
) (repository.DiscoveryEnrichment, error) {
	if !strings.EqualFold(
		strings.TrimSpace(node.NameWithOwner),
		expectedFullName,
	) {
		return repository.DiscoveryEnrichment{}, errors.New(
			"repository enrichment identity does not match",
		)
	}
	readmes := []*graphQLRepositoryDiscoveryBlob{
		node.ReadmeJapaneseLocale,
		node.ReadmeJapanese,
		node.ReadmeMarkdown,
		node.ReadmePlain,
		node.ReadmeRST,
		node.ReadmeAsciidoc,
	}
	available := false
	contentAvailable := false
	sampled := false
	var content strings.Builder
	for _, blob := range readmes {
		if blob == nil {
			continue
		}
		if blob.ByteSize < 0 {
			return repository.DiscoveryEnrichment{}, errors.New(
				"repository enrichment contains a negative blob size",
			)
		}
		available = true
		if blob.IsBinary || blob.Text == nil {
			continue
		}
		contentAvailable = true
		if content.Len() > 0 {
			content.WriteByte('\n')
		}
		content.WriteString(*blob.Text)
		if blob.ByteSize > len(*blob.Text) {
			sampled = true
		}
	}
	return repository.DiscoveryEnrichment{
		Available:              true,
		READMEAvailable:        available,
		READMEContentAvailable: contentAvailable,
		READMEText:             content.String(),
		READMEContentSampled:   sampled,
		ContributingAvailable: node.ContributingRoot != nil ||
			node.ContributingGitHub != nil,
	}, nil
}
