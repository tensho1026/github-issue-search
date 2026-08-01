package handler

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authcrypto"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

const maximumOAuthCodeBytes = 4096

var errInvalidAuthHandlerConfiguration = errors.New(
	"invalid authentication handler configuration",
)

type flowCodec interface {
	Seal(authcrypto.FlowPayload) (string, error)
	Open(string, time.Time) (authcrypto.FlowPayload, error)
}

// AuthHandler exposes optional GitHub sign-in and rotating browser sessions.
// Public application routes remain independent of this handler.
type AuthHandler struct {
	authentication usecase.Authentication
	codec          flowCodec
	cookies        authhttp.Policy
	frontendURL    *url.URL
	configured     bool
	responder      response.Responder
	now            func() time.Time
}

// AuthSessionResponse is the public current-session contract.
type AuthSessionResponse struct {
	Configured    bool              `json:"configured"`
	Authenticated bool              `json:"authenticated"`
	User          *AuthUserResponse `json:"user,omitempty"`
	ExpiresAt     *time.Time        `json:"expiresAt,omitempty"`
	CSRFToken     string            `json:"csrfToken,omitempty"`
}

// AuthUserResponse contains only the public GitHub identity retained by
// IssueScout.
type AuthUserResponse struct {
	AccountID  string `json:"accountId"`
	Login      string `json:"login"`
	AvatarURL  string `json:"avatarUrl"`
	ProfileURL string `json:"profileUrl"`
}

// NewAuthHandler constructs either a configured handler or an explicit
// disabled handler. Disabled authentication never requires a database.
func NewAuthHandler(
	configured bool,
	authentication usecase.Authentication,
	codec flowCodec,
	cookies authhttp.Policy,
	frontendURL *url.URL,
	responder response.Responder,
) (*AuthHandler, error) {
	if configured &&
		(authentication == nil || codec == nil || !validFrontendURL(frontendURL)) {
		return nil, errInvalidAuthHandlerConfiguration
	}
	var frontendCopy *url.URL
	if frontendURL != nil {
		copy := *frontendURL
		frontendCopy = &copy
	}
	return &AuthHandler{
		authentication: authentication,
		codec:          codec,
		cookies:        cookies,
		frontendURL:    frontendCopy,
		configured:     configured,
		responder:      responder,
		now:            time.Now,
	}, nil
}

// Session returns anonymous state without touching persistence when cookies
// are absent or malformed.
func (handler *AuthHandler) Session(ctx *gin.Context) {
	if !handler.configured {
		handler.writeAnonymousSession(ctx, false)
		return
	}
	sessionToken, csrfToken, ok := handler.cookies.Credentials(ctx.Request)
	if !ok {
		handler.writeAnonymousSession(ctx, true)
		return
	}
	principal, err := handler.authentication.Authenticate(
		ctx.Request.Context(),
		sessionToken,
		csrfToken,
	)
	if err != nil {
		applicationError := apperror.From(err)
		if applicationError.Code == apperror.CodeAuthentication ||
			applicationError.Code == apperror.CodeCSRFRejected {
			handler.cookies.ClearSession(ctx.Writer)
			handler.writeAnonymousSession(ctx, true)
			return
		}
		handler.responder.Error(ctx, err)
		return
	}
	expiresAt := principal.Session.ExpiresAt.UTC()
	handler.responder.Data(ctx, http.StatusOK, AuthSessionResponse{
		Configured:    true,
		Authenticated: true,
		User: &AuthUserResponse{
			AccountID:  principal.Session.AccountID.String(),
			Login:      principal.Session.Identity.Login.String(),
			AvatarURL:  principal.Session.Identity.AvatarURL,
			ProfileURL: principal.Session.Identity.ProfileURL,
		},
		ExpiresAt: &expiresAt,
		CSRFToken: principal.CSRFToken.Value(),
	})
}

// Start creates a single-use state record, seals PKCE material in an HttpOnly
// cookie, and redirects to GitHub.
func (handler *AuthHandler) Start(ctx *gin.Context) {
	if !handler.requireConfigured(ctx) {
		return
	}
	output, err := handler.authentication.Start(
		ctx.Request.Context(),
		ctx.Query("returnTo"),
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	sealedFlow, err := handler.codec.Seal(authcrypto.FlowPayload{
		State:      output.State,
		Verifier:   output.CodeVerifier,
		ReturnPath: output.ReturnPath,
		ExpiresAt:  output.ExpiresAt,
	})
	if err != nil {
		handler.responder.Error(ctx, apperror.Wrap(
			apperror.CodeInternal,
			"An unexpected error occurred",
			http.StatusInternalServerError,
			err,
		))
		return
	}
	handler.cookies.WriteFlow(
		ctx.Writer,
		sealedFlow,
		output.ExpiresAt,
		handler.now().UTC(),
	)
	ctx.Redirect(http.StatusFound, output.AuthorizationURL)
}

// Callback validates the encrypted browser binding before consuming state and
// exchanging GitHub's one-time authorization code.
func (handler *AuthHandler) Callback(ctx *gin.Context) {
	if !handler.requireConfigured(ctx) {
		return
	}
	handler.cookies.ClearFlow(ctx.Writer)
	flow, err := handler.openCallbackFlow(ctx)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	if upstreamError := ctx.Query("error"); upstreamError != "" {
		returnPath, denialErr := handler.authentication.Deny(
			ctx.Request.Context(),
			flow.State,
			flow.ReturnPath,
		)
		if denialErr != nil {
			handler.responder.Error(ctx, denialErr)
			return
		}
		status := "error"
		if upstreamError == "access_denied" {
			status = "denied"
		}
		ctx.Redirect(
			http.StatusFound,
			handler.frontendRedirect(returnPath, status),
		)
		return
	}
	code := ctx.Query("code")
	if code == "" || len(code) > maximumOAuthCodeBytes {
		handler.responder.Error(ctx, invalidCallbackState())
		return
	}
	output, err := handler.authentication.Complete(
		ctx.Request.Context(),
		usecase.CompleteOAuthInput{
			State:          flow.State,
			CodeVerifier:   flow.Verifier,
			Code:           auth.NewSecret(code),
			FlowReturnPath: flow.ReturnPath,
		},
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.cookies.WriteSession(
		ctx.Writer,
		output.SessionToken,
		output.CSRFToken,
		output.Session.ExpiresAt,
		handler.now().UTC(),
	)
	ctx.Redirect(
		http.StatusFound,
		handler.frontendRedirect(flow.ReturnPath, "success"),
	)
}

// Refresh rotates both opaque browser credentials and invalidates the prior
// session token atomically.
func (handler *AuthHandler) Refresh(ctx *gin.Context) {
	principal, ok := requestcontext.Principal(ctx.Request.Context())
	if !ok {
		handler.responder.Error(ctx, apperror.New(
			apperror.CodeAuthentication,
			"Authentication is required",
			http.StatusUnauthorized,
		))
		return
	}
	output, err := handler.authentication.Refresh(
		ctx.Request.Context(),
		principal,
	)
	if err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.cookies.WriteSession(
		ctx.Writer,
		output.SessionToken,
		output.CSRFToken,
		output.Session.ExpiresAt,
		handler.now().UTC(),
	)
	expiresAt := output.Session.ExpiresAt.UTC()
	handler.responder.Data(ctx, http.StatusOK, AuthSessionResponse{
		Configured:    true,
		Authenticated: true,
		User: &AuthUserResponse{
			AccountID:  output.Session.AccountID.String(),
			Login:      output.Session.Identity.Login.String(),
			AvatarURL:  output.Session.Identity.AvatarURL,
			ProfileURL: output.Session.Identity.ProfileURL,
		},
		ExpiresAt: &expiresAt,
		CSRFToken: output.CSRFToken.Value(),
	})
}

// Logout revokes the current server session before clearing both browser
// credentials.
func (handler *AuthHandler) Logout(ctx *gin.Context) {
	principal, ok := requestcontext.Principal(ctx.Request.Context())
	if !ok {
		handler.responder.Error(ctx, apperror.New(
			apperror.CodeAuthentication,
			"Authentication is required",
			http.StatusUnauthorized,
		))
		return
	}
	if err := handler.authentication.Logout(
		ctx.Request.Context(),
		principal,
	); err != nil {
		handler.responder.Error(ctx, err)
		return
	}
	handler.cookies.ClearSession(ctx.Writer)
	ctx.Status(http.StatusNoContent)
}

func (handler *AuthHandler) openCallbackFlow(
	ctx *gin.Context,
) (authcrypto.FlowPayload, error) {
	sealedFlow, ok := handler.cookies.Flow(ctx.Request)
	if !ok {
		return authcrypto.FlowPayload{}, invalidCallbackState()
	}
	flow, err := handler.codec.Open(sealedFlow, handler.now().UTC())
	if err != nil {
		return authcrypto.FlowPayload{}, invalidCallbackState()
	}
	queryState := ctx.Query("state")
	if !auth.IsOpaqueCredential(queryState) ||
		subtle.ConstantTimeCompare(
			[]byte(queryState),
			[]byte(flow.State.Value()),
		) != 1 {
		return authcrypto.FlowPayload{}, invalidCallbackState()
	}
	return flow, nil
}

func (handler *AuthHandler) frontendRedirect(
	returnPath string,
	status string,
) string {
	relative, err := url.Parse(returnPath)
	if err != nil {
		return handler.frontendURL.String()
	}
	target := *handler.frontendURL
	target.Path = relative.Path
	target.RawPath = relative.RawPath
	target.RawQuery = relative.RawQuery
	query := target.Query()
	query.Set("auth", status)
	target.RawQuery = query.Encode()
	return target.String()
}

func (handler *AuthHandler) requireConfigured(ctx *gin.Context) bool {
	if handler.configured {
		return true
	}
	handler.responder.Error(ctx, apperror.New(
		apperror.CodeAuthUnavailable,
		"GitHub sign-in is not configured",
		http.StatusServiceUnavailable,
	))
	return false
}

func (handler *AuthHandler) writeAnonymousSession(
	ctx *gin.Context,
	configured bool,
) {
	handler.responder.Data(ctx, http.StatusOK, AuthSessionResponse{
		Configured:    configured,
		Authenticated: false,
	})
}

func validFrontendURL(frontendURL *url.URL) bool {
	return frontendURL != nil &&
		(frontendURL.Scheme == "https" || frontendURL.Scheme == "http") &&
		frontendURL.Host != "" &&
		frontendURL.User == nil &&
		(frontendURL.Path == "" || frontendURL.Path == "/") &&
		frontendURL.RawQuery == "" &&
		frontendURL.Fragment == ""
}

func invalidCallbackState() error {
	return apperror.Wrap(
		apperror.CodeInvalidAuthState,
		"Authorization state is invalid or expired",
		http.StatusBadRequest,
		fmt.Errorf("%w", auth.ErrAuthorizationStateNotFound),
	)
}
