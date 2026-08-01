package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

var (
	// ErrQueryFailed is the safe persistence error returned when PostgreSQL
	// cannot complete an account-owned operation.
	ErrQueryFailed = errors.New("database query failed")
)

// AccountRepository enforces account ownership in every SQL predicate.
type AccountRepository struct {
	executor     accountExecutor
	queryTimeout time.Duration
}

var _ port.AccountRepository = (*AccountRepository)(nil)

type accountExecutor interface {
	Exec(
		ctx context.Context,
		sql string,
		arguments ...any,
	) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(
		ctx context.Context,
		sql string,
		args ...any,
	) (accountRows, error)
}

type accountRows interface {
	Close()
	Err() error
	Next() bool
	Scan(destinations ...any) error
}

type poolAccountExecutor struct {
	pool *pgxpool.Pool
}

func (executor poolAccountExecutor) Exec(
	ctx context.Context,
	sql string,
	arguments ...any,
) (pgconn.CommandTag, error) {
	return executor.pool.Exec(ctx, sql, arguments...)
}

func (executor poolAccountExecutor) QueryRow(
	ctx context.Context,
	sql string,
	arguments ...any,
) pgx.Row {
	return executor.pool.QueryRow(ctx, sql, arguments...)
}

func (executor poolAccountExecutor) Query(
	ctx context.Context,
	sql string,
	arguments ...any,
) (accountRows, error) {
	return executor.pool.Query(ctx, sql, arguments...)
}

// NewAccountRepository binds account operations to the configured pool.
func NewAccountRepository(pool *Pool) (*AccountRepository, error) {
	if pool == nil || pool.client == nil {
		return nil, ErrInvalidConfiguration
	}

	return &AccountRepository{
		executor:     poolAccountExecutor{pool: pool.client},
		queryTimeout: pool.queryTimeout,
	}, nil
}

// OwnedDataSummary returns content-free counts scoped by the authenticated
// account ID. It cannot enumerate another account without that explicit ID.
func (repository *AccountRepository) OwnedDataSummary(
	ctx context.Context,
	accountID account.ID,
) (account.OwnedDataSummary, error) {
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	var summary account.OwnedDataSummary
	err := repository.executor.QueryRow(
		queryContext,
		ownedDataSummarySQL,
		accountID.String(),
	).Scan(
		&summary.Identities,
		&summary.Sessions,
		&summary.Bookmarks,
		&summary.SavedSearches,
		&summary.Preferences,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return account.OwnedDataSummary{}, account.ErrNotFound
		}
		return account.OwnedDataSummary{}, ErrQueryFailed
	}

	return summary, nil
}

// Delete removes one account and all account-owned rows through declared
// foreign-key cascades.
func (repository *AccountRepository) Delete(
	ctx context.Context,
	accountID account.ID,
) error {
	auditID, err := account.NewResourceID()
	if err != nil {
		return ErrQueryFailed
	}
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	command, err := repository.executor.Exec(
		queryContext,
		`WITH deleted AS (
		    DELETE FROM accounts WHERE id = $1 RETURNING id
		)
		INSERT INTO privacy_audit_events (id, account_id, event_type)
		SELECT $2, NULL, 'account_deleted' FROM deleted`,
		accountID.String(),
		auditID.String(),
	)
	if err != nil {
		return ErrQueryFailed
	}
	if command.RowsAffected() != 1 {
		return account.ErrNotFound
	}

	return nil
}

const ownedDataSummarySQL = `SELECT
    (SELECT count(*) FROM github_identities WHERE account_id = $1),
    (SELECT count(*) FROM auth_sessions WHERE account_id = $1),
    (SELECT count(*) FROM bookmarks WHERE account_id = $1),
    (SELECT count(*) FROM saved_searches WHERE account_id = $1),
    (SELECT count(*) FROM user_preferences WHERE account_id = $1)
WHERE EXISTS (SELECT 1 FROM accounts WHERE id = $1)`
