package usecase

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// OAuthStartOutput contains the short-lived browser binding and redirect for
// one GitHub authorization attempt.
type OAuthStartOutput struct {
	AuthorizationURL string
	State            auth.Secret
	CodeVerifier     auth.Secret
	ReturnPath       string
	ExpiresAt        time.Time
}

// CompleteOAuthInput binds the callback query to the encrypted flow cookie.
type CompleteOAuthInput struct {
	State          auth.Secret
	CodeVerifier   auth.Secret
	Code           auth.Secret
	FlowReturnPath string
}

// AuthSessionOutput contains credentials that are written only to secure
// cookies and the public session identity returned by the API.
type AuthSessionOutput struct {
	Session      auth.Session
	SessionToken auth.Secret
	CSRFToken    auth.Secret
}

// Authentication coordinates OAuth, PKCE, identity linking, session
// authentication, rotation, CSRF validation, and revocation.
type Authentication interface {
	// Start validates a same-origin return path, persists a single-use state
	// hash, and returns short-lived redacting browser credentials.
	Start(
		ctx context.Context,
		returnPath string,
	) (OAuthStartOutput, error)
	// Complete consumes the flow exactly once, exchanges PKCE credentials,
	// links the identity, and creates a bounded server session.
	Complete(
		ctx context.Context,
		input CompleteOAuthInput,
	) (AuthSessionOutput, error)
	// Deny consumes the supplied state and returns its validated path without
	// exchanging an authorization code.
	Deny(
		ctx context.Context,
		state auth.Secret,
		flowReturnPath string,
	) (string, error)
	// Authenticate validates opaque cookie credentials, loads an unexpired
	// server session, and never accepts an account ID from the caller.
	Authenticate(
		ctx context.Context,
		sessionToken auth.Secret,
		csrfToken auth.Secret,
	) (auth.Principal, error)
	// ValidateCSRF compares the header, cookie, and stored digest in constant
	// time for state-changing requests.
	ValidateCSRF(principal auth.Principal, headerToken auth.Secret) error
	// Refresh atomically revokes the current session and returns newly generated
	// session and CSRF credentials.
	Refresh(
		ctx context.Context,
		principal auth.Principal,
	) (AuthSessionOutput, error)
	// Logout revokes the principal's current session and honors ctx.
	Logout(ctx context.Context, principal auth.Principal) error
}

type credentialGenerator interface {
	Opaque() (auth.Secret, error)
	UUID() (string, error)
}

type authentication struct {
	oauth       port.GitHubOAuth
	repository  port.AuthRepository
	generator   credentialGenerator
	stateTTL    time.Duration
	sessionTTL  time.Duration
	maxSessions int
	now         func() time.Time
}

// NewAuthentication creates the optional account authentication application
// service.
func NewAuthentication(
	oauth port.GitHubOAuth,
	repository port.AuthRepository,
	generator credentialGenerator,
	stateTTL time.Duration,
	sessionTTL time.Duration,
	maxSessions int,
) (Authentication, error) {
	if oauth == nil || repository == nil || generator == nil ||
		stateTTL <= 0 || stateTTL > 15*time.Minute ||
		sessionTTL <= 0 || sessionTTL > 7*24*time.Hour ||
		maxSessions < 1 || maxSessions > 50 {
		return nil, fmt.Errorf("compose authentication: invalid dependency or bound")
	}
	return &authentication{
		oauth:       oauth,
		repository:  repository,
		generator:   generator,
		stateTTL:    stateTTL,
		sessionTTL:  sessionTTL,
		maxSessions: maxSessions,
		now:         time.Now,
	}, nil
}

func (service *authentication) Start(
	ctx context.Context,
	rawReturnPath string,
) (OAuthStartOutput, error) {
	returnPath, err := auth.ValidateReturnPath(rawReturnPath)
	if err != nil {
		return OAuthStartOutput{}, apperror.Wrap(
			apperror.CodeInvalidRequest,
			"Return path is invalid",
			http.StatusBadRequest,
			err,
		)
	}
	stateID, err := service.generator.UUID()
	if err != nil {
		return OAuthStartOutput{}, internalAuthError(err)
	}
	state, err := service.generator.Opaque()
	if err != nil {
		return OAuthStartOutput{}, internalAuthError(err)
	}
	verifier, err := service.generator.Opaque()
	if err != nil {
		return OAuthStartOutput{}, internalAuthError(err)
	}
	challenge, err := auth.PKCEChallenge(verifier)
	if err != nil {
		return OAuthStartOutput{}, internalAuthError(err)
	}
	now := service.now().UTC()
	expiresAt := now.Add(service.stateTTL)
	if err := service.repository.SaveAuthorizationState(
		ctx,
		auth.AuthorizationState{
			ID:         stateID,
			StateHash:  auth.Hash(state.Value()),
			ReturnPath: returnPath,
			ExpiresAt:  expiresAt,
			CreatedAt:  now,
		},
	); err != nil {
		return OAuthStartOutput{}, mapAuthDependencyError(err)
	}
	return OAuthStartOutput{
		AuthorizationURL: service.oauth.AuthorizationURL(state, challenge),
		State:            state,
		CodeVerifier:     verifier,
		ReturnPath:       returnPath,
		ExpiresAt:        expiresAt,
	}, nil
}

func (service *authentication) Complete(
	ctx context.Context,
	input CompleteOAuthInput,
) (AuthSessionOutput, error) {
	if !auth.IsOpaqueCredential(input.State.Value()) ||
		!auth.IsOpaqueCredential(input.CodeVerifier.Value()) ||
		!input.Code.IsSet() {
		return AuthSessionOutput{}, invalidAuthState(auth.ErrInvalidCredential)
	}
	if _, err := service.consumeState(
		ctx,
		input.State,
		input.FlowReturnPath,
	); err != nil {
		return AuthSessionOutput{}, err
	}
	accessToken, err := service.oauth.Exchange(
		ctx,
		input.Code,
		input.CodeVerifier,
	)
	if err != nil {
		return AuthSessionOutput{}, mapOAuthError(err)
	}
	identity, err := service.oauth.FetchIdentity(ctx, accessToken)
	if err != nil {
		return AuthSessionOutput{}, mapOAuthError(err)
	}
	draft, sessionToken, csrfToken, err := service.newSessionDraft(
		identity,
		account.ID{},
		true,
	)
	if err != nil {
		return AuthSessionOutput{}, internalAuthError(err)
	}
	session, err := service.repository.UpsertIdentityAndCreateSession(
		ctx,
		draft,
	)
	if err != nil {
		return AuthSessionOutput{}, mapAuthDependencyError(err)
	}
	return AuthSessionOutput{
		Session:      session,
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
	}, nil
}

func (service *authentication) Deny(
	ctx context.Context,
	state auth.Secret,
	flowReturnPath string,
) (string, error) {
	if !auth.IsOpaqueCredential(state.Value()) {
		return "", invalidAuthState(auth.ErrInvalidCredential)
	}
	return service.consumeState(ctx, state, flowReturnPath)
}

func (service *authentication) Authenticate(
	ctx context.Context,
	sessionToken auth.Secret,
	csrfToken auth.Secret,
) (auth.Principal, error) {
	if !auth.IsOpaqueCredential(sessionToken.Value()) ||
		!auth.IsOpaqueCredential(csrfToken.Value()) {
		return auth.Principal{}, authenticationRequired(
			auth.ErrSessionNotFound,
		)
	}
	session, err := service.repository.FindSession(
		ctx,
		auth.Hash(sessionToken.Value()),
		service.now().UTC(),
	)
	if err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			return auth.Principal{}, authenticationRequired(err)
		}
		return auth.Principal{}, mapAuthDependencyError(err)
	}
	if !session.CSRFHash.Matches(csrfToken.Value()) {
		return auth.Principal{}, csrfRejected(auth.ErrCSRFTokenMismatch)
	}
	return auth.Principal{
		Session:   session,
		CSRFToken: csrfToken,
	}, nil
}

func (service *authentication) ValidateCSRF(
	principal auth.Principal,
	headerToken auth.Secret,
) error {
	if !auth.IsOpaqueCredential(headerToken.Value()) ||
		!principal.Session.CSRFHash.Matches(headerToken.Value()) ||
		subtle.ConstantTimeCompare(
			[]byte(principal.CSRFToken.Value()),
			[]byte(headerToken.Value()),
		) != 1 {
		return csrfRejected(auth.ErrCSRFTokenMismatch)
	}
	return nil
}

func (service *authentication) Refresh(
	ctx context.Context,
	principal auth.Principal,
) (AuthSessionOutput, error) {
	draft, sessionToken, csrfToken, err := service.newSessionDraft(
		principal.Session.Identity,
		principal.Session.AccountID,
		false,
	)
	if err != nil {
		return AuthSessionOutput{}, internalAuthError(err)
	}
	session, err := service.repository.RotateSession(
		ctx,
		principal.Session.TokenHash,
		draft,
		service.now().UTC(),
	)
	if err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			return AuthSessionOutput{}, authenticationRequired(err)
		}
		return AuthSessionOutput{}, mapAuthDependencyError(err)
	}
	return AuthSessionOutput{
		Session:      session,
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
	}, nil
}

func (service *authentication) Logout(
	ctx context.Context,
	principal auth.Principal,
) error {
	err := service.repository.RevokeSession(
		ctx,
		principal.Session.TokenHash,
		service.now().UTC(),
	)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return authenticationRequired(err)
	}
	if err != nil {
		return mapAuthDependencyError(err)
	}
	return nil
}

func (service *authentication) consumeState(
	ctx context.Context,
	state auth.Secret,
	flowReturnPath string,
) (string, error) {
	validatedPath, err := auth.ValidateReturnPath(flowReturnPath)
	if err != nil {
		return "", invalidAuthState(err)
	}
	returnPath, err := service.repository.ConsumeAuthorizationState(
		ctx,
		auth.Hash(state.Value()),
		service.now().UTC(),
	)
	if errors.Is(err, auth.ErrAuthorizationStateNotFound) {
		return "", invalidAuthState(err)
	}
	if err != nil {
		return "", mapAuthDependencyError(err)
	}
	if returnPath != validatedPath {
		return "", invalidAuthState(auth.ErrAuthorizationStateNotFound)
	}
	return returnPath, nil
}

func (service *authentication) newSessionDraft(
	identity auth.GitHubIdentity,
	existingAccountID account.ID,
	includeIdentityID bool,
) (auth.SessionDraft, auth.Secret, auth.Secret, error) {
	now := service.now().UTC()
	sessionID, err := service.generator.UUID()
	if err != nil {
		return auth.SessionDraft{}, auth.Secret{}, auth.Secret{}, err
	}
	accountID := existingAccountID
	if accountID == (account.ID{}) {
		rawAccountID, generateErr := service.generator.UUID()
		if generateErr != nil {
			return auth.SessionDraft{}, auth.Secret{}, auth.Secret{}, generateErr
		}
		accountID, err = account.ParseID(rawAccountID)
		if err != nil {
			return auth.SessionDraft{}, auth.Secret{}, auth.Secret{}, err
		}
	}
	identityID := ""
	if includeIdentityID {
		identityID, err = service.generator.UUID()
		if err != nil {
			return auth.SessionDraft{}, auth.Secret{}, auth.Secret{}, err
		}
	}
	sessionToken, err := service.generator.Opaque()
	if err != nil {
		return auth.SessionDraft{}, auth.Secret{}, auth.Secret{}, err
	}
	csrfToken, err := service.generator.Opaque()
	if err != nil {
		return auth.SessionDraft{}, auth.Secret{}, auth.Secret{}, err
	}
	return auth.SessionDraft{
		ID:         sessionID,
		IdentityID: identityID,
		AccountID:  accountID,
		TokenHash:  auth.Hash(sessionToken.Value()),
		CSRFHash:   auth.Hash(csrfToken.Value()),
		ExpiresAt:  now.Add(service.sessionTTL),
		CreatedAt:  now,
		Identity:   identity,
		MaxActive:  service.maxSessions,
	}, sessionToken, csrfToken, nil
}

func mapOAuthError(err error) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return apperror.Wrap(
			apperror.CodeRequestTimeout,
			"Authentication request timed out",
			http.StatusGatewayTimeout,
			err,
		)
	case errors.Is(err, auth.ErrOAuthRejected),
		errors.Is(err, auth.ErrInvalidIdentity):
		return apperror.Wrap(
			apperror.CodeOAuthRejected,
			"GitHub authorization could not be accepted",
			http.StatusBadRequest,
			err,
		)
	default:
		return apperror.Wrap(
			apperror.CodeAuthUnavailable,
			"GitHub sign-in is temporarily unavailable",
			http.StatusBadGateway,
			err,
		)
	}
}

func mapAuthDependencyError(err error) error {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return apperror.Wrap(
			apperror.CodeRequestTimeout,
			"Authentication request timed out",
			http.StatusGatewayTimeout,
			err,
		)
	}
	return apperror.Wrap(
		apperror.CodeAuthUnavailable,
		"Account sign-in is temporarily unavailable",
		http.StatusServiceUnavailable,
		err,
	)
}

func invalidAuthState(err error) error {
	return apperror.Wrap(
		apperror.CodeInvalidAuthState,
		"Authorization state is invalid or expired",
		http.StatusBadRequest,
		err,
	)
}

func authenticationRequired(err error) error {
	return apperror.Wrap(
		apperror.CodeAuthentication,
		"Authentication is required",
		http.StatusUnauthorized,
		err,
	)
}

func csrfRejected(err error) error {
	return apperror.Wrap(
		apperror.CodeCSRFRejected,
		"CSRF validation failed",
		http.StatusForbidden,
		err,
	)
}

func internalAuthError(err error) error {
	return apperror.Wrap(
		apperror.CodeInternal,
		"An unexpected error occurred",
		http.StatusInternalServerError,
		err,
	)
}

var _ Authentication = (*authentication)(nil)
