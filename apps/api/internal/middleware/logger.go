package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

// RequestLogger emits one structured completion event per request with
// request ID, method, route template, status, duration, response size, cache
// state, and safe error code. It never records addresses, user agents, query
// strings, bodies, cookies, authorization, or CSRF data.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		ctx.Next()

		path := ctx.FullPath()
		if path == "" {
			path = ctx.Request.URL.Path
		}
		responseBytes := max(ctx.Writer.Size(), 0)
		attributes := []any{
			"requestId", requestcontext.RequestID(ctx.Request.Context()),
			"method", ctx.Request.Method,
			"path", path,
			"status", ctx.Writer.Status(),
			"latencyMs", time.Since(startedAt).Milliseconds(),
			"responseBytes", responseBytes,
		}
		cacheStatus := ctx.Writer.Header().Get("X-IssueScout-Cache")
		if cacheStatus == "HIT" || cacheStatus == "MISS" {
			attributes = append(attributes, "cacheStatus", cacheStatus)
		}
		if errorCode := response.ErrorCode(ctx); errorCode != "" {
			attributes = append(attributes, "errorCode", errorCode)
		}

		logger.Info("request completed", attributes...)
	}
}
