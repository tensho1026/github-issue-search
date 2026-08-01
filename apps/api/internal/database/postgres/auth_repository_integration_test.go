package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

func TestAuthRepositoryAgainstConfiguredPostgreSQL(t *testing.T) {
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
	repository, repositoryErr := NewAuthRepository(pool)
	if repositoryErr != nil {
		t.Fatalf("NewAuthRepository() error = %v", repositoryErr)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	stateCredential := integrationCredential(1)
	state := auth.AuthorizationState{
		ID:         "5c634dd8-a6d8-467d-9c63-b1455c9a4045",
		StateHash:  auth.Hash(stateCredential.Value()),
		ReturnPath: "/workspace?tab=saved",
		ExpiresAt:  now.Add(10 * time.Minute),
		CreatedAt:  now,
	}
	if err := repository.SaveAuthorizationState(ctx, state); err != nil {
		t.Fatalf("SaveAuthorizationState() error = %v", err)
	}
	returnPath, err := repository.ConsumeAuthorizationState(
		ctx,
		state.StateHash,
		now.Add(time.Second),
	)
	if err != nil || returnPath != state.ReturnPath {
		t.Fatalf(
			"ConsumeAuthorizationState() = %q, %v",
			returnPath,
			err,
		)
	}
	if _, err := repository.ConsumeAuthorizationState(
		ctx,
		state.StateHash,
		now.Add(2*time.Second),
	); !errors.Is(err, auth.ErrAuthorizationStateNotFound) {
		t.Fatalf("replayed ConsumeAuthorizationState() error = %v", err)
	}

	firstDraft := integrationSessionDraft(t, now, integrationDraftValues{
		sessionID:  "8a57a939-5f3a-499f-9f53-c694d83806ec",
		identityID: "1c8a573b-608b-434f-aaac-d0b89f065491",
		accountID:  "93b9af3a-b1a8-42c8-bff8-55f8af59f5be",
		tokenFill:  2,
		csrfFill:   3,
	})
	firstSession, firstSessionErr := repository.UpsertIdentityAndCreateSession(
		ctx,
		firstDraft,
	)
	if firstSessionErr != nil {
		t.Fatalf(
			"first UpsertIdentityAndCreateSession() error = %v",
			firstSessionErr,
		)
	}
	found, findErr := repository.FindSession(
		ctx,
		firstDraft.TokenHash,
		now.Add(time.Second),
	)
	if findErr != nil ||
		found.AccountID != firstSession.AccountID ||
		found.Identity.Login.String() != "octocat" {
		t.Fatalf("FindSession() = %+v, %v", found, findErr)
	}

	secondDraft := integrationSessionDraft(t, now.Add(time.Minute), integrationDraftValues{
		sessionID:  "a5f701c8-a1cb-4b8d-b185-f8a40a6b3ab4",
		identityID: "48fdca8c-df3f-4fd5-b242-00c3af3d9f60",
		accountID:  "90d3f3fd-274f-40c2-b6d8-75359ddb9f43",
		tokenFill:  4,
		csrfFill:   5,
	})
	secondSession, secondSessionErr := repository.UpsertIdentityAndCreateSession(
		ctx,
		secondDraft,
	)
	if secondSessionErr != nil {
		t.Fatalf(
			"second UpsertIdentityAndCreateSession() error = %v",
			secondSessionErr,
		)
	}
	if secondSession.AccountID != firstSession.AccountID {
		t.Fatal("repeated GitHub identity created a second account")
	}

	rotatedDraft := integrationSessionDraft(
		t,
		now.Add(2*time.Minute),
		integrationDraftValues{
			sessionID: "04d433b6-ff4f-407d-8159-02fdb5a8f22d",
			accountID: firstSession.AccountID.String(),
			tokenFill: 6,
			csrfFill:  7,
		},
	)
	rotated, rotateErr := repository.RotateSession(
		ctx,
		secondDraft.TokenHash,
		rotatedDraft,
		now.Add(2*time.Minute),
	)
	if rotateErr != nil {
		t.Fatalf("RotateSession() error = %v", rotateErr)
	}
	if _, err := repository.FindSession(
		ctx,
		secondDraft.TokenHash,
		now.Add(3*time.Minute),
	); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("old FindSession() error = %v", err)
	}
	if _, err := repository.FindSession(
		ctx,
		rotated.TokenHash,
		now.Add(3*time.Minute),
	); err != nil {
		t.Fatalf("rotated FindSession() error = %v", err)
	}
	if err := repository.RevokeAllSessions(
		ctx,
		firstSession.AccountID,
		now.Add(4*time.Minute),
	); err != nil {
		t.Fatalf("RevokeAllSessions() error = %v", err)
	}
	for _, tokenHash := range []auth.Digest{
		firstDraft.TokenHash,
		rotated.TokenHash,
	} {
		if _, err := repository.FindSession(
			ctx,
			tokenHash,
			now.Add(5*time.Minute),
		); !errors.Is(err, auth.ErrSessionNotFound) {
			t.Fatalf("revoked FindSession() error = %v", err)
		}
	}
	assertPlaintextCredentialsAbsent(
		t,
		ctx,
		pool,
		stateCredential,
		integrationCredential(2),
		integrationCredential(3),
	)
}

type integrationDraftValues struct {
	sessionID  string
	identityID string
	accountID  string
	tokenFill  byte
	csrfFill   byte
}

func integrationSessionDraft(
	t *testing.T,
	now time.Time,
	values integrationDraftValues,
) auth.SessionDraft {
	t.Helper()
	accountID, err := account.ParseID(values.accountID)
	if err != nil {
		t.Fatalf("account.ParseID() error = %v", err)
	}
	login, err := user.ParseUsername("octocat")
	if err != nil {
		t.Fatalf("user.ParseUsername() error = %v", err)
	}
	return auth.SessionDraft{
		ID:         values.sessionID,
		IdentityID: values.identityID,
		AccountID:  accountID,
		TokenHash:  auth.Hash(integrationCredential(values.tokenFill).Value()),
		CSRFHash:   auth.Hash(integrationCredential(values.csrfFill).Value()),
		ExpiresAt:  now.Add(time.Hour),
		CreatedAt:  now,
		Identity: auth.GitHubIdentity{
			UserID:     583231,
			Login:      login,
			AvatarURL:  "https://avatars.githubusercontent.com/u/583231",
			ProfileURL: "https://github.com/octocat",
		},
		MaxActive: 10,
	}
}

func isolatedIntegrationPool(
	t *testing.T,
	ctx context.Context,
	databaseURL string,
) *Pool {
	t.Helper()
	settings := integrationPoolSettings()
	adminPool, openErr := Open(ctx, databaseURL, settings)
	if openErr != nil {
		t.Fatal("Open() rejected TEST_DATABASE_URL")
	}
	t.Cleanup(adminPool.Close)
	schema := integrationSchemaName(t)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, createErr := adminPool.client.Exec(
		ctx,
		"CREATE SCHEMA "+identifier,
	); createErr != nil {
		t.Fatal("create isolated authentication schema")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(
			context.Background(),
			30*time.Second,
		)
		defer cleanupCancel()
		if _, dropErr := adminPool.client.Exec(
			cleanupContext,
			"DROP SCHEMA "+identifier+" CASCADE",
		); dropErr != nil {
			t.Error("drop isolated authentication schema")
		}
	})
	return openIntegrationPoolInSchema(t, ctx, databaseURL, schema)
}

func assertPlaintextCredentialsAbsent(
	t *testing.T,
	ctx context.Context,
	pool *Pool,
	credentials ...auth.Secret,
) {
	t.Helper()
	for _, credential := range credentials {
		var count int
		if err := pool.client.QueryRow(
			ctx,
			`SELECT
			    (SELECT count(*)
			     FROM oauth_authorization_states
			     WHERE state_hash = $1::bytea)
			  + (SELECT count(*)
			     FROM auth_sessions
			     WHERE token_hash = $1::bytea
			        OR csrf_secret_hash = $1::bytea)`,
			[]byte(credential.Value()),
		).Scan(&count); err != nil {
			t.Fatal("inspect persisted credential representation")
		}
		if count != 0 {
			t.Fatal("repository persisted a plaintext browser credential")
		}
	}
}

func integrationCredential(fill byte) auth.Secret {
	return auth.NewSecret(base64.RawURLEncoding.EncodeToString(
		bytes.Repeat([]byte{fill}, 32),
	))
}
