package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
)

func TestAccountWorkspaceNormalizesBookmarkBeforePersistence(t *testing.T) {
	accountID := workspaceAccountID(t)
	repository := &accountRepositoryStub{}
	service := concreteAccountWorkspace(t, repository)
	service.newID = func() (account.ResourceID, error) {
		return workspaceResourceID(t), nil
	}
	number := 12
	bookmark, err := service.UpsertBookmark(
		context.Background(),
		accountID,
		UpsertBookmarkInput{
			TargetType:      account.BookmarkTargetIssue,
			RepositoryOwner: "OpenAI",
			RepositoryName:  "OpenAI-Go",
			IssueNumber:     &number,
		},
	)
	if err != nil {
		t.Fatalf("UpsertBookmark() error = %v", err)
	}
	if bookmark.Reference.RepositoryOwner != "openai" ||
		bookmark.Reference.RepositoryName != "openai-go" ||
		repository.upsertBookmarkCalls != 1 {
		t.Fatalf("bookmark = %+v", bookmark)
	}

	_, err = service.UpsertBookmark(
		context.Background(),
		accountID,
		UpsertBookmarkInput{TargetType: account.BookmarkTargetIssue},
	)
	assertApplicationError(t, err, apperror.CodeInvalidRequest)
	if repository.upsertBookmarkCalls != 1 {
		t.Fatal("invalid bookmark reached repository")
	}
}

func TestAccountWorkspaceValidatesAndCanonicalizesSavedSearch(t *testing.T) {
	accountID := workspaceAccountID(t)
	repository := &accountRepositoryStub{}
	service := concreteAccountWorkspace(t, repository)
	service.newID = func() (account.ResourceID, error) {
		return workspaceResourceID(t), nil
	}
	saved, err := service.CreateSavedSearch(
		context.Background(),
		accountID,
		WriteSavedSearchInput{
			SearchType: account.SearchTypeIssue,
			Name:       "  Go issues  ",
			Filters:    []byte(`{"username":"octocat","languages":[" Go "]}`),
		},
	)
	if err != nil {
		t.Fatalf("CreateSavedSearch() error = %v", err)
	}
	if saved.Name != "Go issues" ||
		string(saved.Filters) == `{"username":"octocat","languages":[" Go "]}` ||
		repository.createSavedSearchCalls != 1 {
		t.Fatalf("saved search = %+v", saved)
	}

	_, err = service.UpdateSavedSearch(
		context.Background(),
		accountID,
		workspaceResourceID(t),
		0,
		WriteSavedSearchInput{},
	)
	assertApplicationError(t, err, apperror.CodeInvalidRequest)
}

func TestAccountWorkspaceMapsStorageConflictsToStableErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code apperror.Code
	}{
		{name: "not found", err: account.ErrNotFound, code: apperror.CodeNotFound},
		{name: "quota", err: account.ErrQuotaExceeded, code: apperror.CodeAccountQuota},
		{name: "version", err: account.ErrVersionConflict, code: apperror.CodeVersionConflict},
		{name: "duplicate", err: account.ErrDuplicateSavedSearch, code: apperror.CodeDuplicateSavedSearch},
		{name: "database", err: errors.New("secret driver error"), code: apperror.CodeDatabaseUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &accountRepositoryStub{
				deleteBookmarkError: test.err,
			}
			service := NewAccountWorkspace(repository)
			err := service.DeleteBookmark(
				context.Background(),
				workspaceAccountID(t),
				workspaceResourceID(t),
				1,
			)
			assertApplicationError(t, err, test.code)
			if err.Error() == "secret driver error" {
				t.Fatal("storage detail was exposed")
			}
		})
	}
}

func TestAccountWorkspacePreferencesAndExportAreBounded(t *testing.T) {
	accountID := workspaceAccountID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	bookmarks := make([]account.Bookmark, account.MaximumPageSize)
	repository := &accountRepositoryStub{
		listBookmarkResults: []account.PageResult[account.Bookmark]{
			{
				Items: bookmarks,
				Total: account.MaximumPageSize + 1,
			},
			{
				Items: []account.Bookmark{{AccountID: accountID}},
				Total: account.MaximumPageSize + 1,
			},
		},
		listSavedSearchResult: account.PageResult[account.SavedSearch]{
			Items: []account.SavedSearch{{AccountID: accountID}},
			Total: 1,
		},
		preferences: account.Preferences{
			AccountID: accountID,
			Theme:     account.ThemeDark,
			Version:   2,
		},
	}
	service := concreteAccountWorkspace(t, repository)
	service.now = func() time.Time { return now }

	preferences, err := service.UpdatePreferences(
		context.Background(),
		accountID,
		0,
		UpdatePreferencesInput{
			Theme:          account.ThemeDark,
			ReducedMotion:  account.ReducedMotionReduce,
			ResultsPerPage: 50,
		},
	)
	if err != nil || preferences.Theme != account.ThemeDark {
		t.Fatalf("UpdatePreferences() = %+v, %v", preferences, err)
	}
	export, err := service.Export(context.Background(), accountID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(export.Bookmarks) != account.MaximumPageSize+1 ||
		len(export.SavedSearches) != 1 ||
		export.Preferences == nil ||
		!export.GeneratedAt.Equal(now) ||
		repository.listBookmarkCalls != 2 {
		t.Fatalf("export = %+v", export)
	}
}

func TestAccountWorkspaceDeleteReturnsContentFreeSummary(t *testing.T) {
	repository := &accountRepositoryStub{
		summary: account.OwnedDataSummary{
			Bookmarks:     2,
			SavedSearches: 3,
			Sessions:      1,
		},
	}
	service := NewAccountWorkspace(repository)
	summary, err := service.DeleteAccount(
		context.Background(),
		workspaceAccountID(t),
	)
	if err != nil || summary.Bookmarks != 2 || repository.deleteCalls != 1 {
		t.Fatalf("DeleteAccount() = %+v, %v", summary, err)
	}
}

func TestAccountWorkspaceDelegatesOwnedCollectionsAndVersionedSearches(
	t *testing.T,
) {
	accountID := workspaceAccountID(t)
	resourceID := workspaceResourceID(t)
	repository := &accountRepositoryStub{
		listBookmarkResults: []account.PageResult[account.Bookmark]{
			{Items: []account.Bookmark{{AccountID: accountID}}, Total: 1},
		},
		listSavedSearchResult: account.PageResult[account.SavedSearch]{
			Items: []account.SavedSearch{{AccountID: accountID}},
			Total: 1,
		},
		preferences: account.Preferences{
			AccountID: accountID,
			Theme:     account.ThemeSystem,
		},
	}
	service := concreteAccountWorkspace(t, repository)
	service.newID = func() (account.ResourceID, error) {
		return resourceID, nil
	}
	page, _ := account.NewPage(1, 20)
	bookmarks, err := service.ListBookmarks(
		context.Background(),
		accountID,
		page,
	)
	if err != nil || bookmarks.Total != 1 {
		t.Fatalf("ListBookmarks() = %+v, %v", bookmarks, err)
	}
	searches, err := service.ListSavedSearches(
		context.Background(),
		accountID,
		page,
	)
	if err != nil || searches.Total != 1 {
		t.Fatalf("ListSavedSearches() = %+v, %v", searches, err)
	}
	preferences, err := service.GetPreferences(
		context.Background(),
		accountID,
	)
	if err != nil || preferences.Theme != account.ThemeSystem {
		t.Fatalf("GetPreferences() = %+v, %v", preferences, err)
	}
	updated, err := service.UpdateSavedSearch(
		context.Background(),
		accountID,
		resourceID,
		1,
		WriteSavedSearchInput{
			SearchType: account.SearchTypeIssue,
			Name:       "Go",
			Filters:    []byte(`{"username":"octocat"}`),
		},
	)
	if err != nil || updated.Version != 2 || updated.ID != resourceID {
		t.Fatalf("UpdateSavedSearch() = %+v, %v", updated, err)
	}
	if err := service.DeleteSavedSearch(
		context.Background(),
		accountID,
		resourceID,
		2,
	); err != nil {
		t.Fatalf("DeleteSavedSearch() error = %v", err)
	}
}

func TestAccountWorkspaceRejectsMissingRepositoryAndStorageListFailure(
	t *testing.T,
) {
	if service := NewAccountWorkspace(nil); service != nil {
		t.Fatalf("NewAccountWorkspace(nil) = %v", service)
	}
	repository := &accountRepositoryStub{
		listBookmarkError: errors.New("driver unavailable"),
	}
	service := NewAccountWorkspace(repository)
	page, _ := account.NewPage(1, 20)
	_, err := service.ListBookmarks(
		context.Background(),
		workspaceAccountID(t),
		page,
	)
	assertApplicationError(t, err, apperror.CodeDatabaseUnavailable)
}

type accountRepositoryStub struct {
	upsertBookmarkCalls    int
	deleteBookmarkError    error
	listBookmarkCalls      int
	listBookmarkResults    []account.PageResult[account.Bookmark]
	listBookmarkError      error
	createSavedSearchCalls int
	listSavedSearchResult  account.PageResult[account.SavedSearch]
	preferences            account.Preferences
	summary                account.OwnedDataSummary
	deleteCalls            int
}

func (repository *accountRepositoryStub) ListBookmarks(
	_ context.Context,
	_ account.ID,
	page account.Page,
) (account.PageResult[account.Bookmark], error) {
	index := repository.listBookmarkCalls
	repository.listBookmarkCalls++
	if index < len(repository.listBookmarkResults) {
		result := repository.listBookmarkResults[index]
		result.Page = page
		return result, nil
	}
	if repository.listBookmarkError != nil {
		return account.PageResult[account.Bookmark]{},
			repository.listBookmarkError
	}
	return account.PageResult[account.Bookmark]{Page: page}, nil
}

func (repository *accountRepositoryStub) UpsertBookmark(
	_ context.Context,
	bookmark account.Bookmark,
) (account.Bookmark, error) {
	repository.upsertBookmarkCalls++
	bookmark.Version = 1
	return bookmark, nil
}

func (repository *accountRepositoryStub) DeleteBookmark(
	context.Context,
	account.ID,
	account.ResourceID,
	int64,
) error {
	return repository.deleteBookmarkError
}

func (repository *accountRepositoryStub) ListSavedSearches(
	_ context.Context,
	_ account.ID,
	page account.Page,
) (account.PageResult[account.SavedSearch], error) {
	result := repository.listSavedSearchResult
	result.Page = page
	return result, nil
}

func (repository *accountRepositoryStub) CreateSavedSearch(
	_ context.Context,
	savedSearch account.SavedSearch,
) (account.SavedSearch, error) {
	repository.createSavedSearchCalls++
	savedSearch.Version = 1
	return savedSearch, nil
}

func (repository *accountRepositoryStub) UpdateSavedSearch(
	_ context.Context,
	savedSearch account.SavedSearch,
) (account.SavedSearch, error) {
	savedSearch.Version++
	return savedSearch, nil
}

func (repository *accountRepositoryStub) DeleteSavedSearch(
	context.Context,
	account.ID,
	account.ResourceID,
	int64,
) error {
	return nil
}

func (repository *accountRepositoryStub) GetPreferences(
	_ context.Context,
	accountID account.ID,
) (account.Preferences, error) {
	preferences := repository.preferences
	preferences.AccountID = accountID
	return preferences, nil
}

func (repository *accountRepositoryStub) UpsertPreferences(
	_ context.Context,
	preferences account.Preferences,
	_ int64,
) (account.Preferences, error) {
	preferences.Version = 1
	return preferences, nil
}

func (repository *accountRepositoryStub) OwnedDataSummary(
	context.Context,
	account.ID,
) (account.OwnedDataSummary, error) {
	return repository.summary, nil
}

func (repository *accountRepositoryStub) Delete(
	context.Context,
	account.ID,
) error {
	repository.deleteCalls++
	return nil
}

func assertApplicationError(
	t *testing.T,
	err error,
	code apperror.Code,
) {
	t.Helper()
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) ||
		applicationError.Code != code {
		t.Fatalf("application error = %v, want code %s", err, code)
	}
}

func workspaceAccountID(t *testing.T) account.ID {
	t.Helper()
	id, err := account.ParseID("8bbfd7ed-a424-4ec3-a1b8-647006da1816")
	if err != nil {
		t.Fatalf("account.ParseID() error = %v", err)
	}
	return id
}

func workspaceResourceID(t *testing.T) account.ResourceID {
	t.Helper()
	id, err := account.ParseResourceID(
		"69cf232f-f1ba-4c24-9b18-9083f90b1a1a",
	)
	if err != nil {
		t.Fatalf("account.ParseResourceID() error = %v", err)
	}
	return id
}

func concreteAccountWorkspace(
	t *testing.T,
	repository *accountRepositoryStub,
) *accountWorkspace {
	t.Helper()
	service, ok := NewAccountWorkspace(repository).(*accountWorkspace)
	if !ok {
		t.Fatal("NewAccountWorkspace() did not return the concrete service")
	}
	return service
}
