package port

import (
	"context"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
)

// DatabaseHealth probes the optional authenticated-feature database.
type DatabaseHealth interface {
	// Ping verifies database reachability while honoring the caller deadline.
	Ping(context.Context) error
}

// AccountRepository owns account-scoped persistence and deletion semantics.
type AccountRepository interface {
	// ListBookmarks returns one stable account-owned page and total count.
	ListBookmarks(
		ctx context.Context,
		accountID account.ID,
		page account.Page,
	) (account.PageResult[account.Bookmark], error)
	// UpsertBookmark inserts or returns the canonical account-owned reference
	// without allowing a cross-account conflict target.
	UpsertBookmark(
		ctx context.Context,
		bookmark account.Bookmark,
	) (account.Bookmark, error)
	// DeleteBookmark removes only the matching account, resource, and optimistic
	// version tuple.
	DeleteBookmark(
		ctx context.Context,
		accountID account.ID,
		bookmarkID account.ResourceID,
		version int64,
	) error
	// ListSavedSearches returns one stable account-owned page and total count.
	ListSavedSearches(
		ctx context.Context,
		accountID account.ID,
		page account.Page,
	) (account.PageResult[account.SavedSearch], error)
	// CreateSavedSearch inserts one normalized account-owned filter document.
	CreateSavedSearch(
		ctx context.Context,
		savedSearch account.SavedSearch,
	) (account.SavedSearch, error)
	// UpdateSavedSearch replaces one account-owned document only when its
	// optimistic version matches.
	UpdateSavedSearch(
		ctx context.Context,
		savedSearch account.SavedSearch,
	) (account.SavedSearch, error)
	// DeleteSavedSearch removes only the matching account, resource, and
	// optimistic version tuple.
	DeleteSavedSearch(
		ctx context.Context,
		accountID account.ID,
		savedSearchID account.ResourceID,
		version int64,
	) error
	// GetPreferences returns the account's persisted preferences or a documented
	// zero-version default.
	GetPreferences(
		ctx context.Context,
		accountID account.ID,
	) (account.Preferences, error)
	// UpsertPreferences creates or updates preferences only when expectedVersion
	// matches the account-owned row.
	UpsertPreferences(
		ctx context.Context,
		preferences account.Preferences,
		expectedVersion int64,
	) (account.Preferences, error)
	// OwnedDataSummary returns content-free account-owned row counts.
	OwnedDataSummary(
		ctx context.Context,
		accountID account.ID,
	) (account.OwnedDataSummary, error)
	// Delete atomically removes the account and every owned dependent row.
	Delete(ctx context.Context, accountID account.ID) error
}
