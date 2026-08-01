package apperror

import (
	"errors"
	"net/http"
)

// Code is a stable, machine-readable application error identifier.
type Code string

// Code values are stable machine-readable classifications shared by every
// HTTP error response.
const (
	CodeInvalidRequest       Code = "INVALID_REQUEST"
	CodeNotFound             Code = "NOT_FOUND"
	CodeGitHubUserNotFound   Code = "GITHUB_USER_NOT_FOUND"
	CodeForbiddenOrigin      Code = "FORBIDDEN_ORIGIN"
	CodeRequestTimeout       Code = "REQUEST_TIMEOUT"
	CodeRateLimit            Code = "GITHUB_RATE_LIMIT_EXCEEDED"
	CodeGitHubAPI            Code = "GITHUB_API_ERROR"
	CodeDatabaseUnavailable  Code = "DATABASE_UNAVAILABLE"
	CodeAuthUnavailable      Code = "AUTH_UNAVAILABLE"
	CodeAuthentication       Code = "AUTHENTICATION_REQUIRED"
	CodeInvalidAuthState     Code = "INVALID_AUTH_STATE"
	CodeCSRFRejected         Code = "CSRF_REJECTED"
	CodeOAuthRejected        Code = "GITHUB_AUTHORIZATION_REJECTED"
	CodeAccountQuota         Code = "ACCOUNT_QUOTA_EXCEEDED"
	CodeVersionConflict      Code = "VERSION_CONFLICT"
	CodeDuplicateSavedSearch Code = "DUPLICATE_SAVED_SEARCH"
	CodeInternal             Code = "INTERNAL_SERVER_ERROR"
)

// Error carries safe client information while retaining an internal cause.
type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	cause      error
}

// Error returns the safe code and message; it never formats the wrapped cause.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Code) + ": " + e.Message
}

// Unwrap returns the internal cause for errors.Is and errors.As. Transport
// encoders must not serialize the returned error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// New constructs an application error without an underlying cause.
func New(code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

// Wrap preserves an internal cause without exposing it through the API.
func Wrap(code Code, message string, status int, cause error) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		cause:      cause,
	}
}

// From returns a safe application error. Unknown errors become a generic 500.
func From(err error) *Error {
	var applicationError *Error
	if errors.As(err, &applicationError) {
		return applicationError
	}

	return Wrap(
		CodeInternal,
		"An unexpected error occurred",
		http.StatusInternalServerError,
		err,
	)
}
