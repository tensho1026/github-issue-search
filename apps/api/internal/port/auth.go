package port

import (
	"context"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

// GitHubOAuth exchanges a one-time authorization code and resolves the
// minimum public GitHub identity needed for an IssueScout account.
type GitHubOAuth interface {
	AuthorizationURL(
		state auth.Secret,
		codeChallenge string,
	) string
	Exchange(
		ctx context.Context,
		code auth.Secret,
		codeVerifier auth.Secret,
	) (auth.Secret, error)
	FetchIdentity(
		ctx context.Context,
		accessToken auth.Secret,
	) (auth.GitHubIdentity, error)
}

// AuthRepository persists only hashed OAuth/session credentials and the
// authenticated account identity.
type AuthRepository interface {
	SaveAuthorizationState(
		ctx context.Context,
		state auth.AuthorizationState,
	) error
	ConsumeAuthorizationState(
		ctx context.Context,
		stateHash auth.Digest,
		now time.Time,
	) (string, error)
	UpsertIdentityAndCreateSession(
		ctx context.Context,
		draft auth.SessionDraft,
	) (auth.Session, error)
	FindSession(
		ctx context.Context,
		tokenHash auth.Digest,
		now time.Time,
	) (auth.Session, error)
	RotateSession(
		ctx context.Context,
		currentTokenHash auth.Digest,
		draft auth.SessionDraft,
		now time.Time,
	) (auth.Session, error)
	RevokeSession(
		ctx context.Context,
		tokenHash auth.Digest,
		now time.Time,
	) error
	RevokeAllSessions(
		ctx context.Context,
		accountID account.ID,
		now time.Time,
	) error
}
