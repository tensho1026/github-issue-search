package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// AuthRepository atomically consumes OAuth state, links GitHub identities, and
// manages hashed server sessions.
type AuthRepository struct {
	store        authStore
	queryTimeout time.Duration
}

var _ port.AuthRepository = (*AuthRepository)(nil)

type authStore interface {
	Begin(context.Context) (authTransaction, error)
	Exec(
		context.Context,
		string,
		...any,
	) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type authTransaction interface {
	Exec(
		context.Context,
		string,
		...any,
	) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
	Rollback(context.Context) error
}

type poolAuthStore struct {
	pool *pgxpool.Pool
}

func (store poolAuthStore) Begin(
	ctx context.Context,
) (authTransaction, error) {
	return store.pool.Begin(ctx)
}

func (store poolAuthStore) Exec(
	ctx context.Context,
	query string,
	arguments ...any,
) (pgconn.CommandTag, error) {
	return store.pool.Exec(ctx, query, arguments...)
}

func (store poolAuthStore) QueryRow(
	ctx context.Context,
	query string,
	arguments ...any,
) pgx.Row {
	return store.pool.QueryRow(ctx, query, arguments...)
}

// NewAuthRepository binds authentication persistence to the configured pool.
func NewAuthRepository(pool *Pool) (*AuthRepository, error) {
	if pool == nil || pool.client == nil {
		return nil, ErrInvalidConfiguration
	}
	return &AuthRepository{
		store:        poolAuthStore{pool: pool.client},
		queryTimeout: pool.queryTimeout,
	}, nil
}

// SaveAuthorizationState inserts a single-use state hash while pruning a
// bounded batch of stale state records.
func (repository *AuthRepository) SaveAuthorizationState(
	ctx context.Context,
	state auth.AuthorizationState,
) error {
	if _, err := account.ParseID(state.ID); err != nil ||
		state.StateHash == (auth.Digest{}) ||
		!state.ExpiresAt.After(state.CreatedAt) {
		return ErrInvalidConfiguration
	}
	returnPath, err := auth.ValidateReturnPath(state.ReturnPath)
	if err != nil {
		return ErrInvalidConfiguration
	}
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	_, err = repository.store.Exec(
		queryContext,
		saveAuthorizationStateSQL,
		state.ID,
		state.StateHash.Bytes(),
		returnPath,
		state.ExpiresAt.UTC(),
		state.CreatedAt.UTC(),
	)
	if err != nil {
		return ErrQueryFailed
	}
	return nil
}

// ConsumeAuthorizationState atomically marks one unexpired state as used.
func (repository *AuthRepository) ConsumeAuthorizationState(
	ctx context.Context,
	stateHash auth.Digest,
	now time.Time,
) (string, error) {
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	var returnPath string
	err := repository.store.QueryRow(
		queryContext,
		consumeAuthorizationStateSQL,
		stateHash.Bytes(),
		now.UTC(),
	).Scan(&returnPath)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", auth.ErrAuthorizationStateNotFound
	}
	if err != nil {
		return "", ErrQueryFailed
	}
	return returnPath, nil
}

// UpsertIdentityAndCreateSession serializes linking by GitHub user ID, creates
// at most one account, and commits the new hashed session atomically.
func (repository *AuthRepository) UpsertIdentityAndCreateSession(
	ctx context.Context,
	draft auth.SessionDraft,
) (auth.Session, error) {
	if err := validateSessionDraft(draft, true); err != nil {
		return auth.Session{}, err
	}
	transactionContext, cancel := context.WithTimeout(
		ctx,
		repository.queryTimeout,
	)
	defer cancel()
	transaction, beginErr := repository.store.Begin(transactionContext)
	if beginErr != nil {
		return auth.Session{}, ErrQueryFailed
	}
	defer repository.rollback(transaction)
	if _, err := transaction.Exec(
		transactionContext,
		"SELECT pg_advisory_xact_lock($1)",
		draft.Identity.UserID,
	); err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	accountID, err := repository.findOrCreateIdentity(
		transactionContext,
		transaction,
		draft,
	)
	if err != nil {
		return auth.Session{}, err
	}
	if err := insertSession(
		transactionContext,
		transaction,
		accountID,
		draft,
	); err != nil {
		return auth.Session{}, err
	}
	if err := trimActiveSessions(
		transactionContext,
		transaction,
		accountID,
		draft.ID,
		draft.MaxActive,
		draft.CreatedAt,
	); err != nil {
		return auth.Session{}, err
	}
	if err := transaction.Commit(transactionContext); err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	return sessionFromDraft(accountID, draft), nil
}

// FindSession resolves one active, unexpired session by token hash.
func (repository *AuthRepository) FindSession(
	ctx context.Context,
	tokenHash auth.Digest,
	now time.Time,
) (auth.Session, error) {
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	return scanSession(repository.store.QueryRow(
		queryContext,
		findSessionSQL,
		tokenHash.Bytes(),
		now.UTC(),
	))
}

// RotateSession revokes the current token and inserts a fresh token and CSRF
// hash in the same transaction.
func (repository *AuthRepository) RotateSession(
	ctx context.Context,
	currentTokenHash auth.Digest,
	draft auth.SessionDraft,
	now time.Time,
) (auth.Session, error) {
	if err := validateSessionDraft(draft, false); err != nil {
		return auth.Session{}, err
	}
	transactionContext, cancel := context.WithTimeout(
		ctx,
		repository.queryTimeout,
	)
	defer cancel()
	transaction, err := repository.store.Begin(transactionContext)
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	defer repository.rollback(transaction)
	var rawAccountID string
	err = transaction.QueryRow(
		transactionContext,
		revokeCurrentSessionSQL,
		currentTokenHash.Bytes(),
		now.UTC(),
	).Scan(&rawAccountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	accountID, err := account.ParseID(rawAccountID)
	if err != nil || accountID != draft.AccountID {
		return auth.Session{}, ErrQueryFailed
	}
	if err := insertSession(
		transactionContext,
		transaction,
		accountID,
		draft,
	); err != nil {
		return auth.Session{}, err
	}
	if err := trimActiveSessions(
		transactionContext,
		transaction,
		accountID,
		draft.ID,
		draft.MaxActive,
		now,
	); err != nil {
		return auth.Session{}, err
	}
	if err := transaction.Commit(transactionContext); err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	return sessionFromDraft(accountID, draft), nil
}

// RevokeSession revokes exactly one active session token.
func (repository *AuthRepository) RevokeSession(
	ctx context.Context,
	tokenHash auth.Digest,
	now time.Time,
) error {
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	command, err := repository.store.Exec(
		queryContext,
		revokeSessionSQL,
		tokenHash.Bytes(),
		now.UTC(),
	)
	if err != nil {
		return ErrQueryFailed
	}
	if command.RowsAffected() != 1 {
		return auth.ErrSessionNotFound
	}
	return nil
}

// RevokeAllSessions revokes every active session for one account. Account
// deletion also removes these rows through the database cascade.
func (repository *AuthRepository) RevokeAllSessions(
	ctx context.Context,
	accountID account.ID,
	now time.Time,
) error {
	queryContext, cancel := context.WithTimeout(ctx, repository.queryTimeout)
	defer cancel()
	if _, err := repository.store.Exec(
		queryContext,
		revokeAllSessionsSQL,
		accountID.String(),
		now.UTC(),
	); err != nil {
		return ErrQueryFailed
	}
	return nil
}

func (repository *AuthRepository) findOrCreateIdentity(
	ctx context.Context,
	transaction authTransaction,
	draft auth.SessionDraft,
) (account.ID, error) {
	var rawAccountID string
	var status string
	err := transaction.QueryRow(
		ctx,
		findIdentityAccountSQL,
		draft.Identity.UserID,
	).Scan(&rawAccountID, &status)
	if err == nil {
		accountID, parseErr := account.ParseID(rawAccountID)
		if parseErr != nil || status != "active" {
			return account.ID{}, ErrQueryFailed
		}
		if _, updateErr := transaction.Exec(
			ctx,
			updateIdentitySQL,
			draft.Identity.UserID,
			draft.Identity.Login.String(),
			draft.Identity.AvatarURL,
			draft.Identity.ProfileURL,
			draft.CreatedAt.UTC(),
		); updateErr != nil {
			return account.ID{}, ErrQueryFailed
		}
		return accountID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return account.ID{}, ErrQueryFailed
	}
	if _, err := transaction.Exec(
		ctx,
		insertAccountSQL,
		draft.AccountID.String(),
		draft.CreatedAt.UTC(),
	); err != nil {
		return account.ID{}, ErrQueryFailed
	}
	if _, err := transaction.Exec(
		ctx,
		insertIdentitySQL,
		draft.IdentityID,
		draft.AccountID.String(),
		draft.Identity.UserID,
		draft.Identity.Login.String(),
		draft.Identity.AvatarURL,
		draft.Identity.ProfileURL,
		draft.CreatedAt.UTC(),
	); err != nil {
		return account.ID{}, ErrQueryFailed
	}
	return draft.AccountID, nil
}

func insertSession(
	ctx context.Context,
	transaction authTransaction,
	accountID account.ID,
	draft auth.SessionDraft,
) error {
	if _, err := transaction.Exec(
		ctx,
		insertSessionSQL,
		draft.ID,
		accountID.String(),
		draft.TokenHash.Bytes(),
		draft.CSRFHash.Bytes(),
		draft.ExpiresAt.UTC(),
		draft.CreatedAt.UTC(),
	); err != nil {
		return ErrQueryFailed
	}
	return nil
}

func trimActiveSessions(
	ctx context.Context,
	transaction authTransaction,
	accountID account.ID,
	currentSessionID string,
	maxActive int,
	now time.Time,
) error {
	if _, err := transaction.Exec(
		ctx,
		trimActiveSessionsSQL,
		accountID.String(),
		currentSessionID,
		maxActive,
		now.UTC(),
	); err != nil {
		return ErrQueryFailed
	}
	return nil
}

func validateSessionDraft(draft auth.SessionDraft, requireIdentityID bool) error {
	if _, err := account.ParseID(draft.ID); err != nil {
		return ErrInvalidConfiguration
	}
	if requireIdentityID {
		if _, err := account.ParseID(draft.IdentityID); err != nil {
			return ErrInvalidConfiguration
		}
	}
	if draft.AccountID == (account.ID{}) ||
		draft.TokenHash == (auth.Digest{}) ||
		draft.CSRFHash == (auth.Digest{}) ||
		!draft.ExpiresAt.After(draft.CreatedAt) ||
		draft.MaxActive < 1 ||
		draft.MaxActive > 50 ||
		draft.Identity.Validate() != nil {
		return ErrInvalidConfiguration
	}
	return nil
}

func sessionFromDraft(
	accountID account.ID,
	draft auth.SessionDraft,
) auth.Session {
	return auth.Session{
		ID:        draft.ID,
		AccountID: accountID,
		TokenHash: draft.TokenHash,
		CSRFHash:  draft.CSRFHash,
		ExpiresAt: draft.ExpiresAt.UTC(),
		Identity:  draft.Identity,
	}
}

func scanSession(row pgx.Row) (auth.Session, error) {
	var (
		session      auth.Session
		rawAccountID string
		rawTokenHash []byte
		rawCSRFHash  []byte
		rawLogin     string
		gitHubUserID int64
		avatarURL    string
		profileURL   string
	)
	err := row.Scan(
		&session.ID,
		&rawAccountID,
		&rawTokenHash,
		&rawCSRFHash,
		&session.ExpiresAt,
		&gitHubUserID,
		&rawLogin,
		&avatarURL,
		&profileURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	accountID, err := account.ParseID(rawAccountID)
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	tokenHash, err := auth.DigestFromBytes(rawTokenHash)
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	csrfHash, err := auth.DigestFromBytes(rawCSRFHash)
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	login, err := user.ParseUsername(rawLogin)
	if err != nil {
		return auth.Session{}, ErrQueryFailed
	}
	session.AccountID = accountID
	session.TokenHash = tokenHash
	session.CSRFHash = csrfHash
	session.Identity = auth.GitHubIdentity{
		UserID:     gitHubUserID,
		Login:      login,
		AvatarURL:  avatarURL,
		ProfileURL: profileURL,
	}
	if session.Identity.Validate() != nil {
		return auth.Session{}, ErrQueryFailed
	}
	return session, nil
}

func (repository *AuthRepository) rollback(transaction authTransaction) {
	rollbackContext, cancel := context.WithTimeout(
		context.Background(),
		repository.queryTimeout,
	)
	defer cancel()
	_ = transaction.Rollback(rollbackContext)
}

const saveAuthorizationStateSQL = `WITH stale AS (
    SELECT id
    FROM oauth_authorization_states
    WHERE expires_at <= $5
       OR (consumed_at IS NOT NULL AND consumed_at <= $5 - interval '1 hour')
    ORDER BY created_at
    LIMIT 100
    FOR UPDATE SKIP LOCKED
), removed AS (
    DELETE FROM oauth_authorization_states AS states
    USING stale
    WHERE states.id = stale.id
)
INSERT INTO oauth_authorization_states (
    id,
    state_hash,
    return_path,
    expires_at,
    created_at
) VALUES ($1, $2, $3, $4, $5)`

const consumeAuthorizationStateSQL = `UPDATE oauth_authorization_states
SET consumed_at = $2
WHERE state_hash = $1
  AND consumed_at IS NULL
  AND expires_at > $2
RETURNING return_path`

const findIdentityAccountSQL = `SELECT identities.account_id::text, accounts.status
FROM github_identities AS identities
JOIN accounts ON accounts.id = identities.account_id
WHERE identities.github_user_id = $1`

const insertAccountSQL = `INSERT INTO accounts (
    id,
    status,
    created_at,
    updated_at
) VALUES ($1, 'active', $2, $2)`

const insertIdentitySQL = `INSERT INTO github_identities (
    id,
    account_id,
    github_user_id,
    login,
    avatar_url,
    profile_url,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`

const updateIdentitySQL = `UPDATE github_identities
SET login = $2,
    avatar_url = $3,
    profile_url = $4,
    updated_at = $5
WHERE github_user_id = $1`

const insertSessionSQL = `INSERT INTO auth_sessions (
    id,
    account_id,
    token_hash,
    csrf_secret_hash,
    expires_at,
    last_seen_at,
    created_at
) VALUES ($1, $2, $3, $4, $5, $6, $6)`

const trimActiveSessionsSQL = `WITH excess AS (
    SELECT id
    FROM auth_sessions
    WHERE account_id = $1
      AND id <> $2
      AND revoked_at IS NULL
    ORDER BY created_at DESC, id DESC
    OFFSET GREATEST($3 - 1, 0)
)
UPDATE auth_sessions
SET revoked_at = $4
WHERE id IN (SELECT id FROM excess)`

const findSessionSQL = `SELECT
    sessions.id::text,
    sessions.account_id::text,
    sessions.token_hash,
    sessions.csrf_secret_hash,
    sessions.expires_at,
    identities.github_user_id,
    identities.login,
    identities.avatar_url,
    identities.profile_url
FROM auth_sessions AS sessions
JOIN accounts ON accounts.id = sessions.account_id
JOIN github_identities AS identities
    ON identities.account_id = sessions.account_id
WHERE sessions.token_hash = $1
  AND sessions.revoked_at IS NULL
  AND sessions.expires_at > $2
  AND accounts.status = 'active'`

const revokeCurrentSessionSQL = `UPDATE auth_sessions
SET revoked_at = $2
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > $2
RETURNING account_id::text`

const revokeSessionSQL = `UPDATE auth_sessions
SET revoked_at = $2
WHERE token_hash = $1
  AND revoked_at IS NULL`

const revokeAllSessionsSQL = `UPDATE auth_sessions
SET revoked_at = $2
WHERE account_id = $1
  AND revoked_at IS NULL`
