package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
)

const errorCodeContextKey = "issuescout.error_code"

// Meta is correlation, response-time, and optional upstream quota metadata
// included in every API envelope.
type Meta struct {
	RequestID          string    `json:"requestId"`
	Timestamp          time.Time `json:"timestamp"`
	RateLimitRemaining *int      `json:"rateLimitRemaining,omitempty"`
}

// MetaOptions supplies optional metadata for one successful response.
type MetaOptions struct {
	RateLimitRemaining *int
}

type successEnvelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type errorBody struct {
	Code    apperror.Code `json:"code"`
	Message string        `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

// Responder writes the single API response contract with an injectable clock.
type Responder struct {
	now func() time.Time
}

// NewResponder constructs a response writer using the process clock.
func NewResponder() Responder {
	return Responder{now: time.Now}
}

// NewResponderWithClock constructs a response writer with an injected clock.
// The clock must be non-nil and should be safe for concurrent calls.
func NewResponderWithClock(now func() time.Time) Responder {
	return Responder{now: now}
}

// Data writes a success envelope without optional rate-limit metadata.
func (r Responder) Data(ctx *gin.Context, status int, data any) {
	r.DataWithMeta(ctx, status, data, MetaOptions{})
}

// DataWithMeta writes a success envelope and caller-supplied optional metadata.
func (r Responder) DataWithMeta(
	ctx *gin.Context,
	status int,
	data any,
	options MetaOptions,
) {
	ctx.JSON(
		status,
		successEnvelope{Data: data, Meta: r.meta(ctx, options)},
	)
}

// Error maps err to a safe application error, records its stable code for
// logging, aborts the Gin chain, and writes the uniform error envelope.
func (r Responder) Error(ctx *gin.Context, err error) {
	applicationError := apperror.From(err)
	ctx.Set(errorCodeContextKey, string(applicationError.Code))
	ctx.AbortWithStatusJSON(applicationError.HTTPStatus, errorEnvelope{
		Error: errorBody{
			Code:    applicationError.Code,
			Message: applicationError.Message,
		},
		Meta: r.meta(ctx, MetaOptions{}),
	})
}

// NotFound writes the standard route-not-found error envelope.
func (r Responder) NotFound(ctx *gin.Context) {
	r.Error(ctx, apperror.New(
		apperror.CodeNotFound,
		"The requested resource was not found",
		http.StatusNotFound,
	))
}

// ErrorCode returns the safe application code recorded during error rendering.
func ErrorCode(ctx *gin.Context) string {
	code, _ := ctx.Get(errorCodeContextKey)
	errorCode, _ := code.(string)
	return errorCode
}

func (r Responder) meta(ctx *gin.Context, options MetaOptions) Meta {
	return Meta{
		RequestID:          requestcontext.RequestID(ctx.Request.Context()),
		Timestamp:          r.now().UTC(),
		RateLimitRemaining: options.RateLimitRemaining,
	}
}
