package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
)

const errorCodeContextKey = "issuescout.error_code"

type Meta struct {
	RequestID string    `json:"requestId"`
	Timestamp time.Time `json:"timestamp"`
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

func NewResponder() Responder {
	return Responder{now: time.Now}
}

func NewResponderWithClock(now func() time.Time) Responder {
	return Responder{now: now}
}

func (r Responder) Data(ctx *gin.Context, status int, data any) {
	ctx.JSON(status, successEnvelope{Data: data, Meta: r.meta(ctx)})
}

func (r Responder) Error(ctx *gin.Context, err error) {
	applicationError := apperror.From(err)
	ctx.Set(errorCodeContextKey, string(applicationError.Code))
	ctx.AbortWithStatusJSON(applicationError.HTTPStatus, errorEnvelope{
		Error: errorBody{
			Code:    applicationError.Code,
			Message: applicationError.Message,
		},
		Meta: r.meta(ctx),
	})
}

func (r Responder) NotFound(ctx *gin.Context) {
	r.Error(ctx, apperror.New(
		apperror.CodeNotFound,
		"The requested resource was not found",
		http.StatusNotFound,
	))
}

func ErrorCode(ctx *gin.Context) string {
	code, _ := ctx.Get(errorCodeContextKey)
	errorCode, _ := code.(string)
	return errorCode
}

func (r Responder) meta(ctx *gin.Context) Meta {
	return Meta{
		RequestID: requestcontext.RequestID(ctx.Request.Context()),
		Timestamp: r.now().UTC(),
	}
}
