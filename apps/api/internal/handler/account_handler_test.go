package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestAccountHandlerBookmarkCollectionUsesPrincipalOwnership(t *testing.T) {
	accountID := handlerAccountID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	resourceID := handlerResourceID(t)
	number := 42
	reference, _ := account.NewBookmarkReference(
		account.BookmarkTargetIssue,
		"openai",
		"openai-go",
		&number,
	)
	workspace := &accountWorkspaceStub{
		bookmarkPage: account.PageResult[account.Bookmark]{
			Items: []account.Bookmark{{
				ID:        resourceID,
				AccountID: accountID,
				Reference: reference,
				Version:   2,
				CreatedAt: now,
				UpdatedAt: now,
			}},
			Total: 1,
		},
		bookmark: account.Bookmark{
			ID:        resourceID,
			AccountID: accountID,
			Reference: reference,
			Version:   2,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	handler := NewAccountHandler(
		workspace,
		authhttp.NewPolicy(false),
		response.NewResponder(),
	)
	engine := accountTestEngine(accountID)
	engine.GET("/bookmarks", handler.ListBookmarks)
	engine.PUT("/bookmarks", handler.UpsertBookmark)

	listRecorder := httptest.NewRecorder()
	engine.ServeHTTP(
		listRecorder,
		httptest.NewRequest(
			http.MethodGet,
			"/bookmarks?page=1&perPage=10",
			nil,
		),
	)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf(
			"list response = %d %s",
			listRecorder.Code,
			listRecorder.Body.String(),
		)
	}
	for _, fragment := range []string{
		`"targetType":"issue"`,
		`"repositoryOwner":"openai"`,
		`"issueNumber":42`,
		`"upstreamState":"unverified"`,
		`"totalPages":1`,
	} {
		if !strings.Contains(listRecorder.Body.String(), fragment) {
			t.Errorf("list response missing %s", fragment)
		}
	}

	upsertRequest := httptest.NewRequest(
		http.MethodPut,
		"/bookmarks",
		strings.NewReader(
			`{"targetType":"issue","repositoryOwner":"openai",`+
				`"repositoryName":"openai-go","issueNumber":42}`,
		),
	)
	upsertRequest.Header.Set("Content-Type", "application/json")
	upsertRecorder := httptest.NewRecorder()
	engine.ServeHTTP(upsertRecorder, upsertRequest)
	if upsertRecorder.Code != http.StatusOK ||
		workspace.lastAccountID != accountID ||
		workspace.lastBookmarkInput.RepositoryOwner != "openai" {
		t.Fatalf(
			"upsert response = %d %s, workspace = %+v",
			upsertRecorder.Code,
			upsertRecorder.Body.String(),
			workspace,
		)
	}
}

func TestAccountHandlerSavedSearchMutationsAreStrictAndVersioned(
	t *testing.T,
) {
	accountID := handlerAccountID(t)
	resourceID := handlerResourceID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	workspace := &accountWorkspaceStub{savedSearch: account.SavedSearch{
		ID:         resourceID,
		AccountID:  accountID,
		SearchType: account.SearchTypeIssue,
		Name:       "Go",
		Filters:    []byte(`{"username":"octocat"}`),
		Version:    3,
		CreatedAt:  now,
		UpdatedAt:  now,
	}}
	handler := NewAccountHandler(
		workspace,
		authhttp.NewPolicy(false),
		response.NewResponder(),
	)
	engine := accountTestEngine(accountID)
	engine.POST("/saved", handler.CreateSavedSearch)
	engine.PUT("/saved/:savedSearchID", handler.UpdateSavedSearch)
	engine.DELETE("/saved/:savedSearchID", handler.DeleteSavedSearch)

	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/saved",
		strings.NewReader(
			`{"searchType":"issue","name":"Go",`+
				`"filters":{"username":"octocat"}}`,
		),
	)
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	engine.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated ||
		workspace.lastSavedInput.Name != "Go" {
		t.Fatalf(
			"create response = %d %s",
			createRecorder.Code,
			createRecorder.Body.String(),
		)
	}

	updateRequest := httptest.NewRequest(
		http.MethodPut,
		"/saved/"+resourceID.String(),
		strings.NewReader(
			`{"searchType":"issue","name":"Go",`+
				`"filters":{"username":"octocat"},"version":2}`,
		),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	engine.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK ||
		workspace.lastVersion != 2 ||
		workspace.lastResourceID != resourceID {
		t.Fatalf(
			"update response = %d %s",
			updateRecorder.Code,
			updateRecorder.Body.String(),
		)
	}

	deleteRecorder := httptest.NewRecorder()
	engine.ServeHTTP(
		deleteRecorder,
		httptest.NewRequest(
			http.MethodDelete,
			"/saved/"+resourceID.String()+"?version=3",
			nil,
		),
	)
	if deleteRecorder.Code != http.StatusOK ||
		!strings.Contains(deleteRecorder.Body.String(), `"deleted":true`) ||
		workspace.lastVersion != 3 {
		t.Fatalf(
			"delete response = %d %s",
			deleteRecorder.Code,
			deleteRecorder.Body.String(),
		)
	}

	invalidRequest := httptest.NewRequest(
		http.MethodPost,
		"/saved",
		strings.NewReader(
			`{"searchType":"issue","name":"Go","filters":{},`+
				`"unexpected":true}`,
		),
	)
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidRecorder := httptest.NewRecorder()
	engine.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusBadRequest ||
		!strings.Contains(invalidRecorder.Body.String(), "INVALID_REQUEST") {
		t.Fatalf(
			"invalid response = %d %s",
			invalidRecorder.Code,
			invalidRecorder.Body.String(),
		)
	}
}

func TestAccountHandlerPreferencesExportAndDeletion(t *testing.T) {
	accountID := handlerAccountID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	workspace := &accountWorkspaceStub{
		preferences: account.Preferences{
			AccountID:      accountID,
			Theme:          account.ThemeDark,
			ReducedMotion:  account.ReducedMotionReduce,
			ResultsPerPage: 50,
			Version:        2,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		export: account.Export{GeneratedAt: now},
		summary: account.OwnedDataSummary{
			Bookmarks: 2,
			Sessions:  1,
		},
	}
	policy := authhttp.NewPolicy(false)
	handler := NewAccountHandler(
		workspace,
		policy,
		response.NewResponder(),
	)
	engine := accountTestEngine(accountID)
	engine.GET("/preferences", handler.GetPreferences)
	engine.PUT("/preferences", handler.UpdatePreferences)
	engine.GET("/export", handler.Export)
	engine.DELETE("/account", handler.DeleteAccount)

	preferenceRequest := httptest.NewRequest(
		http.MethodPut,
		"/preferences",
		strings.NewReader(
			`{"theme":"dark","reducedMotion":"reduce",`+
				`"resultsPerPage":50,"version":1}`,
		),
	)
	preferenceRequest.Header.Set("Content-Type", "application/json")
	preferenceRecorder := httptest.NewRecorder()
	engine.ServeHTTP(preferenceRecorder, preferenceRequest)
	if preferenceRecorder.Code != http.StatusOK ||
		workspace.lastVersion != 1 ||
		workspace.lastPreferencesInput.ResultsPerPage != 50 {
		t.Fatalf(
			"preferences response = %d %s",
			preferenceRecorder.Code,
			preferenceRecorder.Body.String(),
		)
	}

	exportRecorder := httptest.NewRecorder()
	engine.ServeHTTP(
		exportRecorder,
		httptest.NewRequest(http.MethodGet, "/export", nil),
	)
	if exportRecorder.Code != http.StatusOK ||
		!strings.Contains(exportRecorder.Body.String(), `"schemaVersion":1`) ||
		!strings.Contains(exportRecorder.Body.String(), `"preferences":null`) {
		t.Fatalf(
			"export response = %d %s",
			exportRecorder.Code,
			exportRecorder.Body.String(),
		)
	}

	deleteRequest := httptest.NewRequest(
		http.MethodDelete,
		"/account",
		strings.NewReader(`{"confirmation":"DELETE"}`),
	)
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRecorder := httptest.NewRecorder()
	engine.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK ||
		!strings.Contains(deleteRecorder.Body.String(), `"bookmarks":2`) ||
		!strings.Contains(
			deleteRecorder.Header().Values("Set-Cookie")[0],
			policy.Names().Session,
		) {
		t.Fatalf(
			"delete response = %d %s, cookies = %v",
			deleteRecorder.Code,
			deleteRecorder.Body.String(),
			deleteRecorder.Header().Values("Set-Cookie"),
		)
	}
}

func TestAccountHandlerRejectsMissingPrincipalAndInvalidTarget(t *testing.T) {
	workspace := &accountWorkspaceStub{}
	handler := NewAccountHandler(
		workspace,
		authhttp.NewPolicy(false),
		response.NewResponder(),
	)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.DELETE("/bookmarks/:bookmarkID", handler.DeleteBookmark)

	missingPrincipal := httptest.NewRecorder()
	engine.ServeHTTP(
		missingPrincipal,
		httptest.NewRequest(
			http.MethodDelete,
			"/bookmarks/not-a-uuid?version=1",
			nil,
		),
	)
	if missingPrincipal.Code != http.StatusUnauthorized {
		t.Fatalf("missing principal status = %d", missingPrincipal.Code)
	}

	principalEngine := accountTestEngine(handlerAccountID(t))
	principalEngine.DELETE(
		"/bookmarks/:bookmarkID",
		handler.DeleteBookmark,
	)
	invalidTarget := httptest.NewRecorder()
	principalEngine.ServeHTTP(
		invalidTarget,
		httptest.NewRequest(
			http.MethodDelete,
			"/bookmarks/not-a-uuid?version=1",
			nil,
		),
	)
	if invalidTarget.Code != http.StatusBadRequest {
		t.Fatalf("invalid target status = %d", invalidTarget.Code)
	}
}

func accountTestEngine(accountID account.ID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Request = ctx.Request.WithContext(
			requestcontext.WithPrincipal(
				ctx.Request.Context(),
				auth.Principal{Session: auth.Session{AccountID: accountID}},
			),
		)
		ctx.Next()
	})
	return engine
}

type accountWorkspaceStub struct {
	lastAccountID        account.ID
	lastResourceID       account.ResourceID
	lastVersion          int64
	lastBookmarkInput    usecase.UpsertBookmarkInput
	lastSavedInput       usecase.WriteSavedSearchInput
	lastPreferencesInput usecase.UpdatePreferencesInput
	bookmarkPage         account.PageResult[account.Bookmark]
	bookmark             account.Bookmark
	savedSearchPage      account.PageResult[account.SavedSearch]
	savedSearch          account.SavedSearch
	preferences          account.Preferences
	export               account.Export
	summary              account.OwnedDataSummary
}

func (workspace *accountWorkspaceStub) ListBookmarks(
	_ context.Context,
	accountID account.ID,
	page account.Page,
) (account.PageResult[account.Bookmark], error) {
	workspace.lastAccountID = accountID
	result := workspace.bookmarkPage
	result.Page = page
	return result, nil
}

func (workspace *accountWorkspaceStub) UpsertBookmark(
	_ context.Context,
	accountID account.ID,
	input usecase.UpsertBookmarkInput,
) (account.Bookmark, error) {
	workspace.lastAccountID = accountID
	workspace.lastBookmarkInput = input
	return workspace.bookmark, nil
}

func (workspace *accountWorkspaceStub) DeleteBookmark(
	_ context.Context,
	accountID account.ID,
	resourceID account.ResourceID,
	version int64,
) error {
	workspace.recordTarget(accountID, resourceID, version)
	return nil
}

func (workspace *accountWorkspaceStub) ListSavedSearches(
	_ context.Context,
	accountID account.ID,
	page account.Page,
) (account.PageResult[account.SavedSearch], error) {
	workspace.lastAccountID = accountID
	result := workspace.savedSearchPage
	result.Page = page
	return result, nil
}

func (workspace *accountWorkspaceStub) CreateSavedSearch(
	_ context.Context,
	accountID account.ID,
	input usecase.WriteSavedSearchInput,
) (account.SavedSearch, error) {
	workspace.lastAccountID = accountID
	workspace.lastSavedInput = input
	return workspace.savedSearch, nil
}

func (workspace *accountWorkspaceStub) UpdateSavedSearch(
	_ context.Context,
	accountID account.ID,
	resourceID account.ResourceID,
	version int64,
	input usecase.WriteSavedSearchInput,
) (account.SavedSearch, error) {
	workspace.recordTarget(accountID, resourceID, version)
	workspace.lastSavedInput = input
	return workspace.savedSearch, nil
}

func (workspace *accountWorkspaceStub) DeleteSavedSearch(
	_ context.Context,
	accountID account.ID,
	resourceID account.ResourceID,
	version int64,
) error {
	workspace.recordTarget(accountID, resourceID, version)
	return nil
}

func (workspace *accountWorkspaceStub) GetPreferences(
	_ context.Context,
	accountID account.ID,
) (account.Preferences, error) {
	workspace.lastAccountID = accountID
	return workspace.preferences, nil
}

func (workspace *accountWorkspaceStub) UpdatePreferences(
	_ context.Context,
	accountID account.ID,
	version int64,
	input usecase.UpdatePreferencesInput,
) (account.Preferences, error) {
	workspace.lastAccountID = accountID
	workspace.lastVersion = version
	workspace.lastPreferencesInput = input
	return workspace.preferences, nil
}

func (workspace *accountWorkspaceStub) Export(
	_ context.Context,
	accountID account.ID,
) (account.Export, error) {
	workspace.lastAccountID = accountID
	return workspace.export, nil
}

func (workspace *accountWorkspaceStub) DeleteAccount(
	_ context.Context,
	accountID account.ID,
) (account.OwnedDataSummary, error) {
	workspace.lastAccountID = accountID
	return workspace.summary, nil
}

func (workspace *accountWorkspaceStub) recordTarget(
	accountID account.ID,
	resourceID account.ResourceID,
	version int64,
) {
	workspace.lastAccountID = accountID
	workspace.lastResourceID = resourceID
	workspace.lastVersion = version
}

func handlerAccountID(t *testing.T) account.ID {
	t.Helper()
	id, err := account.ParseID("8bbfd7ed-a424-4ec3-a1b8-647006da1816")
	if err != nil {
		t.Fatalf("account.ParseID() error = %v", err)
	}
	return id
}

func handlerResourceID(t *testing.T) account.ResourceID {
	t.Helper()
	id, err := account.ParseResourceID(
		"69cf232f-f1ba-4c24-9b18-9083f90b1a1a",
	)
	if err != nil {
		t.Fatalf("account.ParseResourceID() error = %v", err)
	}
	return id
}
