// Package requestcontext carries trusted request-scoped values across the
// transport and application boundaries.
package requestcontext

import (
	"context"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

type requestIDKey struct{}
type principalKey struct{}

// WithRequestID stores the correlation identifier in a standard context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestID returns the correlation identifier, if one has been assigned.
func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

// WithPrincipal attaches the server-validated authentication principal.
func WithPrincipal(ctx context.Context, principal auth.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// Principal returns the server-validated authentication principal.
func Principal(ctx context.Context) (auth.Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(auth.Principal)
	return principal, ok
}
