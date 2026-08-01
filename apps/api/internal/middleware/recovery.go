package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

// Recovery converts a downstream panic into the generic application error
// envelope and logs correlation metadata without request credentials.
func Recovery(logger *slog.Logger, responder response.Responder) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(
					"panic recovered",
					"requestId", requestcontext.RequestID(ctx.Request.Context()),
					"method", ctx.Request.Method,
					"path", ctx.FullPath(),
				)
				responder.Error(ctx, apperror.New(
					apperror.CodeInternal,
					"An unexpected error occurred",
					http.StatusInternalServerError,
				))
			}
		}()

		ctx.Next()
	}
}
