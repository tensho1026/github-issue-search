// Package apperror defines safe, stable application errors shared across
// adapters, use cases, and HTTP response encoding.
//
// Public codes and messages are intentionally separate from wrapped internal
// causes. Callers may use errors.Is and errors.As without exposing
// infrastructure details to API consumers.
package apperror
