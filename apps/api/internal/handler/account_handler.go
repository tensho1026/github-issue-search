package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

const maximumAccountRequestBytes = 16 << 10

// AccountHandler exposes only authenticated account-owned data. All methods
// still verify the principal installed by authentication middleware so direct
// handler invocation cannot select an arbitrary account.
type AccountHandler struct {
	workspace usecase.AccountWorkspace
	cookies   authhttp.Policy
	responder response.Responder
}

// NewAccountHandler constructs the account-only HTTP adapter.
func NewAccountHandler(
	workspace usecase.AccountWorkspace,
	cookies authhttp.Policy,
	responder response.Responder,
) AccountHandler {
	return AccountHandler{
		workspace: workspace,
		cookies:   cookies,
		responder: responder,
	}
}

// ListBookmarks returns an owned deterministic page of normalized references.
func (handler AccountHandler) ListBookmarks(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	page, err := parseAccountPage(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	result, err := handler.workspace.ListBookmarks(
		ctx.Request.Context(),
		accountID,
		page,
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	items := make([]bookmarkResponse, len(result.Items))
	for index, bookmark := range result.Items {
		items[index] = newBookmarkResponse(bookmark)
	}
	handler.responder.Data(ctx, http.StatusOK, bookmarkListResponse{
		Items:      items,
		Pagination: newAccountPagination(result.Page, result.Total),
	})
}

// UpsertBookmark creates or idempotently returns one normalized reference.
func (handler AccountHandler) UpsertBookmark(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	request, err := decodeAccountBody[bookmarkWriteRequest](ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	bookmark, err := handler.workspace.UpsertBookmark(
		ctx.Request.Context(),
		accountID,
		usecase.UpsertBookmarkInput{
			TargetType:      account.BookmarkTarget(request.TargetType),
			RepositoryOwner: request.RepositoryOwner,
			RepositoryName:  request.RepositoryName,
			IssueNumber:     request.IssueNumber,
		},
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(
		ctx,
		http.StatusOK,
		newBookmarkResponse(bookmark),
	)
}

// DeleteBookmark deletes an owned bookmark at the supplied current version.
func (handler AccountHandler) DeleteBookmark(ctx *gin.Context) {
	accountID, resourceID, version, ok := handler.ownedMutationTarget(ctx)
	if !ok {
		return
	}
	if err := handler.workspace.DeleteBookmark(
		ctx.Request.Context(),
		accountID,
		resourceID,
		version,
	); err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(ctx, http.StatusOK, deletionResponse{Deleted: true})
}

// ListSavedSearches returns an owned deterministic page of named filters.
func (handler AccountHandler) ListSavedSearches(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	page, err := parseAccountPage(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	result, err := handler.workspace.ListSavedSearches(
		ctx.Request.Context(),
		accountID,
		page,
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	items := make([]savedSearchResponse, len(result.Items))
	for index, savedSearch := range result.Items {
		items[index] = newSavedSearchResponse(savedSearch)
	}
	handler.responder.Data(ctx, http.StatusOK, savedSearchListResponse{
		Items:      items,
		Pagination: newAccountPagination(result.Page, result.Total),
	})
}

// CreateSavedSearch validates and persists a named anonymous-search filter.
func (handler AccountHandler) CreateSavedSearch(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	request, err := decodeAccountBody[savedSearchWriteRequest](ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	savedSearch, err := handler.workspace.CreateSavedSearch(
		ctx.Request.Context(),
		accountID,
		request.input(),
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(
		ctx,
		http.StatusCreated,
		newSavedSearchResponse(savedSearch),
	)
}

// UpdateSavedSearch replaces an owned filter only at its current version.
func (handler AccountHandler) UpdateSavedSearch(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	resourceID, err := account.ParseResourceID(ctx.Param("savedSearchID"))
	if err != nil {
		handler.invalidRequest(ctx, err)
		return
	}
	request, err := decodeAccountBody[savedSearchUpdateRequest](ctx)
	if err != nil || request.Version < 1 {
		handler.invalidRequest(ctx, err)
		return
	}
	savedSearch, err := handler.workspace.UpdateSavedSearch(
		ctx.Request.Context(),
		accountID,
		resourceID,
		request.Version,
		request.input(),
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(
		ctx,
		http.StatusOK,
		newSavedSearchResponse(savedSearch),
	)
}

// DeleteSavedSearch deletes an owned named filter at the supplied version.
func (handler AccountHandler) DeleteSavedSearch(ctx *gin.Context) {
	accountID, resourceID, version, ok := handler.ownedMutationTarget(ctx)
	if !ok {
		return
	}
	if err := handler.workspace.DeleteSavedSearch(
		ctx.Request.Context(),
		accountID,
		resourceID,
		version,
	); err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(ctx, http.StatusOK, deletionResponse{Deleted: true})
}

// GetPreferences returns persisted settings or deterministic version-zero
// defaults.
func (handler AccountHandler) GetPreferences(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	preferences, err := handler.workspace.GetPreferences(
		ctx.Request.Context(),
		accountID,
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(
		ctx,
		http.StatusOK,
		newPreferencesResponse(preferences),
	)
}

// UpdatePreferences creates or optimistically updates display preferences.
func (handler AccountHandler) UpdatePreferences(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	request, err := decodeAccountBody[preferencesWriteRequest](ctx)
	if err != nil || request.Version < 0 {
		handler.invalidRequest(ctx, err)
		return
	}
	preferences, err := handler.workspace.UpdatePreferences(
		ctx.Request.Context(),
		accountID,
		request.Version,
		usecase.UpdatePreferencesInput{
			Theme:          account.Theme(request.Theme),
			ReducedMotion:  account.ReducedMotion(request.ReducedMotion),
			ResultsPerPage: request.ResultsPerPage,
		},
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(
		ctx,
		http.StatusOK,
		newPreferencesResponse(preferences),
	)
}

// Export returns the bounded, non-secret account-owned feature data set.
func (handler AccountHandler) Export(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	export, err := handler.workspace.Export(ctx.Request.Context(), accountID)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.responder.Data(ctx, http.StatusOK, newAccountExportResponse(export))
}

// DeleteAccount permanently removes the account after explicit confirmation.
// Database cascades revoke every session before browser cookies are cleared.
func (handler AccountHandler) DeleteAccount(ctx *gin.Context) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return
	}
	request, err := decodeAccountBody[accountDeleteRequest](ctx)
	if err != nil || request.Confirmation != "DELETE" {
		handler.invalidRequest(ctx, err)
		return
	}
	summary, err := handler.workspace.DeleteAccount(
		ctx.Request.Context(),
		accountID,
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.cookies.ClearSession(ctx.Writer)
	handler.responder.Data(ctx, http.StatusOK, accountDeleteResponse{
		Deleted: true,
		Removed: ownedDataSummaryResponse{
			Bookmarks:     summary.Bookmarks,
			Identities:    summary.Identities,
			Preferences:   summary.Preferences,
			SavedSearches: summary.SavedSearches,
			Sessions:      summary.Sessions,
		},
	})
}

func (handler AccountHandler) accountID(
	ctx *gin.Context,
) (account.ID, bool) {
	if handler.workspace == nil {
		handler.responder.Error(ctx, apperror.New(
			apperror.CodeAuthUnavailable,
			"Account features are not configured",
			http.StatusServiceUnavailable,
		))
		return account.ID{}, false
	}
	principal, ok := requestcontext.Principal(ctx.Request.Context())
	if !ok {
		handler.responder.Error(ctx, apperror.New(
			apperror.CodeAuthentication,
			"Authentication is required",
			http.StatusUnauthorized,
		))
		return account.ID{}, false
	}
	return principal.Session.AccountID, true
}

func (handler AccountHandler) ownedMutationTarget(
	ctx *gin.Context,
) (account.ID, account.ResourceID, int64, bool) {
	accountID, ok := handler.accountID(ctx)
	if !ok {
		return account.ID{}, account.ResourceID{}, 0, false
	}
	rawID := ctx.Param("bookmarkID")
	if rawID == "" {
		rawID = ctx.Param("savedSearchID")
	}
	resourceID, err := account.ParseResourceID(rawID)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return account.ID{}, account.ResourceID{}, 0, false
	}
	version, err := parseRequiredVersion(ctx)
	if err != nil {
		handler.invalidRequest(ctx, err)
		return account.ID{}, account.ResourceID{}, 0, false
	}
	return accountID, resourceID, version, true
}

func (handler AccountHandler) invalidRequest(
	ctx *gin.Context,
	err error,
) {
	if err == nil {
		err = account.ErrInvalidFeatureInput
	}
	handler.responder.Error(ctx, apperror.Wrap(
		apperror.CodeInvalidRequest,
		"Account feature request is invalid",
		http.StatusBadRequest,
		err,
	))
}

type bookmarkWriteRequest struct {
	TargetType      string `json:"targetType"`
	RepositoryOwner string `json:"repositoryOwner"`
	RepositoryName  string `json:"repositoryName"`
	IssueNumber     *int   `json:"issueNumber"`
}

type savedSearchWriteRequest struct {
	SearchType string          `json:"searchType"`
	Name       string          `json:"name"`
	Filters    json.RawMessage `json:"filters"`
}

func (request savedSearchWriteRequest) input() usecase.WriteSavedSearchInput {
	return usecase.WriteSavedSearchInput{
		SearchType: account.SearchType(request.SearchType),
		Name:       request.Name,
		Filters:    request.Filters,
	}
}

type savedSearchUpdateRequest struct {
	SearchType string          `json:"searchType"`
	Name       string          `json:"name"`
	Filters    json.RawMessage `json:"filters"`
	Version    int64           `json:"version"`
}

func (request savedSearchUpdateRequest) input() usecase.WriteSavedSearchInput {
	return usecase.WriteSavedSearchInput{
		SearchType: account.SearchType(request.SearchType),
		Name:       request.Name,
		Filters:    request.Filters,
	}
}

type preferencesWriteRequest struct {
	Theme          string `json:"theme"`
	ReducedMotion  string `json:"reducedMotion"`
	ResultsPerPage int    `json:"resultsPerPage"`
	Version        int64  `json:"version"`
}

type accountDeleteRequest struct {
	Confirmation string `json:"confirmation"`
}

type bookmarkListResponse struct {
	Items      []bookmarkResponse        `json:"items"`
	Pagination accountPaginationResponse `json:"pagination"`
}

type bookmarkResponse struct {
	ID              string                 `json:"id"`
	TargetType      account.BookmarkTarget `json:"targetType"`
	RepositoryOwner string                 `json:"repositoryOwner"`
	RepositoryName  string                 `json:"repositoryName"`
	IssueNumber     *int                   `json:"issueNumber,omitempty"`
	UpstreamState   string                 `json:"upstreamState"`
	Version         int64                  `json:"version"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

func newBookmarkResponse(bookmark account.Bookmark) bookmarkResponse {
	return bookmarkResponse{
		ID:              bookmark.ID.String(),
		TargetType:      bookmark.Reference.TargetType,
		RepositoryOwner: bookmark.Reference.RepositoryOwner,
		RepositoryName:  bookmark.Reference.RepositoryName,
		IssueNumber:     bookmark.Reference.IssueNumber,
		UpstreamState:   "unverified",
		Version:         bookmark.Version,
		CreatedAt:       bookmark.CreatedAt.UTC(),
		UpdatedAt:       bookmark.UpdatedAt.UTC(),
	}
}

type savedSearchListResponse struct {
	Items      []savedSearchResponse     `json:"items"`
	Pagination accountPaginationResponse `json:"pagination"`
}

type savedSearchResponse struct {
	ID         string             `json:"id"`
	SearchType account.SearchType `json:"searchType"`
	Name       string             `json:"name"`
	Filters    json.RawMessage    `json:"filters"`
	Version    int64              `json:"version"`
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
}

func newSavedSearchResponse(savedSearch account.SavedSearch) savedSearchResponse {
	return savedSearchResponse{
		ID:         savedSearch.ID.String(),
		SearchType: savedSearch.SearchType,
		Name:       savedSearch.Name,
		Filters:    savedSearch.Filters,
		Version:    savedSearch.Version,
		CreatedAt:  savedSearch.CreatedAt.UTC(),
		UpdatedAt:  savedSearch.UpdatedAt.UTC(),
	}
}

type preferencesResponse struct {
	Theme          account.Theme         `json:"theme"`
	ReducedMotion  account.ReducedMotion `json:"reducedMotion"`
	ResultsPerPage int                   `json:"resultsPerPage"`
	Version        int64                 `json:"version"`
	CreatedAt      *time.Time            `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time            `json:"updatedAt,omitempty"`
}

func newPreferencesResponse(
	preferences account.Preferences,
) preferencesResponse {
	response := preferencesResponse{
		Theme:          preferences.Theme,
		ReducedMotion:  preferences.ReducedMotion,
		ResultsPerPage: preferences.ResultsPerPage,
		Version:        preferences.Version,
	}
	if preferences.Version > 0 {
		createdAt := preferences.CreatedAt.UTC()
		updatedAt := preferences.UpdatedAt.UTC()
		response.CreatedAt = &createdAt
		response.UpdatedAt = &updatedAt
	}
	return response
}

type accountPaginationResponse struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

func newAccountPagination(
	page account.Page,
	total int,
) accountPaginationResponse {
	totalPages := 0
	if total > 0 {
		totalPages = (total + page.PerPage - 1) / page.PerPage
	}
	return accountPaginationResponse{
		Page:       page.Number,
		PerPage:    page.PerPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

type deletionResponse struct {
	Deleted bool `json:"deleted"`
}

type accountExportResponse struct {
	SchemaVersion int                   `json:"schemaVersion"`
	GeneratedAt   time.Time             `json:"generatedAt"`
	Bookmarks     []bookmarkResponse    `json:"bookmarks"`
	SavedSearches []savedSearchResponse `json:"savedSearches"`
	Preferences   *preferencesResponse  `json:"preferences"`
}

func newAccountExportResponse(export account.Export) accountExportResponse {
	bookmarks := make([]bookmarkResponse, len(export.Bookmarks))
	for index, bookmark := range export.Bookmarks {
		bookmarks[index] = newBookmarkResponse(bookmark)
	}
	savedSearches := make([]savedSearchResponse, len(export.SavedSearches))
	for index, savedSearch := range export.SavedSearches {
		savedSearches[index] = newSavedSearchResponse(savedSearch)
	}
	var preferences *preferencesResponse
	if export.Preferences != nil {
		value := newPreferencesResponse(*export.Preferences)
		preferences = &value
	}
	return accountExportResponse{
		SchemaVersion: 1,
		GeneratedAt:   export.GeneratedAt.UTC(),
		Bookmarks:     bookmarks,
		SavedSearches: savedSearches,
		Preferences:   preferences,
	}
}

type ownedDataSummaryResponse struct {
	Bookmarks     int64 `json:"bookmarks"`
	Identities    int64 `json:"identities"`
	Preferences   int64 `json:"preferences"`
	SavedSearches int64 `json:"savedSearches"`
	Sessions      int64 `json:"sessions"`
}

type accountDeleteResponse struct {
	Deleted bool                     `json:"deleted"`
	Removed ownedDataSummaryResponse `json:"removed"`
}

func decodeAccountBody[T any](ctx *gin.Context) (T, error) {
	return decodeStrictJSONBody[T](ctx, strictJSONOptions{
		description:  "account request",
		maximumBytes: maximumAccountRequestBytes,
	})
}

func parseAccountPage(ctx *gin.Context) (account.Page, error) {
	page, perPage, err := parsePaginationQuery(
		ctx,
		1,
		account.DefaultPageSize,
	)
	if err != nil {
		return account.Page{}, err
	}
	return account.NewPage(page, perPage)
}

func parseRequiredVersion(ctx *gin.Context) (int64, error) {
	query := ctx.Request.URL.Query()
	for key := range query {
		if key != "version" {
			return 0, fmt.Errorf("unsupported query parameter %q", key)
		}
	}
	values := query["version"]
	if len(values) != 1 || values[0] == "" {
		return 0, fmt.Errorf("version must be provided exactly once")
	}
	version, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("version must be a positive integer")
	}
	return version, nil
}
