package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
)

const featureResourceID = "69cf232f-f1ba-4c24-9b18-9083f90b1a1a"

func TestAccountRepositoryListsBookmarksWithOwnedStableQuery(t *testing.T) {
	accountID := mustAccountID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	executor := &scriptedAccountExecutor{
		queryRows: &valueRows{rows: []valueRow{
			{values: []any{
				featureResourceID,
				"issue",
				"openai",
				"openai-go",
				42,
				int64(2),
				now,
				now,
				1,
			}},
		}},
	}
	repository := AccountRepository{
		executor:     executor,
		queryTimeout: time.Second,
	}
	page, _ := account.NewPage(2, 10)
	result, err := repository.ListBookmarks(
		context.Background(),
		accountID,
		page,
	)
	if err != nil {
		t.Fatalf("ListBookmarks() error = %v", err)
	}
	if len(result.Items) != 1 ||
		result.Total != 1 ||
		result.Items[0].AccountID != accountID ||
		result.Items[0].Reference.IssueNumber == nil ||
		*result.Items[0].Reference.IssueNumber != 42 {
		t.Fatalf("result = %+v", result)
	}
	call := executor.calls[0]
	if !strings.Contains(call.query, "WHERE account_id = $1") ||
		!strings.Contains(call.query, "ORDER BY created_at DESC, id DESC") ||
		call.arguments[0] != accountID.String() ||
		call.arguments[2] != 10 {
		t.Fatalf("query call = %+v", call)
	}
}

func TestAccountRepositoryUpsertsBookmarkAndEnforcesQuota(t *testing.T) {
	accountID := mustAccountID(t)
	resourceID := mustResourceID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	number := 42
	reference, _ := account.NewBookmarkReference(
		account.BookmarkTargetIssue,
		"openai",
		"openai-go",
		&number,
	)
	input := account.Bookmark{
		ID:        resourceID,
		AccountID: accountID,
		Reference: reference,
	}
	executor := &scriptedAccountExecutor{rowQueue: []pgx.Row{
		valueRow{values: []any{
			featureResourceID,
			"issue",
			"openai",
			"openai-go",
			42,
			int64(1),
			now,
			now,
		}},
	}}
	repository := AccountRepository{
		executor:     executor,
		queryTimeout: time.Second,
	}
	result, err := repository.UpsertBookmark(context.Background(), input)
	if err != nil || result.ID != resourceID || result.Version != 1 {
		t.Fatalf("UpsertBookmark() = %+v, %v", result, err)
	}
	if !strings.Contains(executor.calls[0].query, "pg_advisory_xact_lock") ||
		!strings.Contains(executor.calls[0].query, "account_id = $1") {
		t.Fatalf("upsert query = %s", executor.calls[0].query)
	}

	quotaRepository := AccountRepository{
		executor: &scriptedAccountExecutor{
			rowQueue: []pgx.Row{valueRow{err: pgx.ErrNoRows}},
		},
		queryTimeout: time.Second,
	}
	_, err = quotaRepository.UpsertBookmark(context.Background(), input)
	if !errors.Is(err, account.ErrQuotaExceeded) {
		t.Fatalf("quota error = %v", err)
	}
}

func TestAccountRepositoryBookmarkDeleteMasksOwnershipAndDetectsVersion(
	t *testing.T,
) {
	accountID := mustAccountID(t)
	resourceID := mustResourceID(t)
	tests := []struct {
		name string
		row  pgx.Row
		want error
	}{
		{name: "other account or missing", row: valueRow{err: pgx.ErrNoRows}, want: account.ErrNotFound},
		{name: "stale version", row: valueRow{values: []any{int64(3)}}, want: account.ErrVersionConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := &scriptedAccountExecutor{
				commandTags: []pgconn.CommandTag{
					pgconn.NewCommandTag("DELETE 0"),
				},
				rowQueue: []pgx.Row{test.row},
			}
			repository := AccountRepository{
				executor:     executor,
				queryTimeout: time.Second,
			}
			err := repository.DeleteBookmark(
				context.Background(),
				accountID,
				resourceID,
				2,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("DeleteBookmark() error = %v", err)
			}
			for _, call := range executor.calls {
				if !strings.Contains(call.query, "account_id = $1") {
					t.Fatalf("unowned query = %q", call.query)
				}
			}
		})
	}
}

func TestAccountRepositoryCreatesAndClassifiesSavedSearchFailures(
	t *testing.T,
) {
	accountID := mustAccountID(t)
	resourceID := mustResourceID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	input := account.SavedSearch{
		ID:         resourceID,
		AccountID:  accountID,
		SearchType: account.SearchTypeIssue,
		Name:       "Go issues",
		Filters:    []byte(`{"username":"octocat"}`),
	}
	successRepository := AccountRepository{
		executor: &scriptedAccountExecutor{rowQueue: []pgx.Row{
			valueRow{values: []any{
				featureResourceID,
				"issue",
				"Go issues",
				[]byte(`{"username":"octocat"}`),
				int64(1),
				now,
				now,
			}},
		}},
		queryTimeout: time.Second,
	}
	created, err := successRepository.CreateSavedSearch(
		context.Background(),
		input,
	)
	if err != nil || created.Version != 1 {
		t.Fatalf("CreateSavedSearch() = %+v, %v", created, err)
	}

	for _, test := range []struct {
		name      string
		duplicate bool
		want      error
	}{
		{name: "duplicate", duplicate: true, want: account.ErrDuplicateSavedSearch},
		{name: "quota", duplicate: false, want: account.ErrQuotaExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := AccountRepository{
				executor: &scriptedAccountExecutor{rowQueue: []pgx.Row{
					valueRow{err: pgx.ErrNoRows},
					valueRow{values: []any{test.duplicate}},
				}},
				queryTimeout: time.Second,
			}
			_, err := repository.CreateSavedSearch(
				context.Background(),
				input,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("CreateSavedSearch() error = %v", err)
			}
		})
	}
}

func TestAccountRepositoryListsSavedSearchesWithOwnedStableQuery(
	t *testing.T,
) {
	accountID := mustAccountID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	executor := &scriptedAccountExecutor{
		queryRows: &valueRows{rows: []valueRow{
			{values: []any{
				featureResourceID,
				"issue",
				"Go issues",
				[]byte(`{"username":"octocat"}`),
				int64(2),
				now,
				now,
				1,
			}},
		}},
	}
	repository := AccountRepository{
		executor:     executor,
		queryTimeout: time.Second,
	}
	page, _ := account.NewPage(1, 20)
	result, err := repository.ListSavedSearches(
		context.Background(),
		accountID,
		page,
	)
	if err != nil {
		t.Fatalf("ListSavedSearches() error = %v", err)
	}
	if len(result.Items) != 1 ||
		result.Total != 1 ||
		result.Items[0].Name != "Go issues" ||
		string(result.Items[0].Filters) != `{"username":"octocat"}` {
		t.Fatalf("result = %+v", result)
	}
	call := executor.calls[0]
	if !strings.Contains(call.query, "WHERE account_id = $1") ||
		!strings.Contains(call.query, "ORDER BY updated_at DESC, id DESC") ||
		call.arguments[0] != accountID.String() {
		t.Fatalf("query call = %+v", call)
	}
}

func TestAccountRepositoryDeletesSavedSearchWithOwnershipAndVersion(
	t *testing.T,
) {
	accountID := mustAccountID(t)
	resourceID := mustResourceID(t)
	successExecutor := &scriptedAccountExecutor{
		commandTags: []pgconn.CommandTag{
			pgconn.NewCommandTag("DELETE 1"),
		},
	}
	successRepository := AccountRepository{
		executor:     successExecutor,
		queryTimeout: time.Second,
	}
	if err := successRepository.DeleteSavedSearch(
		context.Background(),
		accountID,
		resourceID,
		2,
	); err != nil {
		t.Fatalf("DeleteSavedSearch() error = %v", err)
	}
	if !strings.Contains(
		successExecutor.calls[0].query,
		"account_id = $1 AND id = $2 AND version = $3",
	) {
		t.Fatalf("delete query = %q", successExecutor.calls[0].query)
	}

	missingRepository := AccountRepository{
		executor: &scriptedAccountExecutor{
			commandTags: []pgconn.CommandTag{
				pgconn.NewCommandTag("DELETE 0"),
			},
			rowQueue: []pgx.Row{valueRow{err: pgx.ErrNoRows}},
		},
		queryTimeout: time.Second,
	}
	if err := missingRepository.DeleteSavedSearch(
		context.Background(),
		accountID,
		resourceID,
		2,
	); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("missing DeleteSavedSearch() error = %v", err)
	}
}

func TestAccountRepositorySavedSearchVersionAndDuplicateHandling(
	t *testing.T,
) {
	input := account.SavedSearch{
		ID:         mustResourceID(t),
		AccountID:  mustAccountID(t),
		SearchType: account.SearchTypeRepository,
		Name:       "Maintained Go",
		Filters:    []byte(`{"languages":["Go"]}`),
		Version:    4,
	}
	tests := []struct {
		name      string
		duplicate bool
		want      error
	}{
		{name: "duplicate", duplicate: true, want: account.ErrDuplicateSavedSearch},
		{name: "stale", duplicate: false, want: account.ErrVersionConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := AccountRepository{
				executor: &scriptedAccountExecutor{rowQueue: []pgx.Row{
					valueRow{err: pgx.ErrNoRows},
					valueRow{values: []any{int64(5)}},
					valueRow{values: []any{test.duplicate}},
				}},
				queryTimeout: time.Second,
			}
			_, err := repository.UpdateSavedSearch(
				context.Background(),
				input,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("UpdateSavedSearch() error = %v", err)
			}
		})
	}
}

func TestAccountRepositoryPreferencesDefaultsAndOptimisticUpsert(
	t *testing.T,
) {
	accountID := mustAccountID(t)
	now := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	defaultRepository := AccountRepository{
		executor: &scriptedAccountExecutor{
			rowQueue: []pgx.Row{valueRow{err: pgx.ErrNoRows}},
		},
		queryTimeout: time.Second,
	}
	defaults, err := defaultRepository.GetPreferences(
		context.Background(),
		accountID,
	)
	if err != nil ||
		defaults.Version != 0 ||
		defaults.Theme != account.ThemeSystem ||
		defaults.ResultsPerPage != account.DefaultPageSize {
		t.Fatalf("GetPreferences() = %+v, %v", defaults, err)
	}

	storedRepository := AccountRepository{
		executor: &scriptedAccountExecutor{rowQueue: []pgx.Row{
			valueRow{values: []any{
				"dark",
				"reduce",
				50,
				int64(2),
				now,
				now,
			}},
		}},
		queryTimeout: time.Second,
	}
	stored, err := storedRepository.GetPreferences(
		context.Background(),
		accountID,
	)
	if err != nil ||
		stored.Version != 2 ||
		stored.Theme != account.ThemeDark ||
		!stored.CreatedAt.Equal(now) {
		t.Fatalf("stored GetPreferences() = %+v, %v", stored, err)
	}

	input, _ := account.NewPreferences(
		account.ThemeDark,
		account.ReducedMotionReduce,
		50,
	)
	input.AccountID = accountID
	successRepository := AccountRepository{
		executor: &scriptedAccountExecutor{rowQueue: []pgx.Row{
			valueRow{values: []any{
				"dark",
				"reduce",
				50,
				int64(1),
				now,
				now,
			}},
		}},
		queryTimeout: time.Second,
	}
	updated, err := successRepository.UpsertPreferences(
		context.Background(),
		input,
		0,
	)
	if err != nil || updated.Version != 1 || updated.Theme != account.ThemeDark {
		t.Fatalf("UpsertPreferences() = %+v, %v", updated, err)
	}

	staleRepository := AccountRepository{
		executor: &scriptedAccountExecutor{
			rowQueue: []pgx.Row{valueRow{err: pgx.ErrNoRows}},
		},
		queryTimeout: time.Second,
	}
	_, err = staleRepository.UpsertPreferences(
		context.Background(),
		input,
		9,
	)
	if !errors.Is(err, account.ErrVersionConflict) {
		t.Fatalf("UpsertPreferences() error = %v", err)
	}
}

type accountQueryCall struct {
	query     string
	arguments []any
}

type scriptedAccountExecutor struct {
	calls       []accountQueryCall
	commandTags []pgconn.CommandTag
	execError   error
	queryError  error
	queryRows   accountRows
	rowQueue    []pgx.Row
}

func (executor *scriptedAccountExecutor) Exec(
	_ context.Context,
	query string,
	arguments ...any,
) (pgconn.CommandTag, error) {
	executor.calls = append(executor.calls, accountQueryCall{
		query:     query,
		arguments: arguments,
	})
	if len(executor.commandTags) == 0 {
		return pgconn.CommandTag{}, executor.execError
	}
	tag := executor.commandTags[0]
	executor.commandTags = executor.commandTags[1:]
	return tag, executor.execError
}

func (executor *scriptedAccountExecutor) QueryRow(
	_ context.Context,
	query string,
	arguments ...any,
) pgx.Row {
	executor.calls = append(executor.calls, accountQueryCall{
		query:     query,
		arguments: arguments,
	})
	if len(executor.rowQueue) == 0 {
		return valueRow{err: errors.New("unexpected QueryRow call")}
	}
	row := executor.rowQueue[0]
	executor.rowQueue = executor.rowQueue[1:]
	return row
}

func (executor *scriptedAccountExecutor) Query(
	_ context.Context,
	query string,
	arguments ...any,
) (accountRows, error) {
	executor.calls = append(executor.calls, accountQueryCall{
		query:     query,
		arguments: arguments,
	})
	return executor.queryRows, executor.queryError
}

type valueRows struct {
	index int
	rows  []valueRow
	err   error
}

func (rows *valueRows) Close() {}

func (rows *valueRows) Err() error {
	return rows.err
}

func (rows *valueRows) Next() bool {
	if rows.index >= len(rows.rows) {
		return false
	}
	rows.index++
	return true
}

func (rows *valueRows) Scan(destinations ...any) error {
	return rows.rows[rows.index-1].Scan(destinations...)
}

type valueRow struct {
	values []any
	err    error
}

func (row valueRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != len(row.values) {
		return errors.New("unexpected scan destination count")
	}
	for index, value := range row.values {
		switch destination := destinations[index].(type) {
		case *string:
			typed, ok := value.(string)
			if !ok {
				return errors.New("unexpected string value")
			}
			*destination = typed
		case *int:
			typed, ok := value.(int)
			if !ok {
				return errors.New("unexpected integer value")
			}
			*destination = typed
		case **int:
			typed, ok := value.(int)
			if !ok {
				return errors.New("unexpected nullable integer value")
			}
			*destination = &typed
		case *int64:
			typed, ok := value.(int64)
			if !ok {
				return errors.New("unexpected int64 value")
			}
			*destination = typed
		case *time.Time:
			typed, ok := value.(time.Time)
			if !ok {
				return errors.New("unexpected timestamp value")
			}
			*destination = typed
		case *[]byte:
			typed, ok := value.([]byte)
			if !ok {
				return errors.New("unexpected byte slice value")
			}
			*destination = append([]byte(nil), typed...)
		case *bool:
			typed, ok := value.(bool)
			if !ok {
				return errors.New("unexpected boolean value")
			}
			*destination = typed
		default:
			return errors.New("unexpected scan destination type")
		}
	}
	return nil
}

func mustResourceID(t *testing.T) account.ResourceID {
	t.Helper()
	id, err := account.ParseResourceID(featureResourceID)
	if err != nil {
		t.Fatalf("account.ParseResourceID() error = %v", err)
	}
	return id
}
