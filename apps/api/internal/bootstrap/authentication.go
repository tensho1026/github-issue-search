package bootstrap

import (
	"fmt"

	"github.com/tensho1026/github-issue-search/apps/api/internal/client/githuboauth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/database/postgres"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authcrypto"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

// AuthenticationDependencies contains optional account-authentication
// components. Both values are nil when OAuth is intentionally disabled.
type AuthenticationDependencies struct {
	Service   usecase.Authentication
	FlowCodec *authcrypto.FlowCodec
}

// NewAuthentication composes GitHub OAuth, hashed PostgreSQL sessions,
// cryptographic credential generation, and the encrypted browser-flow codec.
func NewAuthentication(
	cfg config.Config,
	databasePool *postgres.Pool,
) (AuthenticationDependencies, error) {
	if !cfg.AuthEnabled {
		return AuthenticationDependencies{}, nil
	}
	if databasePool == nil {
		return AuthenticationDependencies{}, fmt.Errorf(
			"compose authentication: database pool is required",
		)
	}
	repository, err := postgres.NewAuthRepository(databasePool)
	if err != nil {
		return AuthenticationDependencies{}, fmt.Errorf(
			"compose authentication repository: %w",
			err,
		)
	}
	oauthClient, err := githuboauth.NewClient(
		cfg.GitHubOAuthAuthorizeURL,
		cfg.GitHubOAuthTokenURL,
		cfg.GitHubAPIBaseURL,
		cfg.GitHubOAuthCallbackURL,
		cfg.GitHubOAuthClientID,
		cfg.GitHubOAuthClientSecret.Value(),
		cfg.GitHubRequestTimeout,
	)
	if err != nil {
		return AuthenticationDependencies{}, fmt.Errorf(
			"compose GitHub OAuth client: %w",
			err,
		)
	}
	authentication, err := usecase.NewAuthentication(
		oauthClient,
		repository,
		authcrypto.NewGenerator(),
		cfg.AuthStateTTL,
		cfg.AuthSessionTTL,
		cfg.AuthMaxSessions,
	)
	if err != nil {
		return AuthenticationDependencies{}, err
	}
	flowCodec, err := authcrypto.NewFlowCodec(
		cfg.AuthFlowEncryptionKey.Value(),
	)
	if err != nil {
		return AuthenticationDependencies{}, fmt.Errorf(
			"compose OAuth flow codec: %w",
			err,
		)
	}
	return AuthenticationDependencies{
		Service:   authentication,
		FlowCodec: flowCodec,
	}, nil
}
