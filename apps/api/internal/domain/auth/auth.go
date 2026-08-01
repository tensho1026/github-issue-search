// Package auth defines OAuth identity and server-session rules without
// depending on HTTP, GitHub payloads, or PostgreSQL.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

const MaximumReturnPathBytes = 2048

const opaqueCredentialBytes = 32

var (
	// ErrInvalidReturnPath reports a path that could escape the configured
	// frontend origin.
	ErrInvalidReturnPath = errors.New("invalid OAuth return path")
	// ErrAuthorizationStateNotFound covers unknown, consumed, and expired OAuth
	// state without revealing which condition occurred.
	ErrAuthorizationStateNotFound = errors.New("OAuth authorization state not found")
	// ErrSessionNotFound covers missing, revoked, and expired sessions.
	ErrSessionNotFound = errors.New("authentication session not found")
	// ErrCSRFTokenMismatch reports a missing or invalid CSRF credential.
	ErrCSRFTokenMismatch = errors.New("CSRF token mismatch")
	// ErrOAuthDenied reports that GitHub authorization was declined.
	ErrOAuthDenied = errors.New("GitHub authorization denied")
	// ErrOAuthRejected reports an invalid or unusable authorization code.
	ErrOAuthRejected = errors.New("GitHub authorization rejected")
	// ErrOAuthUnavailable reports a safe upstream or persistence failure.
	ErrOAuthUnavailable = errors.New("GitHub authentication unavailable")
	// ErrInvalidIdentity reports an incomplete upstream identity.
	ErrInvalidIdentity = errors.New("invalid GitHub identity")
	// ErrInvalidCredential reports a malformed opaque authentication value.
	ErrInvalidCredential = errors.New("invalid authentication credential")
)

// Digest is a fixed-size SHA-256 credential digest stored by persistence
// adapters instead of the source credential.
type Digest [sha256.Size]byte

// Hash returns the SHA-256 digest of an opaque credential.
func Hash(raw string) Digest {
	return sha256.Sum256([]byte(raw))
}

// IsOpaqueCredential reports whether a value is an unpadded base64url
// encoding of exactly 256 random bits.
func IsOpaqueCredential(raw string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	return err == nil && len(decoded) == opaqueCredentialBytes
}

// PKCEChallenge returns the RFC 7636 S256 challenge for an opaque verifier.
func PKCEChallenge(verifier Secret) (string, error) {
	if !IsOpaqueCredential(verifier.Value()) {
		return "", ErrInvalidCredential
	}
	digest := sha256.Sum256([]byte(verifier.Value()))
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

// Matches compares a candidate credential to the digest in constant time.
func (digest Digest) Matches(raw string) bool {
	candidate := Hash(raw)
	return subtle.ConstantTimeCompare(digest[:], candidate[:]) == 1
}

// Bytes returns a defensive copy suitable for a database argument.
func (digest Digest) Bytes() []byte {
	value := make([]byte, len(digest))
	copy(value, digest[:])
	return value
}

// DigestFromBytes validates and copies a database digest.
func DigestFromBytes(value []byte) (Digest, error) {
	var digest Digest
	if len(value) != len(digest) {
		return Digest{}, ErrSessionNotFound
	}
	copy(digest[:], value)
	return digest, nil
}

// Secret carries a short-lived credential while preventing accidental
// formatting. It must not be logged or persisted as plaintext.
type Secret struct {
	value string
}

// NewSecret wraps an already generated credential.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// Value exposes the credential only at its intended adapter boundary.
func (secret Secret) Value() string {
	return secret.value
}

// IsSet reports whether the secret contains a value.
func (secret Secret) IsSet() bool {
	return secret.value != ""
}

// String deliberately redacts the credential.
func (secret Secret) String() string {
	if !secret.IsSet() {
		return "<unset>"
	}
	return "<redacted>"
}

// GoString prevents %#v formatting from revealing the credential.
func (secret Secret) GoString() string {
	return secret.String()
}

// GitHubIdentity is the minimum public identity retained for sign-in.
type GitHubIdentity struct {
	UserID     int64
	Login      user.Username
	AvatarURL  string
	ProfileURL string
}

// Validate rejects incomplete or non-HTTPS identity URLs.
func (identity GitHubIdentity) Validate() error {
	if identity.UserID <= 0 || identity.Login == "" {
		return ErrInvalidIdentity
	}
	for _, raw := range []string{identity.AvatarURL, identity.ProfileURL} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
			parsed.User != nil {
			return ErrInvalidIdentity
		}
	}
	return nil
}

// AuthorizationState is a single-use, short-lived OAuth transaction record.
type AuthorizationState struct {
	ID         string
	StateHash  Digest
	ReturnPath string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// SessionDraft contains only hashes and public identity material for an
// atomic account/session write.
type SessionDraft struct {
	ID         string
	IdentityID string
	AccountID  account.ID
	TokenHash  Digest
	CSRFHash   Digest
	ExpiresAt  time.Time
	CreatedAt  time.Time
	Identity   GitHubIdentity
	MaxActive  int
}

// Session is an active server-side authentication session.
type Session struct {
	ID        string
	AccountID account.ID
	TokenHash Digest
	CSRFHash  Digest
	ExpiresAt time.Time
	Identity  GitHubIdentity
}

// Principal is the authenticated request identity. CSRF is retained only as a
// redacting in-memory value so the frontend can echo it in a mutation header.
type Principal struct {
	Session   Session
	CSRFToken Secret
}

// ValidateReturnPath accepts only a bounded root-relative frontend location.
func ValidateReturnPath(raw string) (string, error) {
	if raw == "" {
		return "/", nil
	}
	if len(raw) > MaximumReturnPathBytes ||
		!strings.HasPrefix(raw, "/") ||
		strings.HasPrefix(raw, "//") ||
		strings.Contains(raw, "\\") {
		return "", ErrInvalidReturnPath
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil ||
		parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/") {
		return "", ErrInvalidReturnPath
	}
	return parsed.String(), nil
}
