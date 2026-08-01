package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

const (
	maxIssueDetailSkills     = 20
	maxIssueDetailSkillBytes = 64
)

// IssueDetailHandler validates a public issue reference and bounded skills
// before invoking detailed recommendation analysis.
type IssueDetailHandler struct {
	recommend usecase.IssueRecommender
	responder response.Responder
}

// NewIssueDetailHandler binds the issue recommender to a shared responder.
func NewIssueDetailHandler(
	recommend usecase.IssueRecommender,
	responder response.Responder,
) IssueDetailHandler {
	return IssueDetailHandler{
		recommend: recommend,
		responder: responder,
	}
}

// Get handles one cancellable issue-detail request and reports cache and
// GitHub rate-limit metadata.
func (handler IssueDetailHandler) Get(ctx *gin.Context) {
	number, err := strconv.Atoi(ctx.Param("issueNumber"))
	if err != nil {
		handler.invalidRequest(ctx, fmt.Errorf("issueNumber must be an integer"))
		return
	}
	reference, err := issue.NewReference(
		ctx.Param("owner"),
		ctx.Param("repository"),
		number,
	)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	skills, err := parseIssueDetailSkills(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}

	output, err := handler.recommend.Execute(
		ctx.Request.Context(),
		usecase.RecommendIssueInput{
			Reference:     reference,
			DesiredSkills: skills,
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
		newIssueDetailResponse(output),
		response.MetaOptions{RateLimitRemaining: remaining},
	)
}

func (handler IssueDetailHandler) invalidRequest(
	ctx *gin.Context,
	err error,
) {
	handler.responder.Error(ctx, apperror.Wrap(
		apperror.CodeInvalidRequest,
		"Issue detail request is invalid",
		http.StatusBadRequest,
		err,
	))
}

func parseIssueDetailSkills(ctx *gin.Context) ([]string, error) {
	query := ctx.Request.URL.Query()
	for key := range query {
		if key != "skills" {
			return nil, fmt.Errorf("unsupported query parameter %q", key)
		}
	}
	values := query["skills"]
	if len(values) > maxIssueDetailSkills {
		return nil, fmt.Errorf(
			"skills may contain at most %d values",
			maxIssueDetailSkills,
		)
	}
	skills := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		skill := strings.TrimSpace(raw)
		if skill == "" || len(skill) > maxIssueDetailSkillBytes {
			return nil, fmt.Errorf(
				"each skill must contain between 1 and %d bytes",
				maxIssueDetailSkillBytes,
			)
		}
		for _, character := range skill {
			if unicode.IsControl(character) {
				return nil, fmt.Errorf("skills must not contain control characters")
			}
		}
		key := strings.ToLower(strings.Join(strings.Fields(skill), " "))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		skills = append(skills, strings.Join(strings.Fields(skill), " "))
	}
	return skills, nil
}

type issueDetailResponse struct {
	Repository       repositoryDetailResponse   `json:"repository"`
	Issue            issueDetailIssueResponse   `json:"issue"`
	Analysis         issueAnalysisResponse      `json:"analysis"`
	Recommendation   recommendationResponse     `json:"recommendation"`
	RepositoryHealth []repositorySignalResponse `json:"repositoryHealth"`
	Activity         activityMetricsResponse    `json:"activity"`
	Inspection       inspectionResponse         `json:"inspection"`
}

type repositoryDetailResponse struct {
	ID            int64      `json:"id"`
	Owner         string     `json:"owner"`
	Name          string     `json:"name"`
	FullName      string     `json:"fullName"`
	Description   string     `json:"description"`
	URL           string     `json:"url"`
	MainLanguage  string     `json:"mainLanguage"`
	Stars         int        `json:"stars"`
	Forks         int        `json:"forks"`
	OpenIssues    int        `json:"openIssues"`
	IsFork        bool       `json:"isFork"`
	IsArchived    bool       `json:"isArchived"`
	DefaultBranch string     `json:"defaultBranch"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	PushedAt      *time.Time `json:"pushedAt"`
}

type issueDetailIssueResponse struct {
	Number    int           `json:"number"`
	Title     string        `json:"title"`
	Body      string        `json:"body"`
	URL       string        `json:"url"`
	State     string        `json:"state"`
	Labels    []string      `json:"labels"`
	Assignees []string      `json:"assignees"`
	Author    actorResponse `json:"author"`
	Comments  int           `json:"comments"`
	Locked    bool          `json:"locked"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type actorResponse struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type inspectionResponse struct {
	Incomplete bool `json:"incomplete"`
}

func newIssueDetailResponse(
	output usecase.RecommendIssueOutput,
) issueDetailResponse {
	ranked := output.Item
	candidate := ranked.Candidate
	return issueDetailResponse{
		Repository: newRepositoryDetailResponse(candidate.Repository),
		Issue: issueDetailIssueResponse{
			Number:    candidate.Issue.Number,
			Title:     candidate.Issue.Title,
			Body:      candidate.Issue.Body,
			URL:       candidate.Issue.URL,
			State:     candidate.Issue.State,
			Labels:    cloneResponseSlice(candidate.Issue.Labels),
			Assignees: cloneResponseSlice(candidate.Issue.Assignees),
			Author: actorResponse{
				Login: candidate.Issue.AuthorLogin,
				Type:  candidate.Issue.AuthorType,
			},
			Comments:  candidate.Issue.Comments,
			Locked:    candidate.Issue.Locked,
			CreatedAt: candidate.Issue.CreatedAt,
			UpdatedAt: candidate.Issue.UpdatedAt,
		},
		Analysis:       newIssueAnalysisResponse(ranked.Analysis),
		Recommendation: newRecommendationResponse(ranked.Recommendation),
		RepositoryHealth: newRepositorySignalResponses(
			ranked.Recommendation.RepositorySignals,
		),
		Activity: newActivityMetricsResponse(
			ranked.Recommendation.Activity,
		),
		Inspection: inspectionResponse{Incomplete: output.Incomplete},
	}
}

func newRepositoryDetailResponse(
	item repository.Summary,
) repositoryDetailResponse {
	var pushedAt *time.Time
	if !item.PushedAt.IsZero() {
		value := item.PushedAt.UTC()
		pushedAt = &value
	}
	return repositoryDetailResponse{
		ID:            item.ID,
		Owner:         item.Owner,
		Name:          item.Name,
		FullName:      item.FullName,
		Description:   item.Description,
		URL:           item.URL,
		MainLanguage:  item.MainLanguage,
		Stars:         item.Stars,
		Forks:         item.Forks,
		OpenIssues:    item.OpenIssues,
		IsFork:        item.IsFork,
		IsArchived:    item.IsArchived,
		DefaultBranch: item.DefaultBranch,
		UpdatedAt:     item.UpdatedAt.UTC(),
		PushedAt:      pushedAt,
	}
}
