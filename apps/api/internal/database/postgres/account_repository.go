package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
}

// NewAccountRepository binds account operations to the configured pool.
func NewAccountRepository(pool *Pool) (*AccountRepository, error) {
	if pool == nil || pool.client == nil {
		return nil, ErrInvalidConfiguration
	}

	return &AccountRepository{
		executor:     pool.client,
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
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	command, err := repository.executor.Exec(
		queryContext,
		"DELETE FROM accounts WHERE id = $1",
		accountID.String(),
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
