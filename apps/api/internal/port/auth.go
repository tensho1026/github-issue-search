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
	// AuthorizationURL builds the GitHub endpoint for a generated state and
	// RFC 7636 S256 challenge without persisting either value.
	AuthorizationURL(
		state auth.Secret,
		codeChallenge string,
	) string
	// Exchange trades a one-time code and verifier for a redacting access
	// token, honors ctx, and returns classified authentication errors.
	Exchange(
		ctx context.Context,
		code auth.Secret,
		codeVerifier auth.Secret,
	) (auth.Secret, error)
	// FetchIdentity resolves the minimum public identity, honors ctx, and must
	// not retain or expose the access token.
	FetchIdentity(
		ctx context.Context,
		accessToken auth.Secret,
	) (auth.GitHubIdentity, error)
}

// AuthRepository persists only hashed OAuth/session credentials and the
// authenticated account identity.
type AuthRepository interface {
	// SaveAuthorizationState persists only the state hash and bounded flow
	// metadata; plaintext browser credentials are forbidden.
	SaveAuthorizationState(
		ctx context.Context,
		state auth.AuthorizationState,
	) error
	// ConsumeAuthorizationState atomically deletes one unexpired state hash and
	// returns its validated same-origin path. Replay returns
	// auth.ErrAuthorizationStateNotFound.
	ConsumeAuthorizationState(
		ctx context.Context,
		stateHash auth.Digest,
		now time.Time,
	) (string, error)
	// UpsertIdentityAndCreateSession atomically links the GitHub identity and
	// inserts a hashed session while enforcing the active-session bound.
	UpsertIdentityAndCreateSession(
		ctx context.Context,
		draft auth.SessionDraft,
	) (auth.Session, error)
	// FindSession returns one unexpired session by token hash and updates
	// bounded last-seen metadata without accepting plaintext credentials.
	FindSession(
		ctx context.Context,
		tokenHash auth.Digest,
		now time.Time,
	) (auth.Session, error)
	// RotateSession atomically revokes the current hash and inserts the supplied
	// replacement for the same account.
	RotateSession(
		ctx context.Context,
		currentTokenHash auth.Digest,
		draft auth.SessionDraft,
		now time.Time,
	) (auth.Session, error)
	// RevokeSession invalidates one token hash. Unknown or expired sessions
	// return auth.ErrSessionNotFound.
	RevokeSession(
		ctx context.Context,
		tokenHash auth.Digest,
		now time.Time,
	) error
	// RevokeAllSessions invalidates every active session owned by accountID.
	RevokeAllSessions(
		ctx context.Context,
		accountID account.ID,
		now time.Time,
	) error
}
