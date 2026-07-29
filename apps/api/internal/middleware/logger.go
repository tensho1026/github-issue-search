package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		ctx.Next()

		attributes := []any{
			"requestId", requestcontext.RequestID(ctx.Request.Context()),
			"method", ctx.Request.Method,
			"path", ctx.FullPath(),
			"status", ctx.Writer.Status(),
			"latencyMs", time.Since(startedAt).Milliseconds(),
			"userAgent", ctx.Request.UserAgent(),
			"clientIP", ctx.ClientIP(),
		}
		if errorCode := response.ErrorCode(ctx); errorCode != "" {
			attributes = append(attributes, "errorCode", errorCode)
		}

		logger.Info("request completed", attributes...)
	}
}
