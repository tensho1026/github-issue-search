package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

const (
	maxIssueSearchRequestBytes = 16 << 10
	issueSearchCacheHeader     = "X-IssueScout-Cache"
	issueSearchCacheHit        = "HIT"
	issueSearchCacheMiss       = "MISS"
)

// IssueSearchHandler decodes bounded discovery input and presents ranked issue
// results with pagination and partial-result metadata.
type IssueSearchHandler struct {
	search    usecase.SearchIssues
	responder response.Responder
}

// NewIssueSearchHandler binds the issue-search use case to a responder.
func NewIssueSearchHandler(
	search usecase.SearchIssues,
	responder response.Responder,
) IssueSearchHandler {
	return IssueSearchHandler{search: search, responder: responder}
}

// Search handles one cancellable JSON issue-discovery request. Unknown fields,
// trailing values, oversized bodies, and unsupported query keys are rejected.
func (handler IssueSearchHandler) Search(ctx *gin.Context) {
	request, err := decodeIssueSearchRequest(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	pagination, err := parseIssueSearchPagination(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	criteria, err := issue.NewSearchCriteria(issue.SearchCriteriaOptions{
		Username:             request.Username,
		Languages:            request.Languages,
		Frameworks:           request.Frameworks,
		Labels:               request.Labels,
		MinimumStars:         request.MinimumStars,
		MaximumDifficulty:    request.MaximumDifficulty,
		MaximumEffort:        request.MaximumEffort,
		UpdatedWithinDays:    request.UpdatedWithinDays,
		IncludeDocumentation: request.IncludeDocumentation,
		IncludeEnglish:       request.IncludeEnglish,
		ExcludeArchived:      request.ExcludeArchived,
	})
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}

	output, err := handler.search.Execute(
		ctx.Request.Context(),
		usecase.SearchIssuesInput{
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
		newIssueSearchResponse(output),
		response.MetaOptions{RateLimitRemaining: remaining},
	)
}

func (handler IssueSearchHandler) invalidRequest(
	ctx *gin.Context,
	err error,
) {
	handler.responder.Error(ctx, apperror.Wrap(
		apperror.CodeInvalidRequest,
		"Issue search request is invalid",
		http.StatusBadRequest,
		err,
	))
}

type issueSearchRequest struct {
	Username             string   `json:"username"`
	Languages            []string `json:"languages"`
	Frameworks           []string `json:"frameworks"`
	Labels               []string `json:"labels"`
	MinimumStars         *int     `json:"minimumStars"`
	MaximumDifficulty    *int     `json:"maximumDifficulty"`
	MaximumEffort        *string  `json:"maximumEffort"`
	UpdatedWithinDays    *int     `json:"updatedWithinDays"`
	IncludeDocumentation *bool    `json:"includeDocumentation"`
	IncludeEnglish       *bool    `json:"includeEnglish"`
	ExcludeArchived      *bool    `json:"excludeArchived"`
}

func decodeIssueSearchRequest(ctx *gin.Context) (issueSearchRequest, error) {
	contentType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || contentType != "application/json" {
		return issueSearchRequest{}, fmt.Errorf(
			"Content-Type must be application/json",
		)
	}

	body := http.MaxBytesReader(
		ctx.Writer,
		ctx.Request.Body,
		maxIssueSearchRequestBytes,
	)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var request issueSearchRequest
	if err := decoder.Decode(&request); err != nil {
		return issueSearchRequest{}, fmt.Errorf(
			"decode issue search request: %w",
			err,
		)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return issueSearchRequest{}, fmt.Errorf(
				"issue search request must contain exactly one JSON object",
			)
		}
		return issueSearchRequest{}, fmt.Errorf(
			"decode trailing issue search request data: %w",
			err,
		)
	}
	return request, nil
}

func parseIssueSearchPagination(ctx *gin.Context) (issue.Pagination, error) {
	query := ctx.Request.URL.Query()
	for key := range query {
		if key != "page" && key != "perPage" {
			return issue.Pagination{}, fmt.Errorf(
				"unsupported query parameter %q",
				key,
			)
		}
	}
	page, err := parseSingleQueryInteger(query["page"], issue.DefaultPage)
	if err != nil {
		return issue.Pagination{}, fmt.Errorf("page: %w", err)
	}
	perPage, err := parseSingleQueryInteger(
		query["perPage"],
		issue.DefaultPerPage,
	)
	if err != nil {
		return issue.Pagination{}, fmt.Errorf("perPage: %w", err)
	}
	return issue.NewPagination(page, perPage)
}

func parseSingleQueryInteger(values []string, fallback int) (int, error) {
	if len(values) == 0 {
		return fallback, nil
	}
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		return 0, fmt.Errorf("must be provided exactly once")
	}
	value, err := strconv.Atoi(values[0])
	if err != nil {
		return 0, fmt.Errorf("must be an integer: %w", err)
	}
	return value, nil
}

type issueSearchResponse struct {
	Items         []issueSearchItemResponse `json:"items"`
	Pagination    paginationResponse        `json:"pagination"`
	SearchSummary searchSummaryResponse     `json:"searchSummary"`
	Warnings      []searchWarningResponse   `json:"warnings"`
}

type issueSearchItemResponse struct {
	Repository     repositorySearchResponse `json:"repository"`
	Issue          issueSummaryResponse     `json:"issue"`
	Difficulty     searchDifficultyResponse `json:"difficulty"`
	Effort         searchEffortResponse     `json:"effort"`
	Recommendation recommendationResponse   `json:"recommendation"`
}

type searchDifficultyResponse struct {
	Level      int              `json:"level"`
	Label      string           `json:"label"`
	Confidence issue.Confidence `json:"confidence"`
}

type searchEffortResponse struct {
	Band       issue.EffortBand `json:"band"`
	Label      string           `json:"label"`
	Confidence issue.Confidence `json:"confidence"`
}

type repositorySearchResponse struct {
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	FullName      string    `json:"fullName"`
	Description   string    `json:"description"`
	URL           string    `json:"url"`
	Stars         int       `json:"stars"`
	MainLanguage  string    `json:"mainLanguage"`
	IsArchived    bool      `json:"isArchived"`
	LastUpdatedAt time.Time `json:"lastUpdatedAt"`
}

type issueSummaryResponse struct {
	Number              int       `json:"number"`
	Title               string    `json:"title"`
	URL                 string    `json:"url"`
	Labels              []string  `json:"labels"`
	Comments            int       `json:"comments"`
	EstimatedDifficulty int       `json:"estimatedDifficulty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type paginationResponse struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"perPage"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasNext    bool `json:"hasNext"`
}

type searchSummaryResponse struct {
	CandidatesChecked   int                      `json:"candidatesChecked"`
	UpstreamTotal       int                      `json:"upstreamTotal"`
	EnrichmentAttempted int                      `json:"enrichmentAttempted"`
	EnrichmentFailed    int                      `json:"enrichmentFailed"`
	ExcludedByReason    []exclusionCountResponse `json:"excludedByReason"`
}

type exclusionCountResponse struct {
	Reason issue.ExclusionReason `json:"reason"`
	Count  int                   `json:"count"`
}

type searchWarningResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newIssueSearchResponse(
	output usecase.SearchIssuesOutput,
) issueSearchResponse {
	items := make([]issueSearchItemResponse, 0, len(output.Items))
	for _, ranked := range output.Items {
		candidate := ranked.Candidate
		items = append(items, issueSearchItemResponse{
			Repository: repositorySearchResponse{
				Owner:         candidate.Repository.Owner,
				Name:          candidate.Repository.Name,
				FullName:      candidate.Repository.FullName,
				Description:   candidate.Repository.Description,
				URL:           candidate.Repository.URL,
				Stars:         candidate.Repository.Stars,
				MainLanguage:  candidate.Repository.MainLanguage,
				IsArchived:    candidate.Repository.IsArchived,
				LastUpdatedAt: candidate.Repository.UpdatedAt,
			},
			Issue: issueSummaryResponse{
				Number:              candidate.Issue.Number,
				Title:               candidate.Issue.Title,
				URL:                 candidate.Issue.URL,
				Labels:              cloneResponseSlice(candidate.Issue.Labels),
				Comments:            candidate.Issue.Comments,
				EstimatedDifficulty: ranked.Analysis.Difficulty.Level.Int(),
				CreatedAt:           candidate.Issue.CreatedAt,
				UpdatedAt:           candidate.Issue.UpdatedAt,
			},
			Difficulty: searchDifficultyResponse{
				Level:      ranked.Analysis.Difficulty.Level.Int(),
				Label:      ranked.Analysis.Difficulty.Label,
				Confidence: ranked.Analysis.Difficulty.Confidence,
			},
			Effort: searchEffortResponse{
				Band:       ranked.Analysis.Effort.Band,
				Label:      ranked.Analysis.Effort.Label,
				Confidence: ranked.Analysis.Effort.Confidence,
			},
			Recommendation: newRecommendationResponse(
				ranked.Recommendation,
			),
		})
	}

	reasons := make([]issue.ExclusionReason, 0, len(output.ExclusionCounts))
	for reason := range output.ExclusionCounts {
		reasons = append(reasons, reason)
	}
	slices.Sort(reasons)
	exclusions := make([]exclusionCountResponse, 0, len(reasons))
	for _, reason := range reasons {
		exclusions = append(exclusions, exclusionCountResponse{
			Reason: reason,
			Count:  output.ExclusionCounts[reason],
		})
	}

	warnings := make([]searchWarningResponse, 0, 2)
	if output.GitHubIncomplete {
		warnings = append(warnings, searchWarningResponse{
			Code:    "github_search_incomplete",
			Message: "GitHub returned an incomplete search result window",
		})
	}
	if output.EnrichmentIncomplete {
		warnings = append(warnings, searchWarningResponse{
			Code: "issue_enrichment_incomplete",
			Message: "One or more bounded issue detail inspections were " +
				"unavailable or incomplete",
		})
	}

	return issueSearchResponse{
		Items: items,
		Pagination: paginationResponse{
			Page:       output.Pagination.Page,
			PerPage:    output.Pagination.PerPage,
			Total:      output.Pagination.Total,
			TotalPages: output.Pagination.TotalPages,
			HasNext:    output.Pagination.HasNext,
		},
		SearchSummary: searchSummaryResponse{
			CandidatesChecked:   output.CandidatesChecked,
			UpstreamTotal:       output.UpstreamTotal,
			EnrichmentAttempted: output.EnrichmentAttempted,
			EnrichmentFailed:    output.EnrichmentFailed,
			ExcludedByReason:    exclusions,
		},
		Warnings: warnings,
	}
}
