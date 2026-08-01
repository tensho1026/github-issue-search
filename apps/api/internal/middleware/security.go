package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders applies non-sniffing, frame denial, strict referrer, and
// no-store defaults before any downstream handler writes a response.
func SecurityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		ctx.Header("Cache-Control", "no-store")
		ctx.Next()
	}
}
