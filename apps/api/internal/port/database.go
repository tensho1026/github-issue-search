package port

import (
	"context"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
)

// DatabaseHealth probes the optional authenticated-feature database.
type DatabaseHealth interface {
	Ping(context.Context) error
}

// AccountRepository owns account-scoped persistence and deletion semantics.
type AccountRepository interface {
	ListBookmarks(
		ctx context.Context,
		accountID account.ID,
		page account.Page,
	) (account.PageResult[account.Bookmark], error)
	UpsertBookmark(
		ctx context.Context,
		bookmark account.Bookmark,
	) (account.Bookmark, error)
	DeleteBookmark(
		ctx context.Context,
		accountID account.ID,
		bookmarkID account.ResourceID,
		version int64,
	) error
	ListSavedSearches(
		ctx context.Context,
		accountID account.ID,
		page account.Page,
	) (account.PageResult[account.SavedSearch], error)
	CreateSavedSearch(
		ctx context.Context,
		savedSearch account.SavedSearch,
	) (account.SavedSearch, error)
	UpdateSavedSearch(
		ctx context.Context,
		savedSearch account.SavedSearch,
	) (account.SavedSearch, error)
	DeleteSavedSearch(
		ctx context.Context,
		accountID account.ID,
		savedSearchID account.ResourceID,
		version int64,
	) error
	GetPreferences(
		ctx context.Context,
		accountID account.ID,
	) (account.Preferences, error)
	UpsertPreferences(
		ctx context.Context,
		preferences account.Preferences,
		expectedVersion int64,
	) (account.Preferences, error)
	OwnedDataSummary(
		ctx context.Context,
		accountID account.ID,
	) (account.OwnedDataSummary, error)
	Delete(ctx context.Context, accountID account.ID) error
}
