package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
)

func TestAccountFeaturesAgainstConfiguredPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool := isolatedIntegrationPool(t, ctx, databaseURL)
	if err := pool.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, err := NewAccountRepository(pool)
	if err != nil {
		t.Fatalf("NewAccountRepository() error = %v", err)
	}
	firstAccount := integrationAccountID(
		t,
		"93b9af3a-b1a8-42c8-bff8-55f8af59f5be",
	)
	secondAccount := integrationAccountID(
		t,
		"90d3f3fd-274f-40c2-b6d8-75359ddb9f43",
	)
	if _, insertErr := pool.client.Exec(
		ctx,
		"INSERT INTO accounts (id) VALUES ($1), ($2)",
		firstAccount.String(),
		secondAccount.String(),
	); insertErr != nil {
		var postgresError *pgconn.PgError
		if errors.As(insertErr, &postgresError) {
			t.Fatalf(
				"insert isolated accounts: PostgreSQL code %s constraint %s",
				postgresError.Code,
				postgresError.ConstraintName,
			)
		}
		t.Fatal("insert isolated accounts")
	}

	number := 42
	reference, err := account.NewBookmarkReference(
		account.BookmarkTargetIssue,
		"OpenAI",
		"OpenAI-Go",
		&number,
	)
	if err != nil {
		t.Fatalf("NewBookmarkReference() error = %v", err)
	}
	bookmarkID := integrationResourceID(
		t,
		"69cf232f-f1ba-4c24-9b18-9083f90b1a1a",
	)
	createdBookmark, err := repository.UpsertBookmark(
		ctx,
		account.Bookmark{
			ID:        bookmarkID,
			AccountID: firstAccount,
			Reference: reference,
		},
	)
	if err != nil {
		assertBookmarkSQLDiagnostic(
			t,
			ctx,
			pool,
			firstAccount,
			reference,
		)
		t.Fatalf("UpsertBookmark() error = %v", err)
	}
	duplicateBookmark, err := repository.UpsertBookmark(
		ctx,
		account.Bookmark{
			ID: integrationResourceID(
				t,
				"4821383d-858b-4567-88fb-cbf8d779481e",
			),
			AccountID: firstAccount,
			Reference: reference,
		},
	)
	if err != nil ||
		duplicateBookmark.ID != createdBookmark.ID ||
		duplicateBookmark.Version != createdBookmark.Version {
		t.Fatalf(
			"duplicate UpsertBookmark() = %+v, %v",
			duplicateBookmark,
			err,
		)
	}
	page, _ := account.NewPage(1, account.DefaultPageSize)
	firstBookmarks, err := repository.ListBookmarks(ctx, firstAccount, page)
	if err != nil || firstBookmarks.Total != 1 {
		t.Fatalf("first ListBookmarks() = %+v, %v", firstBookmarks, err)
	}
	secondBookmarks, err := repository.ListBookmarks(ctx, secondAccount, page)
	if err != nil || secondBookmarks.Total != 0 || len(secondBookmarks.Items) != 0 {
		t.Fatalf("second ListBookmarks() = %+v, %v", secondBookmarks, err)
	}
	if deleteErr := repository.DeleteBookmark(
		ctx,
		secondAccount,
		createdBookmark.ID,
		createdBookmark.Version,
	); !errors.Is(deleteErr, account.ErrNotFound) {
		t.Fatalf("cross-account DeleteBookmark() error = %v", deleteErr)
	}

	savedSearchID := integrationResourceID(
		t,
		"c718f6dd-8ea6-4af0-8363-b09b733463ac",
	)
	createdSearch, err := repository.CreateSavedSearch(
		ctx,
		account.SavedSearch{
			ID:         savedSearchID,
			AccountID:  firstAccount,
			SearchType: account.SearchTypeIssue,
			Name:       "Go starter issues",
			Filters:    []byte(`{"username":"octocat"}`),
		},
	)
	if err != nil {
		t.Fatalf("CreateSavedSearch() error = %v", err)
	}
	crossAccountUpdate := createdSearch
	crossAccountUpdate.AccountID = secondAccount
	if _, updateErr := repository.UpdateSavedSearch(
		ctx,
		crossAccountUpdate,
	); !errors.Is(updateErr, account.ErrNotFound) {
		t.Fatalf("cross-account UpdateSavedSearch() error = %v", updateErr)
	}
	staleUpdate := createdSearch
	staleUpdate.Version++
	if _, updateErr := repository.UpdateSavedSearch(
		ctx,
		staleUpdate,
	); !errors.Is(updateErr, account.ErrVersionConflict) {
		t.Fatalf("stale UpdateSavedSearch() error = %v", updateErr)
	}

	preferences, err := account.NewPreferences(
		account.ThemeDark,
		account.ReducedMotionReduce,
		50,
	)
	if err != nil {
		t.Fatalf("NewPreferences() error = %v", err)
	}
	preferences.AccountID = firstAccount
	persistedPreferences, err := repository.UpsertPreferences(
		ctx,
		preferences,
		0,
	)
	if err != nil || persistedPreferences.Version != 1 {
		t.Fatalf(
			"UpsertPreferences() = %+v, %v",
			persistedPreferences,
			err,
		)
	}
	if _, preferenceErr := repository.UpsertPreferences(
		ctx,
		preferences,
		0,
	); !errors.Is(preferenceErr, account.ErrVersionConflict) {
		t.Fatalf("stale UpsertPreferences() error = %v", preferenceErr)
	}

	summary, err := repository.OwnedDataSummary(ctx, firstAccount)
	if err != nil ||
		summary.Bookmarks != 1 ||
		summary.SavedSearches != 1 ||
		summary.Preferences != 1 {
		t.Fatalf("OwnedDataSummary() = %+v, %v", summary, err)
	}
	if err := repository.Delete(ctx, firstAccount); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	var accountRows int
	var auditRows int
	if err := pool.client.QueryRow(
		ctx,
		"SELECT count(*) FROM accounts WHERE id = $1",
		firstAccount.String(),
	).Scan(&accountRows); err != nil {
		t.Fatal("count deleted account")
	}
	if err := pool.client.QueryRow(
		ctx,
		`SELECT count(*) FROM privacy_audit_events
		 WHERE event_type = 'account_deleted' AND account_id IS NULL`,
	).Scan(&auditRows); err != nil {
		t.Fatal("count privacy-safe deletion events")
	}
	if accountRows != 0 || auditRows != 1 {
		t.Fatalf(
			"post-delete accounts = %d, deletion events = %d",
			accountRows,
			auditRows,
		)
	}
}

func assertBookmarkSQLDiagnostic(
	t *testing.T,
	ctx context.Context,
	pool *Pool,
	accountID account.ID,
	reference account.BookmarkReference,
) {
	t.Helper()
	var rawID string
	var rawTarget string
	var owner string
	var repositoryName string
	var issueNumber *int
	var version int64
	var createdAt time.Time
	var updatedAt time.Time
	err := pool.client.QueryRow(
		ctx,
		upsertBookmarkSQL,
		accountID.String(),
		"4821383d-858b-4567-88fb-cbf8d779481e",
		string(reference.TargetType),
		reference.RepositoryOwner,
		reference.RepositoryName,
		reference.IssueNumber,
		account.MaximumBookmarks,
	).Scan(
		&rawID,
		&rawTarget,
		&owner,
		&repositoryName,
		&issueNumber,
		&version,
		&createdAt,
		&updatedAt,
	)
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		t.Fatalf(
			"bookmark SQL failed: PostgreSQL code %s routine %s message %s",
			postgresError.Code,
			postgresError.Routine,
			postgresError.Message,
		)
	}
	if err != nil {
		t.Fatal("bookmark SQL failed without a PostgreSQL diagnostic")
	}
}

func integrationAccountID(t *testing.T, raw string) account.ID {
	t.Helper()
	id, err := account.ParseID(raw)
	if err != nil {
		t.Fatalf("account.ParseID() error = %v", err)
	}
	return id
}

func integrationResourceID(
	t *testing.T,
	raw string,
) account.ResourceID {
	t.Helper()
	id, err := account.ParseResourceID(raw)
	if err != nil {
		t.Fatalf("account.ParseResourceID() error = %v", err)
	}
	return id
}
