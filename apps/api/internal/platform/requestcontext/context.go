package requestcontext

import "context"

type requestIDKey struct{}

// WithRequestID stores the correlation identifier in a standard context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestID returns the correlation identifier, if one has been assigned.
func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}
