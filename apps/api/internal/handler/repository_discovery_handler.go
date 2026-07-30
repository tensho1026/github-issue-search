package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

const maxRepositoryDiscoveryRequestBytes = 16 << 10

type RepositoryDiscoveryHandler struct {
	search    usecase.SearchRepositories
	responder response.Responder
}

func NewRepositoryDiscoveryHandler(
	search usecase.SearchRepositories,
	responder response.Responder,
) RepositoryDiscoveryHandler {
	return RepositoryDiscoveryHandler{search: search, responder: responder}
}

func (handler RepositoryDiscoveryHandler) Search(ctx *gin.Context) {
	request, err := decodeRepositoryDiscoveryRequest(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	pagination, err := parseRepositoryDiscoveryPagination(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	criteria, err := repository.NewDiscoveryCriteria(
		repository.DiscoveryCriteriaOptions{
			Languages:         request.Languages,
			Technologies:      request.Technologies,
			Licenses:          request.Licenses,
			Categories:        request.Categories,
			MinimumStars:      request.MinimumStars,
			MinimumForks:      request.MinimumForks,
			MinimumOpenIssues: request.MinimumOpenIssues,
			MaximumOpenIssues: request.MaximumOpenIssues,
			UpdatedWithinDays: request.UpdatedWithinDays,
			MaximumDifficulty: request.MaximumDifficulty,
			MinimumReadiness:  request.MinimumReadiness,
			HasJapaneseREADME: request.HasJapaneseREADME,
			ForkPolicy:        request.ForkPolicy,
			ExcludeArchived:   request.ExcludeArchived,
		},
	)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}

	output, err := handler.search.Execute(
		ctx.Request.Context(),
		usecase.SearchRepositoriesInput{
			Criteria:   criteria,
			Pagination: pagination,
		},
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}

	cacheStatus := issueSearchCacheMiss
	if output.CacheHit {
		cacheStatus = issueSearchCacheHit
	}
	ctx.Header(issueSearchCacheHeader, cacheStatus)

	var remaining *int
	if output.RateLimit.Known {
		remaining = &output.RateLimit.Remaining
	}
	handler.responder.DataWithMeta(
		ctx,
		http.StatusOK,
		newRepositoryDiscoveryResponse(output),
		response.MetaOptions{RateLimitRemaining: remaining},
	)
}

func (handler RepositoryDiscoveryHandler) invalidRequest(
	ctx *gin.Context,
	err error,
) {
	handler.responder.Error(ctx, apperror.Wrap(
		apperror.CodeInvalidRequest,
		"Repository discovery request is invalid",
		http.StatusBadRequest,
		err,
	))
}

type repositoryDiscoveryRequest struct {
	Languages         []string `json:"languages"`
	Technologies      []string `json:"technologies"`
	Licenses          []string `json:"licenses"`
	Categories        []string `json:"categories"`
	MinimumStars      *int     `json:"minimumStars"`
	MinimumForks      *int     `json:"minimumForks"`
	MinimumOpenIssues *int     `json:"minimumOpenIssues"`
	MaximumOpenIssues *int     `json:"maximumOpenIssues"`
	UpdatedWithinDays *int     `json:"updatedWithinDays"`
	MaximumDifficulty *int     `json:"maximumDifficulty"`
	MinimumReadiness  *int     `json:"minimumReadiness"`
	HasJapaneseREADME *bool    `json:"hasJapaneseReadme"`
	ForkPolicy        *string  `json:"forkPolicy"`
	ExcludeArchived   *bool    `json:"excludeArchived"`
}

func decodeRepositoryDiscoveryRequest(
	ctx *gin.Context,
) (repositoryDiscoveryRequest, error) {
	contentType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || contentType != "application/json" {
		return repositoryDiscoveryRequest{}, fmt.Errorf(
			"Content-Type must be application/json",
		)
	}
	body := http.MaxBytesReader(
		ctx.Writer,
		ctx.Request.Body,
		maxRepositoryDiscoveryRequestBytes,
	)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var request repositoryDiscoveryRequest
	if err := decoder.Decode(&request); err != nil {
		return repositoryDiscoveryRequest{}, fmt.Errorf(
			"decode repository discovery request: %w",
			err,
		)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return repositoryDiscoveryRequest{}, fmt.Errorf(
				"repository discovery request must contain exactly one JSON object",
			)
		}
		return repositoryDiscoveryRequest{}, fmt.Errorf(
			"decode trailing repository discovery request data: %w",
			err,
		)
	}
	return request, nil
}

func parseRepositoryDiscoveryPagination(
	ctx *gin.Context,
) (repository.DiscoveryPagination, error) {
	query := ctx.Request.URL.Query()
	for key := range query {
		if key != "page" && key != "perPage" {
			return repository.DiscoveryPagination{}, fmt.Errorf(
				"unsupported query parameter %q",
				key,
			)
		}
	}
	page, err := parseSingleQueryInteger(
		query["page"],
		repository.DefaultDiscoveryPage,
	)
	if err != nil {
		return repository.DiscoveryPagination{}, fmt.Errorf("page: %w", err)
	}
	perPage, err := parseSingleQueryInteger(
		query["perPage"],
		repository.DefaultDiscoveryPerPage,
	)
	if err != nil {
		return repository.DiscoveryPagination{}, fmt.Errorf(
			"perPage: %w",
			err,
		)
	}
	return repository.NewDiscoveryPagination(page, perPage)
}

type repositoryDiscoveryResponse struct {
	Items         []repositoryDiscoveryItemResponse    `json:"items"`
	Pagination    paginationResponse                   `json:"pagination"`
	SearchSummary repositoryDiscoverySummaryResponse   `json:"searchSummary"`
	Warnings      []repositoryDiscoveryWarningResponse `json:"warnings"`
}

type repositoryDiscoveryItemResponse struct {
	Repository    repositoryDiscoveryIdentityResponse      `json:"repository"`
	Topics        []string                                 `json:"topics"`
	Technologies  []string                                 `json:"technologies"`
	Language      string                                   `json:"language"`
	Category      repository.Category                      `json:"category"`
	License       repositoryDiscoveryLicenseResponse       `json:"license"`
	Popularity    repositoryDiscoveryPopularityResponse    `json:"popularity"`
	Activity      repositoryDiscoveryActivityResponse      `json:"activity"`
	Readiness     repositoryDiscoveryReadinessResponse     `json:"readiness"`
	Documentation repositoryDiscoveryDocumentationResponse `json:"documentation"`
	Difficulty    repositoryDiscoveryDifficultyResponse    `json:"difficulty"`
	Warnings      []repositoryDiscoveryWarningResponse     `json:"warnings"`
}

type repositoryDiscoveryIdentityResponse struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	URL         string `json:"url"`
	IsFork      bool   `json:"isFork"`
	IsArchived  bool   `json:"isArchived"`
}

type repositoryDiscoveryLicenseResponse struct {
	SPDXID *string `json:"spdxId"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
}

type repositoryDiscoveryPopularityResponse struct {
	Stars      int `json:"stars"`
	Forks      int `json:"forks"`
	Watchers   int `json:"watchers"`
	OpenIssues int `json:"openIssues"`
}

type repositoryDiscoveryActivityResponse struct {
	UpdatedAt time.Time `json:"updatedAt"`
	PushedAt  time.Time `json:"pushedAt"`
}

type repositoryDiscoveryReadinessResponse struct {
	Score              int                      `json:"score"`
	Band               repository.ReadinessBand `json:"band"`
	Reasons            []string                 `json:"reasons"`
	GoodFirstIssues    int                      `json:"goodFirstIssues"`
	HelpWantedIssues   int                      `json:"helpWantedIssues"`
	IssuesEnabled      bool                     `json:"issuesEnabled"`
	DiscussionsEnabled bool                     `json:"discussionsEnabled"`
}

type repositoryDiscoveryDocumentationResponse struct {
	Status            repository.EvidenceStatus                 `json:"status"`
	ReadmeAvailable   bool                                      `json:"readmeAvailable"`
	ContributingGuide bool                                      `json:"contributingGuide"`
	CodeOfConduct     bool                                      `json:"codeOfConduct"`
	SecurityPolicy    bool                                      `json:"securityPolicy"`
	JapaneseReadme    repositoryDiscoveryJapaneseREADMEResponse `json:"japaneseReadme"`
}

type repositoryDiscoveryJapaneseREADMEResponse struct {
	Detected      bool                      `json:"detected"`
	Status        repository.EvidenceStatus `json:"status"`
	Confidence    repository.Confidence     `json:"confidence"`
	JapaneseRunes int                       `json:"japaneseRunes"`
	LetterRunes   int                       `json:"letterRunes"`
	AnalyzedBytes int                       `json:"analyzedBytes"`
}

type repositoryDiscoveryDifficultyResponse struct {
	Level   int      `json:"level"`
	Label   string   `json:"label"`
	Reasons []string `json:"reasons"`
}

type repositoryDiscoverySummaryResponse struct {
	CandidatesChecked    int  `json:"candidatesChecked"`
	UpstreamTotal        int  `json:"upstreamTotal"`
	EnrichmentAttempted  int  `json:"enrichmentAttempted"`
	EnrichmentFailed     int  `json:"enrichmentFailed"`
	GitHubIncomplete     bool `json:"githubIncomplete"`
	EnrichmentIncomplete bool `json:"enrichmentIncomplete"`
}

type repositoryDiscoveryWarningResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newRepositoryDiscoveryResponse(
	output usecase.SearchRepositoriesOutput,
) repositoryDiscoveryResponse {
	items := make(
		[]repositoryDiscoveryItemResponse,
		len(output.Items),
	)
	for index, item := range output.Items {
		items[index] = newRepositoryDiscoveryItemResponse(item)
	}
	warnings := make(
		[]repositoryDiscoveryWarningResponse,
		0,
		len(output.Warnings),
	)
	for _, warning := range output.Warnings {
		warnings = append(
			warnings,
			repositoryDiscoveryApplicationWarning(warning),
		)
	}
	return repositoryDiscoveryResponse{
		Items: items,
		Pagination: paginationResponse{
			Page:       output.Pagination.Page,
			PerPage:    output.Pagination.PerPage,
			Total:      output.Pagination.Total,
			TotalPages: output.Pagination.TotalPages,
			HasNext:    output.Pagination.HasNext,
		},
		SearchSummary: repositoryDiscoverySummaryResponse{
			CandidatesChecked:    output.CandidatesChecked,
			UpstreamTotal:        output.UpstreamTotal,
			EnrichmentAttempted:  output.EnrichmentAttempted,
			EnrichmentFailed:     output.EnrichmentFailed,
			GitHubIncomplete:     output.GitHubIncomplete,
			EnrichmentIncomplete: output.EnrichmentIncomplete,
		},
		Warnings: warnings,
	}
}

func newRepositoryDiscoveryItemResponse(
	item repository.DiscoveryResult,
) repositoryDiscoveryItemResponse {
	var spdxID *string
	if item.LicenseKnown {
		value := item.License.String()
		spdxID = &value
	}
	itemWarnings := make(
		[]repositoryDiscoveryWarningResponse,
		0,
		len(item.Warnings),
	)
	for _, warning := range item.Warnings {
		itemWarnings = append(
			itemWarnings,
			repositoryDiscoveryItemWarning(warning),
		)
	}
	return repositoryDiscoveryItemResponse{
		Repository: repositoryDiscoveryIdentityResponse{
			Owner:       item.Repository.Owner,
			Name:        item.Repository.Name,
			FullName:    item.Repository.FullName,
			Description: item.Repository.Description,
			URL:         item.Repository.URL,
			IsFork:      item.Repository.IsFork,
			IsArchived:  item.Repository.IsArchived,
		},
		Topics:       slices.Clone(item.Topics),
		Technologies: slices.Clone(item.Technologies),
		Language:     item.Repository.MainLanguage,
		Category:     item.Category,
		License: repositoryDiscoveryLicenseResponse{
			SPDXID: spdxID,
			Name:   item.LicenseName,
			Status: licenseStatus(item.LicenseKnown),
		},
		Popularity: repositoryDiscoveryPopularityResponse{
			Stars:      item.Repository.Stars,
			Forks:      item.Repository.Forks,
			Watchers:   item.Watchers,
			OpenIssues: item.Repository.OpenIssues,
		},
		Activity: repositoryDiscoveryActivityResponse{
			UpdatedAt: item.Repository.UpdatedAt,
			PushedAt:  item.Repository.PushedAt,
		},
		Readiness: repositoryDiscoveryReadinessResponse{
			Score:              item.Readiness.Score,
			Band:               item.Readiness.Band,
			Reasons:            slices.Clone(item.Readiness.Reasons),
			GoodFirstIssues:    item.GoodFirstIssues,
			HelpWantedIssues:   item.HelpWantedIssues,
			IssuesEnabled:      item.HasIssuesEnabled,
			DiscussionsEnabled: item.HasDiscussions,
		},
		Documentation: repositoryDiscoveryDocumentationResponse{
			Status:            item.Documentation.Status,
			ReadmeAvailable:   item.Documentation.READMEAvailable,
			ContributingGuide: item.Documentation.ContributingAvailable,
			CodeOfConduct:     item.Documentation.CodeOfConduct,
			SecurityPolicy:    item.Documentation.SecurityPolicy,
			JapaneseReadme: repositoryDiscoveryJapaneseREADMEResponse{
				Detected:      item.Documentation.JapaneseREADME.Detected,
				Status:        item.Documentation.JapaneseREADME.Status,
				Confidence:    item.Documentation.JapaneseREADME.Confidence,
				JapaneseRunes: item.Documentation.JapaneseREADME.JapaneseRunes,
				LetterRunes:   item.Documentation.JapaneseREADME.LetterRunes,
				AnalyzedBytes: item.Documentation.JapaneseREADME.SampledBytes,
			},
		},
		Difficulty: repositoryDiscoveryDifficultyResponse{
			Level:   item.Difficulty.Level,
			Label:   item.Difficulty.Label,
			Reasons: slices.Clone(item.Difficulty.Reasons),
		},
		Warnings: itemWarnings,
	}
}

func repositoryDiscoveryApplicationWarning(
	warning usecase.RepositoryDiscoveryWarning,
) repositoryDiscoveryWarningResponse {
	switch warning {
	case usecase.RepositoryDiscoveryWarningGitHubIncomplete:
		return repositoryDiscoveryWarningResponse{
			Code: "github_results_incomplete",
			Message: "GitHub reported more or partially unavailable " +
				"repositories than the bounded candidate window",
		}
	default:
		return repositoryDiscoveryWarningResponse{
			Code: "repository_enrichment_incomplete",
			Message: "Some shortlist documentation evidence was unavailable; " +
				"affected fields are marked unavailable",
		}
	}
}

func repositoryDiscoveryItemWarning(
	warning repository.DiscoveryWarning,
) repositoryDiscoveryWarningResponse {
	switch warning {
	case repository.WarningREADMEContentSampled:
		return repositoryDiscoveryWarningResponse{
			Code: "readme_content_sampled",
			Message: "README language and technology evidence uses a bounded " +
				"content sample",
		}
	default:
		return repositoryDiscoveryWarningResponse{
			Code:    "enrichment_unavailable",
			Message: "README and contribution-file evidence was unavailable",
		}
	}
}

func licenseStatus(known bool) string {
	if known {
		return string(repository.EvidenceExact)
	}
	return string(repository.EvidenceUnavailable)
}
