package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestRequestIDPreservesValidAndReplacesInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		incoming string
		want     string
	}{
		{name: "valid", incoming: "client-request_123", want: "client-request_123"},
		{name: "invalid", incoming: "contains spaces", want: "req_generated"},
		{name: "missing", want: "req_generated"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(RequestIDWithGenerator(func() string { return "req_generated" }))
			engine.GET("/", func(ctx *gin.Context) {
				ctx.String(
					http.StatusOK,
					requestcontext.RequestID(ctx.Request.Context()),
				)
			})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set(requestIDHeader, test.incoming)
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, request)

			if got := recorder.Header().Get(requestIDHeader); got != test.want {
				t.Fatalf("response request ID = %q, want %q", got, test.want)
			}
			if got := recorder.Body.String(); got != test.want {
				t.Fatalf("context request ID = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCORSAllowsConfiguredOriginAndRejectsOtherOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responder := response.NewResponder()
	engine := gin.New()
	engine.Use(
		RequestIDWithGenerator(func() string { return "req_cors" }),
		CORS([]string{"https://issuescout.example"}, responder),
	)
	engine.GET("/", func(ctx *gin.Context) { responder.Data(ctx, http.StatusOK, "ok") })

	allowed := httptest.NewRequest(http.MethodGet, "/", nil)
	allowed.Header.Set("Origin", "https://issuescout.example")
	allowedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(allowedRecorder, allowed)
	if got := allowedRecorder.Header().Get("Access-Control-Allow-Origin"); got != allowed.Header.Get("Origin") {
		t.Fatalf("allow origin = %q", got)
	}
	if got := allowedRecorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials = %q, want true", got)
	}

	denied := httptest.NewRequest(http.MethodGet, "/", nil)
	denied.Header.Set("Origin", "https://attacker.example")
	deniedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(deniedRecorder, denied)
	if deniedRecorder.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d", deniedRecorder.Code)
	}
	if !strings.Contains(deniedRecorder.Body.String(), "FORBIDDEN_ORIGIN") ||
		!strings.Contains(deniedRecorder.Body.String(), "req_cors") {
		t.Fatalf("denied response = %s", deniedRecorder.Body.String())
	}
}

func TestAuthenticatedCSRFMiddlewareRejectsMissingCookiesWithoutServiceCall(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)
	service := &middlewareAuthenticationStub{}
	engine := gin.New()
	engine.Use(RequireAuthenticatedCSRF(
		service,
		authhttp.NewPolicy(false),
		response.NewResponder(),
	))
	engine.POST("/", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/", nil),
	)

	if recorder.Code != http.StatusUnauthorized ||
		service.authenticateCalls != 0 {
		t.Fatalf(
			"response = %d calls = %d",
			recorder.Code,
			service.authenticateCalls,
		)
	}
}

func TestAuthenticatedReadMiddlewareAttachesPrincipalWithoutCSRFHeader(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)
	csrf := middlewareCredential(2)
	principal := auth.Principal{CSRFToken: csrf}
	service := &middlewareAuthenticationStub{principal: principal}
	policy := authhttp.NewPolicy(false)
	engine := gin.New()
	engine.Use(RequireAuthenticated(
		service,
		policy,
		response.NewResponder(),
	))
	engine.GET("/", func(ctx *gin.Context) {
		got, ok := requestcontext.Principal(ctx.Request.Context())
		if !ok || got.CSRFToken.Value() != csrf.Value() {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(middlewareRequestCookie(
		policy.Names().Session,
		middlewareCredential(1).Value(),
	))
	request.AddCookie(middlewareRequestCookie(
		policy.Names().CSRF,
		csrf.Value(),
	))
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent ||
		service.authenticateCalls != 1 ||
		service.validatedHeader.Value() != "" {
		t.Fatalf(
			"response = %d authenticate calls = %d header = %s",
			recorder.Code,
			service.authenticateCalls,
			service.validatedHeader,
		)
	}
}

func TestAuthenticatedCSRFMiddlewareAttachesValidatedPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := middlewareCredential(2)
	principal := auth.Principal{CSRFToken: csrf}
	service := &middlewareAuthenticationStub{principal: principal}
	policy := authhttp.NewPolicy(false)
	engine := gin.New()
	engine.Use(RequireAuthenticatedCSRF(
		service,
		policy,
		response.NewResponder(),
	))
	engine.POST("/", func(ctx *gin.Context) {
		got, ok := requestcontext.Principal(ctx.Request.Context())
		if !ok || got.CSRFToken.Value() != csrf.Value() {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(middlewareRequestCookie(
		policy.Names().Session,
		middlewareCredential(1).Value(),
	))
	request.AddCookie(middlewareRequestCookie(
		policy.Names().CSRF,
		csrf.Value(),
	))
	request.Header.Set(csrfHeader, csrf.Value())
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent ||
		service.authenticateCalls != 1 ||
		service.validatedHeader.Value() != csrf.Value() {
		t.Fatalf(
			"response = %d authenticate calls = %d header = %s",
			recorder.Code,
			service.authenticateCalls,
			service.validatedHeader,
		)
	}
}

func TestAuthenticatedCSRFMiddlewareStopsRejectedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := middlewareCredential(2)
	service := &middlewareAuthenticationStub{
		principal: auth.Principal{CSRFToken: csrf},
		validateErr: apperror.New(
			apperror.CodeCSRFRejected,
			"CSRF validation failed",
			http.StatusForbidden,
		),
	}
	policy := authhttp.NewPolicy(false)
	engine := gin.New()
	engine.Use(RequireAuthenticatedCSRF(
		service,
		policy,
		response.NewResponder(),
	))
	engine.POST("/", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(middlewareRequestCookie(
		policy.Names().Session,
		middlewareCredential(1).Value(),
	))
	request.AddCookie(middlewareRequestCookie(
		policy.Names().CSRF,
		csrf.Value(),
	))
	request.Header.Set(csrfHeader, middlewareCredential(9).Value())
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden ||
		!strings.Contains(recorder.Body.String(), "CSRF_REJECTED") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestSecurityHeadersAreApplied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SecurityHeaders())
	engine.GET("/", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Cache-Control":          "no-store",
	}
	for name, want := range headers {
		if got := recorder.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestRecoveryReturnsSafeEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	responder := response.NewResponder()
	engine := gin.New()
	engine.Use(
		RequestIDWithGenerator(func() string { return "req_panic" }),
		Recovery(logger, responder),
	)
	engine.GET("/", func(*gin.Context) { panic("private panic detail") })
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "private panic detail") ||
		strings.Contains(logs.String(), "private panic detail") {
		t.Fatalf("panic detail was exposed")
	}
	if !strings.Contains(recorder.Body.String(), "req_panic") {
		t.Fatalf("response is missing request ID: %s", recorder.Body.String())
	}
}

func TestTimeoutCancelsRequestAndReturnsGatewayTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responder := response.NewResponder()
	engine := gin.New()
	engine.Use(
		RequestIDWithGenerator(func() string { return "req_timeout" }),
		Timeout(time.Millisecond, responder),
	)
	engine.GET("/", func(ctx *gin.Context) {
		<-ctx.Request.Context().Done()
	})
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusGatewayTimeout)
	}
	if !strings.Contains(recorder.Body.String(), "REQUEST_TIMEOUT") {
		t.Fatalf("response = %s", recorder.Body.String())
	}
}

func TestRequestLoggerUsesStructuredSafeFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies() error = %v", err)
	}
	engine.Use(
		RequestIDWithGenerator(func() string { return "req_log" }),
		RequestLogger(logger),
	)
	engine.NoRoute(func(ctx *gin.Context) { ctx.Status(http.StatusNotFound) })
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	request.Header.Set("Authorization", "Bearer must-not-appear")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	var entry map[string]any
	if err := json.NewDecoder(bytes.NewReader(logs.Bytes())).Decode(&entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if entry["requestId"] != "req_log" ||
		entry["status"] != float64(http.StatusNotFound) ||
		entry["path"] != "/missing" {
		t.Fatalf("log entry = %+v", entry)
	}
	if strings.Contains(logs.String(), "must-not-appear") {
		t.Fatalf("authorization value was logged")
	}
}

type middlewareAuthenticationStub struct {
	principal         auth.Principal
	authenticateErr   error
	authenticateCalls int
	validateErr       error
	validatedHeader   auth.Secret
}

func (stub *middlewareAuthenticationStub) Start(
	context.Context,
	string,
) (usecase.OAuthStartOutput, error) {
	return usecase.OAuthStartOutput{}, nil
}

func (stub *middlewareAuthenticationStub) Complete(
	context.Context,
	usecase.CompleteOAuthInput,
) (usecase.AuthSessionOutput, error) {
	return usecase.AuthSessionOutput{}, nil
}

func (stub *middlewareAuthenticationStub) Deny(
	context.Context,
	auth.Secret,
	string,
) (string, error) {
	return "", nil
}

func (stub *middlewareAuthenticationStub) Authenticate(
	context.Context,
	auth.Secret,
	auth.Secret,
) (auth.Principal, error) {
	stub.authenticateCalls++
	return stub.principal, stub.authenticateErr
}

func (stub *middlewareAuthenticationStub) ValidateCSRF(
	_ auth.Principal,
	header auth.Secret,
) error {
	stub.validatedHeader = header
	return stub.validateErr
}

func (stub *middlewareAuthenticationStub) Refresh(
	context.Context,
	auth.Principal,
) (usecase.AuthSessionOutput, error) {
	return usecase.AuthSessionOutput{}, nil
}

func (stub *middlewareAuthenticationStub) Logout(
	context.Context,
	auth.Principal,
) error {
	return nil
}

func middlewareCredential(fill byte) auth.Secret {
	return auth.NewSecret(base64.RawURLEncoding.EncodeToString(
		bytes.Repeat([]byte{fill}, 32),
	))
}

func middlewareRequestCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}
