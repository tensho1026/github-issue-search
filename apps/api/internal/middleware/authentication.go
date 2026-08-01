package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

const csrfHeader = "X-CSRF-Token"

// RequireAuthenticatedCSRF validates both HttpOnly browser credentials and a
// double-submit header before an authenticated mutation reaches its handler.
// It is intentionally registered only on account/session mutation routes.
func RequireAuthenticatedCSRF(
	authentication usecase.Authentication,
	cookies authhttp.Policy,
	responder response.Responder,
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if authentication == nil {
			responder.Error(ctx, apperror.New(
				apperror.CodeAuthUnavailable,
				"GitHub sign-in is not configured",
				http.StatusServiceUnavailable,
			))
			return
		}
		sessionToken, csrfToken, ok := cookies.Credentials(ctx.Request)
		if !ok {
			responder.Error(ctx, apperror.New(
				apperror.CodeAuthentication,
				"Authentication is required",
				http.StatusUnauthorized,
			))
			return
		}
		principal, err := authentication.Authenticate(
			ctx.Request.Context(),
			sessionToken,
			csrfToken,
		)
		if err != nil {
			responder.Error(ctx, err)
			return
		}
		if err := authentication.ValidateCSRF(
			principal,
			auth.NewSecret(ctx.GetHeader(csrfHeader)),
		); err != nil {
			responder.Error(ctx, err)
			return
		}
		ctx.Request = ctx.Request.WithContext(
			requestcontext.WithPrincipal(
				ctx.Request.Context(),
				principal,
			),
		)
		ctx.Next()
	}
}
