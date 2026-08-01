package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

func TestSaveAuthorizationStatePersistsOnlyDigest(t *testing.T) {
	store := &recordingAuthStore{
		execTag: pgconn.NewCommandTag("INSERT 0 1"),
	}
	repository := &AuthRepository{
		store:        store,
		queryTimeout: time.Second,
	}
	now := time.Date(2026, time.August, 1, 4, 0, 0, 0, time.UTC)
	const rawState = "raw-state-must-not-be-persisted"
	state := auth.AuthorizationState{
		ID:         "624fc28b-46aa-468b-86a3-f112d69356cb",
		StateHash:  auth.Hash(rawState),
		ReturnPath: "/workspace",
		ExpiresAt:  now.Add(10 * time.Minute),
		CreatedAt:  now,
	}
	if err := repository.SaveAuthorizationState(
		context.Background(),
		state,
	); err != nil {
		t.Fatalf("SaveAuthorizationState() error = %v", err)
	}
	if store.execQuery != saveAuthorizationStateSQL ||
		len(store.execArguments) != 5 {
		t.Fatalf("query = %q args = %v", store.execQuery, store.execArguments)
	}
	if strings.Contains(store.execQuery, rawState) {
		t.Fatal("state plaintext was interpolated into SQL")
	}
	digest, ok := store.execArguments[1].([]byte)
	if !ok || string(digest) == rawState || len(digest) != 32 {
		t.Fatal("state argument was not a fixed-size digest")
	}
}

func TestConsumeAuthorizationStateMasksReplayAndExpiry(t *testing.T) {
	store := &recordingAuthStore{
		queryRow: scanAuthRow(func(...any) error { return pgx.ErrNoRows }),
	}
	repository := &AuthRepository{
		store:        store,
		queryTimeout: time.Second,
	}
	_, err := repository.ConsumeAuthorizationState(
		context.Background(),
		auth.Hash("state"),
		time.Now(),
	)
	if !errors.Is(err, auth.ErrAuthorizationStateNotFound) {
		t.Fatalf("ConsumeAuthorizationState() error = %v", err)
	}
	if store.query != consumeAuthorizationStateSQL {
		t.Fatalf("query = %q", store.query)
	}
}

func TestUpsertIdentityAndCreateSessionCommitsAtomicNewAccount(t *testing.T) {
	transaction := &recordingAuthTransaction{
		queryRows: []pgx.Row{
			scanAuthRow(func(...any) error { return pgx.ErrNoRows }),
		},
	}
	store := &recordingAuthStore{transaction: transaction}
	repository := &AuthRepository{
		store:        store,
		queryTimeout: time.Second,
	}
	draft := testSessionDraft(t)

	session, err := repository.UpsertIdentityAndCreateSession(
		context.Background(),
		draft,
	)
	if err != nil {
		t.Fatalf("UpsertIdentityAndCreateSession() error = %v", err)
	}
	if !transaction.committed || transaction.rolledBack {
		t.Fatal("authentication transaction did not commit exactly once")
	}
	wantQueries := []string{
		"SELECT pg_advisory_xact_lock($1)",
		insertAccountSQL,
		insertIdentitySQL,
		insertSessionSQL,
		trimActiveSessionsSQL,
	}
	if len(transaction.execQueries) != len(wantQueries) {
		t.Fatalf("Exec() queries = %v", transaction.execQueries)
	}
	for index, expected := range wantQueries {
		if transaction.execQueries[index] != expected {
			t.Fatalf("query %d = %q", index, transaction.execQueries[index])
		}
	}
	if session.AccountID != draft.AccountID ||
		session.Identity.UserID != draft.Identity.UserID {
		t.Fatalf("session = %+v", session)
	}
}

func TestFindSessionNormalizesIdentityAndHashes(t *testing.T) {
	accountID := mustAccountID(t)
	tokenHash := auth.Hash("session-token")
	csrfHash := auth.Hash("csrf-token")
	expiresAt := time.Date(2026, time.August, 2, 4, 0, 0, 0, time.UTC)
	store := &recordingAuthStore{
		queryRow: scanAuthRow(func(destinations ...any) error {
			mustSetScanValue(
				destinations[0],
				"624fc28b-46aa-468b-86a3-f112d69356cb",
			)
			mustSetScanValue(destinations[1], accountID.String())
			mustSetScanValue(destinations[2], tokenHash.Bytes())
			mustSetScanValue(destinations[3], csrfHash.Bytes())
			mustSetScanValue(destinations[4], expiresAt)
			mustSetScanValue(destinations[5], int64(583231))
			mustSetScanValue(destinations[6], "octocat")
			mustSetScanValue(
				destinations[7],
				"https://avatars.githubusercontent.com/u/583231",
			)
			mustSetScanValue(destinations[8], "https://github.com/octocat")
			return nil
		}),
	}
	repository := &AuthRepository{
		store:        store,
		queryTimeout: time.Second,
	}

	session, err := repository.FindSession(
		context.Background(),
		tokenHash,
		expiresAt.Add(-time.Hour),
	)
	if err != nil {
		t.Fatalf("FindSession() error = %v", err)
	}
	if session.AccountID != accountID ||
		session.Identity.Login != "octocat" ||
		session.CSRFHash != csrfHash {
		t.Fatalf("session = %+v", session)
	}
	if store.query != findSessionSQL || len(store.queryArguments) != 2 {
		t.Fatalf("query = %q args = %v", store.query, store.queryArguments)
	}
}

func TestAuthRepositoryDoesNotForwardDriverErrors(t *testing.T) {
	const driverDetail = "postgresql://credential@database.example/private"
	store := &recordingAuthStore{
		execErr: errors.New(driverDetail),
	}
	repository := &AuthRepository{
		store:        store,
		queryTimeout: time.Second,
	}
	err := repository.RevokeSession(
		context.Background(),
		auth.Hash("session"),
		time.Now(),
	)
	if !errors.Is(err, ErrQueryFailed) ||
		strings.Contains(err.Error(), driverDetail) {
		t.Fatalf("RevokeSession() error = %v", err)
	}
}

func testSessionDraft(t *testing.T) auth.SessionDraft {
	t.Helper()
	login, err := user.ParseUsername("octocat")
	if err != nil {
		t.Fatal(err)
	}
	return auth.SessionDraft{
		ID:         "624fc28b-46aa-468b-86a3-f112d69356cb",
		IdentityID: "6ca6dfc4-0114-44fb-a9f8-d703f8c9a8b2",
		AccountID:  mustAccountID(t),
		TokenHash:  auth.Hash("session-token"),
		CSRFHash:   auth.Hash("csrf-token"),
		CreatedAt: time.Date(
			2026,
			time.August,
			1,
			4,
			0,
			0,
			0,
			time.UTC,
		),
		ExpiresAt: time.Date(
			2026,
			time.August,
			2,
			4,
			0,
			0,
			0,
			time.UTC,
		),
		Identity: auth.GitHubIdentity{
			UserID:     583231,
			Login:      login,
			AvatarURL:  "https://avatars.githubusercontent.com/u/583231",
			ProfileURL: "https://github.com/octocat",
		},
		MaxActive: 10,
	}
}

type recordingAuthStore struct {
	transaction    authTransaction
	execTag        pgconn.CommandTag
	execErr        error
	execQuery      string
	execArguments  []any
	queryRow       pgx.Row
	query          string
	queryArguments []any
}

func (store *recordingAuthStore) Begin(
	context.Context,
) (authTransaction, error) {
	return store.transaction, nil
}

func (store *recordingAuthStore) Exec(
	_ context.Context,
	query string,
	arguments ...any,
) (pgconn.CommandTag, error) {
	store.execQuery = query
	store.execArguments = arguments
	return store.execTag, store.execErr
}

func (store *recordingAuthStore) QueryRow(
	_ context.Context,
	query string,
	arguments ...any,
) pgx.Row {
	store.query = query
	store.queryArguments = arguments
	return store.queryRow
}

type recordingAuthTransaction struct {
	queryRows   []pgx.Row
	execQueries []string
	committed   bool
	rolledBack  bool
}

func (transaction *recordingAuthTransaction) Exec(
	_ context.Context,
	query string,
	_ ...any,
) (pgconn.CommandTag, error) {
	transaction.execQueries = append(transaction.execQueries, query)
	return pgconn.NewCommandTag("OK"), nil
}

func (transaction *recordingAuthTransaction) QueryRow(
	_ context.Context,
	_ string,
	_ ...any,
) pgx.Row {
	row := transaction.queryRows[0]
	transaction.queryRows = transaction.queryRows[1:]
	return row
}

func (transaction *recordingAuthTransaction) Commit(context.Context) error {
	transaction.committed = true
	return nil
}

func (transaction *recordingAuthTransaction) Rollback(context.Context) error {
	if transaction.committed {
		return pgx.ErrTxClosed
	}
	transaction.rolledBack = true
	return nil
}

type scanAuthRow func(...any) error

func (row scanAuthRow) Scan(destinations ...any) error {
	return row(destinations...)
}

func mustSetScanValue[T any](destination any, value T) {
	target, ok := destination.(*T)
	if !ok {
		panic("unexpected scan destination")
	}
	*target = value
}
