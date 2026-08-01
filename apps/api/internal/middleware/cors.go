package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

// CORS allows only exact configured browser origins, emits credential-aware
// preflight headers, and rejects disallowed origins without invoking handlers.
// The origin slice is copied into an immutable lookup during construction.
func CORS(allowedOrigins []string, responder response.Responder) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin == "" {
			ctx.Next()
			return
		}
		if _, ok := allowed[origin]; !ok {
			responder.Error(ctx, apperror.New(
				apperror.CodeForbiddenOrigin,
				"Origin is not allowed",
				http.StatusForbidden,
			))
			return
		}

		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header(
			"Access-Control-Allow-Headers",
			"Accept, Authorization, Content-Type, X-CSRF-Token, X-Request-ID",
		)
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		ctx.Header("Access-Control-Expose-Headers", "X-Request-ID")
		ctx.Header("Vary", "Origin")

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
