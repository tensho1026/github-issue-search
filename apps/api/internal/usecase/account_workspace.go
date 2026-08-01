package usecase

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// AccountWorkspace exposes persistence only for an authenticated account.
// Anonymous profile, repository, and issue use cases never depend on it.
type AccountWorkspace interface {
	// ListBookmarks returns one account-owned page and never accepts account
	// identity from an HTTP request body or URL.
	ListBookmarks(
		context.Context,
		account.ID,
		account.Page,
	) (account.PageResult[account.Bookmark], error)
	// UpsertBookmark validates and canonicalizes the reference before performing
	// an idempotent account-owned write.
	UpsertBookmark(
		context.Context,
		account.ID,
		UpsertBookmarkInput,
	) (account.Bookmark, error)
	// DeleteBookmark requires the authenticated account, resource ID, and
	// optimistic version.
	DeleteBookmark(
		context.Context,
		account.ID,
		account.ResourceID,
		int64,
	) error
	// ListSavedSearches returns one account-owned page with canonical JSON
	// filters.
	ListSavedSearches(
		context.Context,
		account.ID,
		account.Page,
	) (account.PageResult[account.SavedSearch], error)
	// CreateSavedSearch validates name, type, size, keys, and canonical JSON
	// before persistence.
	CreateSavedSearch(
		context.Context,
		account.ID,
		WriteSavedSearchInput,
	) (account.SavedSearch, error)
	// UpdateSavedSearch validates replacement content and requires the current
	// optimistic version.
	UpdateSavedSearch(
		context.Context,
		account.ID,
		account.ResourceID,
		int64,
		WriteSavedSearchInput,
	) (account.SavedSearch, error)
	// DeleteSavedSearch requires the authenticated account, resource ID, and
	// optimistic version.
	DeleteSavedSearch(
		context.Context,
		account.ID,
		account.ResourceID,
		int64,
	) error
	// GetPreferences returns persisted account preferences or the domain default.
	GetPreferences(
		context.Context,
		account.ID,
	) (account.Preferences, error)
	// UpdatePreferences validates the closed preference vocabulary and requires
	// the expected version.
	UpdatePreferences(
		context.Context,
		account.ID,
		int64,
		UpdatePreferencesInput,
	) (account.Preferences, error)
	// Export returns a bounded ownership-isolated snapshot without authentication
	// credentials or account identifiers from another principal.
	Export(context.Context, account.ID) (account.Export, error)
	// DeleteAccount returns content-free deletion counts after atomically
	// removing the authenticated account and owned data.
	DeleteAccount(
		context.Context,
		account.ID,
	) (account.OwnedDataSummary, error)
}

// UpsertBookmarkInput contains an untrusted normalized-reference candidate.
type UpsertBookmarkInput struct {
	TargetType      account.BookmarkTarget
	RepositoryOwner string
	RepositoryName  string
	IssueNumber     *int
}

// WriteSavedSearchInput contains an untrusted named filter document.
type WriteSavedSearchInput struct {
	SearchType account.SearchType
	Name       string
	Filters    []byte
}

// UpdatePreferencesInput contains an untrusted display-preference candidate.
type UpdatePreferencesInput struct {
	Theme          account.Theme
	ReducedMotion  account.ReducedMotion
	ResultsPerPage int
}

type accountWorkspace struct {
	repository port.AccountRepository
	newID      func() (account.ResourceID, error)
	now        func() time.Time
}

// NewAccountWorkspace composes account-owned persistence with domain
// validation, safe error translation, and bounded export orchestration.
func NewAccountWorkspace(
	repository port.AccountRepository,
) AccountWorkspace {
	if repository == nil {
		return nil
	}
	return &accountWorkspace{
		repository: repository,
		newID:      account.NewResourceID,
		now:        time.Now,
	}
}

func (service *accountWorkspace) ListBookmarks(
	ctx context.Context,
	accountID account.ID,
	page account.Page,
) (account.PageResult[account.Bookmark], error) {
	result, err := service.repository.ListBookmarks(ctx, accountID, page)
	if err != nil {
		return account.PageResult[account.Bookmark]{}, accountStorageError(err)
	}
	return result, nil
}

func (service *accountWorkspace) UpsertBookmark(
	ctx context.Context,
	accountID account.ID,
	input UpsertBookmarkInput,
) (account.Bookmark, error) {
	reference, err := account.NewBookmarkReference(
		input.TargetType,
		input.RepositoryOwner,
		input.RepositoryName,
		input.IssueNumber,
	)
	if err != nil {
		return account.Bookmark{}, invalidAccountInput(err)
	}
	id, err := service.newID()
	if err != nil {
		return account.Bookmark{}, apperror.Wrap(
			apperror.CodeInternal,
			"An unexpected error occurred",
			http.StatusInternalServerError,
			err,
		)
	}
	result, err := service.repository.UpsertBookmark(ctx, account.Bookmark{
		ID:        id,
		AccountID: accountID,
		Reference: reference,
	})
	if err != nil {
		return account.Bookmark{}, accountStorageError(err)
	}
	return result, nil
}

func (service *accountWorkspace) DeleteBookmark(
	ctx context.Context,
	accountID account.ID,
	bookmarkID account.ResourceID,
	version int64,
) error {
	if version < 1 {
		return invalidAccountInput(account.ErrInvalidFeatureInput)
	}
	return accountStorageErrorOrNil(service.repository.DeleteBookmark(
		ctx,
		accountID,
		bookmarkID,
		version,
	))
}

func (service *accountWorkspace) ListSavedSearches(
	ctx context.Context,
	accountID account.ID,
	page account.Page,
) (account.PageResult[account.SavedSearch], error) {
	result, err := service.repository.ListSavedSearches(ctx, accountID, page)
	if err != nil {
		return account.PageResult[account.SavedSearch]{}, accountStorageError(err)
	}
	return result, nil
}

func (service *accountWorkspace) CreateSavedSearch(
	ctx context.Context,
	accountID account.ID,
	input WriteSavedSearchInput,
) (account.SavedSearch, error) {
	savedSearch, err := service.newSavedSearch(accountID, input)
	if err != nil {
		return account.SavedSearch{}, err
	}
	result, err := service.repository.CreateSavedSearch(ctx, savedSearch)
	if err != nil {
		return account.SavedSearch{}, accountStorageError(err)
	}
	return result, nil
}

func (service *accountWorkspace) UpdateSavedSearch(
	ctx context.Context,
	accountID account.ID,
	savedSearchID account.ResourceID,
	version int64,
	input WriteSavedSearchInput,
) (account.SavedSearch, error) {
	if version < 1 {
		return account.SavedSearch{}, invalidAccountInput(
			account.ErrInvalidFeatureInput,
		)
	}
	savedSearch, err := service.newSavedSearch(accountID, input)
	if err != nil {
		return account.SavedSearch{}, err
	}
	savedSearch.ID = savedSearchID
	savedSearch.Version = version
	result, err := service.repository.UpdateSavedSearch(ctx, savedSearch)
	if err != nil {
		return account.SavedSearch{}, accountStorageError(err)
	}
	return result, nil
}

func (service *accountWorkspace) DeleteSavedSearch(
	ctx context.Context,
	accountID account.ID,
	savedSearchID account.ResourceID,
	version int64,
) error {
	if version < 1 {
		return invalidAccountInput(account.ErrInvalidFeatureInput)
	}
	return accountStorageErrorOrNil(service.repository.DeleteSavedSearch(
		ctx,
		accountID,
		savedSearchID,
		version,
	))
}

func (service *accountWorkspace) GetPreferences(
	ctx context.Context,
	accountID account.ID,
) (account.Preferences, error) {
	preferences, err := service.repository.GetPreferences(ctx, accountID)
	if err != nil {
		return account.Preferences{}, accountStorageError(err)
	}
	return preferences, nil
}

func (service *accountWorkspace) UpdatePreferences(
	ctx context.Context,
	accountID account.ID,
	expectedVersion int64,
	input UpdatePreferencesInput,
) (account.Preferences, error) {
	if expectedVersion < 0 {
		return account.Preferences{}, invalidAccountInput(
			account.ErrInvalidFeatureInput,
		)
	}
	preferences, err := account.NewPreferences(
		input.Theme,
		input.ReducedMotion,
		input.ResultsPerPage,
	)
	if err != nil {
		return account.Preferences{}, invalidAccountInput(err)
	}
	preferences.AccountID = accountID
	result, err := service.repository.UpsertPreferences(
		ctx,
		preferences,
		expectedVersion,
	)
	if err != nil {
		return account.Preferences{}, accountStorageError(err)
	}
	return result, nil
}

func (service *accountWorkspace) Export(
	ctx context.Context,
	accountID account.ID,
) (account.Export, error) {
	bookmarks := make([]account.Bookmark, 0, account.MaximumBookmarks)
	for pageNumber := 1; ; pageNumber++ {
		page, _ := account.NewPage(pageNumber, account.MaximumPageSize)
		result, err := service.repository.ListBookmarks(ctx, accountID, page)
		if err != nil {
			return account.Export{}, accountStorageError(err)
		}
		bookmarks = append(bookmarks, result.Items...)
		if len(bookmarks) >= result.Total ||
			len(result.Items) < account.MaximumPageSize {
			break
		}
	}
	savedPage, _ := account.NewPage(1, account.MaximumPageSize)
	savedSearches, err := service.repository.ListSavedSearches(
		ctx,
		accountID,
		savedPage,
	)
	if err != nil {
		return account.Export{}, accountStorageError(err)
	}
	preferences, err := service.repository.GetPreferences(ctx, accountID)
	if err != nil {
		return account.Export{}, accountStorageError(err)
	}
	var persistedPreferences *account.Preferences
	if preferences.Version > 0 {
		copy := preferences
		persistedPreferences = &copy
	}
	return account.Export{
		GeneratedAt:   service.now().UTC(),
		Bookmarks:     bookmarks,
		SavedSearches: savedSearches.Items,
		Preferences:   persistedPreferences,
	}, nil
}

func (service *accountWorkspace) DeleteAccount(
	ctx context.Context,
	accountID account.ID,
) (account.OwnedDataSummary, error) {
	summary, err := service.repository.OwnedDataSummary(ctx, accountID)
	if err != nil {
		return account.OwnedDataSummary{}, accountStorageError(err)
	}
	if err := service.repository.Delete(ctx, accountID); err != nil {
		return account.OwnedDataSummary{}, accountStorageError(err)
	}
	return summary, nil
}

func (service *accountWorkspace) newSavedSearch(
	accountID account.ID,
	input WriteSavedSearchInput,
) (account.SavedSearch, error) {
	name, err := account.NormalizeSavedSearchName(input.Name)
	if err != nil {
		return account.SavedSearch{}, invalidAccountInput(err)
	}
	filters, err := account.NormalizeSavedSearchFilters(
		input.SearchType,
		input.Filters,
	)
	if err != nil {
		return account.SavedSearch{}, invalidAccountInput(err)
	}
	id, err := service.newID()
	if err != nil {
		return account.SavedSearch{}, apperror.Wrap(
			apperror.CodeInternal,
			"An unexpected error occurred",
			http.StatusInternalServerError,
			err,
		)
	}
	return account.SavedSearch{
		ID:         id,
		AccountID:  accountID,
		SearchType: input.SearchType,
		Name:       name,
		Filters:    filters,
	}, nil
}

func invalidAccountInput(cause error) error {
	return apperror.Wrap(
		apperror.CodeInvalidRequest,
		"Account feature request is invalid",
		http.StatusBadRequest,
		cause,
	)
}

func accountStorageErrorOrNil(err error) error {
	if err == nil {
		return nil
	}
	return accountStorageError(err)
}

func accountStorageError(err error) error {
	switch {
	case errors.Is(err, account.ErrNotFound):
		return apperror.Wrap(
			apperror.CodeNotFound,
			"The requested account resource was not found",
			http.StatusNotFound,
			err,
		)
	case errors.Is(err, account.ErrQuotaExceeded):
		return apperror.Wrap(
			apperror.CodeAccountQuota,
			"The account feature quota has been reached",
			http.StatusConflict,
			err,
		)
	case errors.Is(err, account.ErrVersionConflict):
		return apperror.Wrap(
			apperror.CodeVersionConflict,
			"The account resource changed; reload it and retry",
			http.StatusConflict,
			err,
		)
	case errors.Is(err, account.ErrDuplicateSavedSearch):
		return apperror.Wrap(
			apperror.CodeDuplicateSavedSearch,
			"A saved search with this name already exists",
			http.StatusConflict,
			err,
		)
	default:
		return apperror.Wrap(
			apperror.CodeDatabaseUnavailable,
			"Account storage is temporarily unavailable",
			http.StatusServiceUnavailable,
			err,
		)
	}
}
