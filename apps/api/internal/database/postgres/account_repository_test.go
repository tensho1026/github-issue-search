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

func TestAccountRepositoryScopesSummaryToOneAccount(t *testing.T) {
	accountID := mustAccountID(t)
	executor := &recordingAccountExecutor{
		row: recordingRow{values: []int64{1, 2, 3, 4, 1}},
	}
	repository := AccountRepository{
		executor:     executor,
		queryTimeout: time.Second,
	}

	summary, err := repository.OwnedDataSummary(
		context.Background(),
		accountID,
	)
	if err != nil {
		t.Fatalf("OwnedDataSummary() error = %v", err)
	}
	if summary.Identities != 1 ||
		summary.Sessions != 2 ||
		summary.Bookmarks != 3 ||
		summary.SavedSearches != 4 ||
		summary.Preferences != 1 {
		t.Fatalf("OwnedDataSummary() = %+v", summary)
	}
	if len(executor.arguments) != 1 ||
		executor.arguments[0] != accountID.String() {
		t.Fatalf("query arguments = %v", executor.arguments)
	}
	if strings.Contains(executor.query, accountID.String()) ||
		strings.Count(executor.query, "$1") < 6 {
		t.Fatal("summary query did not preserve parameterized account ownership")
	}
}

func TestAccountRepositoryMapsMissingAndDriverErrors(t *testing.T) {
	accountID := mustAccountID(t)
	for _, test := range []struct {
		name string
		row  pgx.Row
		want error
	}{
		{
			name: "missing account",
			row:  recordingRow{err: pgx.ErrNoRows},
			want: account.ErrNotFound,
		},
		{
			name: "driver failure",
			row: recordingRow{
				err: errors.New("sensitive-host.example: connection failed"),
			},
			want: ErrQueryFailed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := AccountRepository{
				executor:     &recordingAccountExecutor{row: test.row},
				queryTimeout: time.Second,
			}
			_, err := repository.OwnedDataSummary(
				context.Background(),
				accountID,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("OwnedDataSummary() error = %v", err)
			}
			if strings.Contains(err.Error(), "sensitive-host.example") {
				t.Fatal("repository error exposed a driver detail")
			}
		})
	}
}

func TestAccountRepositoryDeleteUsesOwnedParameterizedPredicate(t *testing.T) {
	accountID := mustAccountID(t)
	executor := &recordingAccountExecutor{
		commandTag: pgconn.NewCommandTag("DELETE 1"),
	}
	repository := AccountRepository{
		executor:     executor,
		queryTimeout: time.Second,
	}

	if err := repository.Delete(context.Background(), accountID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !strings.Contains(
		executor.query,
		"DELETE FROM accounts WHERE id = $1",
	) ||
		!strings.Contains(executor.query, "'account_deleted'") ||
		len(executor.arguments) != 2 ||
		executor.arguments[0] != accountID.String() {
		t.Fatalf(
			"Delete() query = %q, arguments = %v",
			executor.query,
			executor.arguments,
		)
	}
}

func TestAccountRepositoryDeleteReportsMissingAccount(t *testing.T) {
	repository := AccountRepository{
		executor: &recordingAccountExecutor{
			commandTag: pgconn.NewCommandTag("DELETE 0"),
		},
		queryTimeout: time.Second,
	}

	err := repository.Delete(context.Background(), mustAccountID(t))
	if !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("Delete() error = %v", err)
	}
}

type recordingAccountExecutor struct {
	arguments  []any
	commandTag pgconn.CommandTag
	err        error
	query      string
	row        pgx.Row
}

func (executor *recordingAccountExecutor) Exec(
	_ context.Context,
	query string,
	arguments ...any,
) (pgconn.CommandTag, error) {
	executor.query = query
	executor.arguments = arguments
	return executor.commandTag, executor.err
}

func (executor *recordingAccountExecutor) QueryRow(
	_ context.Context,
	query string,
	arguments ...any,
) pgx.Row {
	executor.query = query
	executor.arguments = arguments
	return executor.row
}

func (executor *recordingAccountExecutor) Query(
	_ context.Context,
	query string,
	arguments ...any,
) (accountRows, error) {
	executor.query = query
	executor.arguments = arguments
	return nil, executor.err
}

type recordingRow struct {
	err    error
	values []int64
}

func (row recordingRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	for index, value := range row.values {
		destination, ok := destinations[index].(*int64)
		if !ok {
			return errors.New("unexpected scan destination")
		}
		*destination = value
	}
	return nil
}

func mustAccountID(t *testing.T) account.ID {
	t.Helper()
	id, err := account.ParseID("8bbfd7ed-a424-4ec3-a1b8-647006da1816")
	if err != nil {
		t.Fatalf("account.ParseID() error = %v", err)
	}
	return id
}
