package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
)

func TestAuthenticationStartPersistsHashAndBuildsPKCE(t *testing.T) {
	repository := &authRepositoryStub{}
	oauth := &oauthStub{authorizationURL: "https://github.example/authorize"}
	generator := &credentialGeneratorStub{
		uuids:  []string{"624fc28b-46aa-468b-86a3-f112d69356cb"},
		opaque: []auth.Secret{opaqueSecret(1), opaqueSecret(2)},
	}
	service := newAuthenticationForTest(t, oauth, repository, generator)
	output, err := service.Start(context.Background(), "/workspace?tab=saved")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if output.AuthorizationURL != oauth.authorizationURL ||
		output.ReturnPath != "/workspace?tab=saved" ||
		repository.savedState.StateHash != auth.Hash(output.State.Value()) ||
		repository.savedState.ReturnPath != output.ReturnPath ||
		oauth.state.Value() != output.State.Value() ||
		oauth.challenge == "" {
		t.Fatalf("Start() output = %+v state = %+v", output, repository.savedState)
	}
	if repository.savedState.StateHash.Matches(output.CodeVerifier.Value()) {
		t.Fatal("state hash unexpectedly matched the PKCE verifier")
	}
}

func TestAuthenticationCompleteLinksIdentityAndReturnsCookieCredentials(
	t *testing.T,
) {
	accountID := mustAuthenticationAccountID(t)
	repository := &authRepositoryStub{returnPath: "/workspace"}
	oauth := &oauthStub{
		accessToken: auth.NewSecret("upstream-token"),
		identity:    testGitHubIdentity(t),
	}
	sessionToken := opaqueSecret(3)
	csrfToken := opaqueSecret(4)
	generator := &credentialGeneratorStub{
		uuids: []string{
			"624fc28b-46aa-468b-86a3-f112d69356cb",
			accountID.String(),
			"6ca6dfc4-0114-44fb-a9f8-d703f8c9a8b2",
		},
		opaque: []auth.Secret{sessionToken, csrfToken},
	}
	service := newAuthenticationForTest(t, oauth, repository, generator)
	state := opaqueSecret(1)
	output, err := service.Complete(context.Background(), CompleteOAuthInput{
		State:          state,
		CodeVerifier:   opaqueSecret(2),
		Code:           auth.NewSecret("authorization-code"),
		FlowReturnPath: "/workspace",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if repository.consumedState != auth.Hash(state.Value()) ||
		repository.sessionDraft.TokenHash != auth.Hash(sessionToken.Value()) ||
		repository.sessionDraft.CSRFHash != auth.Hash(csrfToken.Value()) ||
		output.SessionToken.Value() != sessionToken.Value() ||
		output.CSRFToken.Value() != csrfToken.Value() {
		t.Fatalf("Complete() output = %+v draft = %+v", output, repository.sessionDraft)
	}
	if oauth.exchangedCode.Value() != "authorization-code" ||
		oauth.fetchedToken.Value() != "upstream-token" {
		t.Fatal("OAuth adapter did not receive the expected in-memory credentials")
	}
}

func TestAuthenticationRejectsReplayAndReturnPathMismatch(t *testing.T) {
	tests := map[string]*authRepositoryStub{
		"replayed state": {
			consumeErr: auth.ErrAuthorizationStateNotFound,
		},
		"mismatched path": {
			returnPath: "/different",
		},
	}
	for name, repository := range tests {
		t.Run(name, func(t *testing.T) {
			service := newAuthenticationForTest(
				t,
				&oauthStub{},
				repository,
				&credentialGeneratorStub{},
			)
			_, err := service.Complete(
				context.Background(),
				CompleteOAuthInput{
					State:          opaqueSecret(1),
					CodeVerifier:   opaqueSecret(2),
					Code:           auth.NewSecret("code"),
					FlowReturnPath: "/workspace",
				},
			)
			if apperror.From(err).Code != apperror.CodeInvalidAuthState {
				t.Fatalf("Complete() error = %v", err)
			}
		})
	}
}

func TestAuthenticationSkipsDatabaseForMalformedCookie(t *testing.T) {
	repository := &authRepositoryStub{}
	service := newAuthenticationForTest(
		t,
		&oauthStub{},
		repository,
		&credentialGeneratorStub{},
	)
	_, err := service.Authenticate(
		context.Background(),
		auth.NewSecret("malformed"),
		auth.NewSecret("malformed"),
	)
	if apperror.From(err).Code != apperror.CodeAuthentication {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if repository.findCalls != 0 {
		t.Fatalf("FindSession() calls = %d", repository.findCalls)
	}
}

func TestAuthenticationMapsDatabaseFailureWithoutLeakingDetails(t *testing.T) {
	repository := &authRepositoryStub{
		findErr: errors.New(
			"private database host and credential detail",
		),
	}
	service := newAuthenticationForTest(
		t,
		&oauthStub{},
		repository,
		&credentialGeneratorStub{},
	)
	_, err := service.Authenticate(
		context.Background(),
		opaqueSecret(1),
		opaqueSecret(2),
	)
	applicationError := apperror.From(err)
	if applicationError.Code != apperror.CodeAuthUnavailable ||
		applicationError.HTTPStatus != 503 {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if strings.Contains(
		applicationError.Error(),
		"private database",
	) {
		t.Fatal("authentication error exposed a database detail")
	}
}

func TestAuthenticationValidatesCSRFAndRotatesSession(t *testing.T) {
	accountID := mustAuthenticationAccountID(t)
	currentToken := opaqueSecret(1)
	csrfToken := opaqueSecret(2)
	session := auth.Session{
		ID:        "624fc28b-46aa-468b-86a3-f112d69356cb",
		AccountID: accountID,
		TokenHash: auth.Hash(currentToken.Value()),
		CSRFHash:  auth.Hash(csrfToken.Value()),
		ExpiresAt: time.Now().Add(time.Hour),
		Identity:  testGitHubIdentity(t),
	}
	repository := &authRepositoryStub{foundSession: session}
	nextToken := opaqueSecret(3)
	nextCSRF := opaqueSecret(4)
	generator := &credentialGeneratorStub{
		uuids:  []string{"6ca6dfc4-0114-44fb-a9f8-d703f8c9a8b2"},
		opaque: []auth.Secret{nextToken, nextCSRF},
	}
	service := newAuthenticationForTest(
		t,
		&oauthStub{},
		repository,
		generator,
	)
	principal, authenticateErr := service.Authenticate(
		context.Background(),
		currentToken,
		csrfToken,
	)
	if authenticateErr != nil {
		t.Fatalf("Authenticate() error = %v", authenticateErr)
	}
	if validationErr := service.ValidateCSRF(
		principal,
		csrfToken,
	); validationErr != nil {
		t.Fatalf("ValidateCSRF() error = %v", validationErr)
	}
	if validationErr := service.ValidateCSRF(
		principal,
		opaqueSecret(9),
	); apperror.From(
		validationErr,
	).Code != apperror.CodeCSRFRejected {
		t.Fatalf("mismatched ValidateCSRF() error = %v", validationErr)
	}
	refreshed, err := service.Refresh(context.Background(), principal)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.SessionToken.Value() != nextToken.Value() ||
		repository.rotatedCurrentHash != session.TokenHash ||
		repository.sessionDraft.AccountID != accountID {
		t.Fatalf("Refresh() output = %+v draft = %+v", refreshed, repository.sessionDraft)
	}
	if err := service.Logout(context.Background(), principal); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if repository.revokedHash != session.TokenHash {
		t.Fatal("Logout() revoked the wrong session")
	}
}

func newAuthenticationForTest(
	t *testing.T,
	oauthClient *oauthStub,
	repository *authRepositoryStub,
	generator *credentialGeneratorStub,
) *authentication {
	t.Helper()
	service, err := NewAuthentication(
		oauthClient,
		repository,
		generator,
		10*time.Minute,
		12*time.Hour,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	concrete, ok := service.(*authentication)
	if !ok {
		t.Fatal("NewAuthentication() returned an unexpected implementation")
	}
	concrete.now = func() time.Time {
		return time.Date(2026, time.August, 1, 4, 0, 0, 0, time.UTC)
	}
	return concrete
}

func opaqueSecret(fill byte) auth.Secret {
	return auth.NewSecret(base64.RawURLEncoding.EncodeToString(
		bytes.Repeat([]byte{fill}, 32),
	))
}

func testGitHubIdentity(t *testing.T) auth.GitHubIdentity {
	t.Helper()
	login, err := user.ParseUsername("octocat")
	if err != nil {
		t.Fatal(err)
	}
	return auth.GitHubIdentity{
		UserID:     583231,
		Login:      login,
		AvatarURL:  "https://avatars.githubusercontent.com/u/583231",
		ProfileURL: "https://github.com/octocat",
	}
}

func mustAuthenticationAccountID(t *testing.T) account.ID {
	t.Helper()
	id, err := account.ParseID("8bbfd7ed-a424-4ec3-a1b8-647006da1816")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type credentialGeneratorStub struct {
	opaque []auth.Secret
	uuids  []string
	err    error
}

func (generator *credentialGeneratorStub) Opaque() (auth.Secret, error) {
	if generator.err != nil {
		return auth.Secret{}, generator.err
	}
	value := generator.opaque[0]
	generator.opaque = generator.opaque[1:]
	return value, nil
}

func (generator *credentialGeneratorStub) UUID() (string, error) {
	if generator.err != nil {
		return "", generator.err
	}
	value := generator.uuids[0]
	generator.uuids = generator.uuids[1:]
	return value, nil
}

type oauthStub struct {
	authorizationURL string
	state            auth.Secret
	challenge        string
	accessToken      auth.Secret
	identity         auth.GitHubIdentity
	exchangeErr      error
	identityErr      error
	exchangedCode    auth.Secret
	fetchedToken     auth.Secret
}

func (oauth *oauthStub) AuthorizationURL(
	state auth.Secret,
	challenge string,
) string {
	oauth.state = state
	oauth.challenge = challenge
	return oauth.authorizationURL
}

func (oauth *oauthStub) Exchange(
	_ context.Context,
	code auth.Secret,
	_ auth.Secret,
) (auth.Secret, error) {
	oauth.exchangedCode = code
	return oauth.accessToken, oauth.exchangeErr
}

func (oauth *oauthStub) FetchIdentity(
	_ context.Context,
	token auth.Secret,
) (auth.GitHubIdentity, error) {
	oauth.fetchedToken = token
	return oauth.identity, oauth.identityErr
}

type authRepositoryStub struct {
	savedState         auth.AuthorizationState
	saveErr            error
	consumedState      auth.Digest
	returnPath         string
	consumeErr         error
	sessionDraft       auth.SessionDraft
	createErr          error
	foundSession       auth.Session
	findErr            error
	findCalls          int
	rotatedCurrentHash auth.Digest
	rotateErr          error
	revokedHash        auth.Digest
	revokeErr          error
}

func (repository *authRepositoryStub) SaveAuthorizationState(
	_ context.Context,
	state auth.AuthorizationState,
) error {
	repository.savedState = state
	return repository.saveErr
}

func (repository *authRepositoryStub) ConsumeAuthorizationState(
	_ context.Context,
	stateHash auth.Digest,
	_ time.Time,
) (string, error) {
	repository.consumedState = stateHash
	return repository.returnPath, repository.consumeErr
}

func (repository *authRepositoryStub) UpsertIdentityAndCreateSession(
	_ context.Context,
	draft auth.SessionDraft,
) (auth.Session, error) {
	repository.sessionDraft = draft
	return sessionFromAuthDraft(draft), repository.createErr
}

func (repository *authRepositoryStub) FindSession(
	_ context.Context,
	_ auth.Digest,
	_ time.Time,
) (auth.Session, error) {
	repository.findCalls++
	return repository.foundSession, repository.findErr
}

func (repository *authRepositoryStub) RotateSession(
	_ context.Context,
	currentTokenHash auth.Digest,
	draft auth.SessionDraft,
	_ time.Time,
) (auth.Session, error) {
	repository.rotatedCurrentHash = currentTokenHash
	repository.sessionDraft = draft
	return sessionFromAuthDraft(draft), repository.rotateErr
}

func (repository *authRepositoryStub) RevokeSession(
	_ context.Context,
	tokenHash auth.Digest,
	_ time.Time,
) error {
	repository.revokedHash = tokenHash
	return repository.revokeErr
}

func (repository *authRepositoryStub) RevokeAllSessions(
	context.Context,
	account.ID,
	time.Time,
) error {
	return nil
}

func sessionFromAuthDraft(draft auth.SessionDraft) auth.Session {
	return auth.Session{
		ID:        draft.ID,
		AccountID: draft.AccountID,
		TokenHash: draft.TokenHash,
		CSRFHash:  draft.CSRFHash,
		ExpiresAt: draft.ExpiresAt,
		Identity:  draft.Identity,
	}
}
