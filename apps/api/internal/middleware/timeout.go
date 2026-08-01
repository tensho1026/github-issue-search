package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

// Timeout derives a request context deadline for downstream I/O and writes a
// gateway-timeout envelope only when the deadline expires before any response.
// Downstream clients and repositories remain responsible for honoring context.
func Timeout(duration time.Duration, responder response.Responder) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestContext, cancel := context.WithTimeout(ctx.Request.Context(), duration)
		defer cancel()
		ctx.Request = ctx.Request.WithContext(requestContext)

		ctx.Next()

		if requestContext.Err() == context.DeadlineExceeded && !ctx.Writer.Written() {
			responder.Error(ctx, apperror.New(
				apperror.CodeRequestTimeout,
				"The request timed out",
				http.StatusGatewayTimeout,
			))
		}
	}
}
