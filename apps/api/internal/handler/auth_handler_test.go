package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authcrypto"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/authhttp"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

func TestAuthSessionIsAnonymousWithoutCredentialsOrConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	frontend := mustURL(t, "https://issuescout.example")
	service := &authenticationHandlerStub{}
	configured := newTestAuthHandler(
		t,
		true,
		service,
		&flowCodecStub{},
		frontend,
	)
	recorder := serveAuthHandler(
		t,
		http.MethodGet,
		"/api/auth/session",
		configured.Session,
		nil,
	)
	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"configured":true`) ||
		!strings.Contains(recorder.Body.String(), `"authenticated":false`) {
		t.Fatalf("configured response = %d %s", recorder.Code, recorder.Body.String())
	}
	if service.authenticateCalls != 0 {
		t.Fatalf("Authenticate() calls = %d", service.authenticateCalls)
	}

	disabled := newTestAuthHandler(t, false, nil, nil, nil)
	recorder = serveAuthHandler(
		t,
		http.MethodGet,
		"/api/auth/session",
		disabled.Session,
		nil,
	)
	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"configured":false`) {
		t.Fatalf("disabled response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthSessionReturnsPublicIdentityAndCSRFOnly(t *testing.T) {
	principal := testPrincipal(t)
	service := &authenticationHandlerStub{principal: principal}
	handler := newTestAuthHandler(
		t,
		true,
		service,
		&flowCodecStub{},
		mustURL(t, "https://issuescout.example"),
	)
	policy := authhttp.NewPolicy(false)
	sessionToken := testCredential(1)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/session",
		nil,
	)
	request.AddCookie(authHandlerRequestCookie(
		policy.Names().Session,
		sessionToken.Value(),
	))
	request.AddCookie(authHandlerRequestCookie(
		policy.Names().CSRF,
		principal.CSRFToken.Value(),
	))
	recorder := serveAuthHandler(
		t,
		http.MethodGet,
		"/api/auth/session",
		handler.Session,
		request,
	)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK ||
		!strings.Contains(body, `"authenticated":true`) ||
		!strings.Contains(body, `"login":"octocat"`) ||
		!strings.Contains(body, principal.CSRFToken.Value()) {
		t.Fatalf("response = %d %s", recorder.Code, body)
	}
	if strings.Contains(body, sessionToken.Value()) {
		t.Fatal("session response exposed the session credential")
	}
}

func TestAuthStartSealsFlowAndRedirectsToGitHub(t *testing.T) {
	now := testAuthTime()
	state := testCredential(1)
	verifier := testCredential(2)
	service := &authenticationHandlerStub{
		startOutput: usecase.OAuthStartOutput{
			AuthorizationURL: "https://github.com/login/oauth/authorize?client_id=test",
			State:            state,
			CodeVerifier:     verifier,
			ReturnPath:       "/workspace?tab=saved",
			ExpiresAt:        now.Add(10 * time.Minute),
		},
	}
	codec := &flowCodecStub{sealed: "v1.encrypted-flow"}
	handler := newTestAuthHandler(
		t,
		true,
		service,
		codec,
		mustURL(t, "https://issuescout.example"),
	)
	recorder := serveAuthHandler(
		t,
		http.MethodGet,
		"/api/auth/github/start?returnTo=%2Fworkspace%3Ftab%3Dsaved",
		handler.Start,
		nil,
	)

	if recorder.Code != http.StatusFound ||
		recorder.Header().Get("Location") != service.startOutput.AuthorizationURL {
		t.Fatalf(
			"redirect = %d %q",
			recorder.Code,
			recorder.Header().Get("Location"),
		)
	}
	if service.startReturnPath != "/workspace?tab=saved" {
		t.Fatalf("Start() return path = %q", service.startReturnPath)
	}
	if codec.sealPayload.State.Value() != state.Value() ||
		codec.sealPayload.Verifier.Value() != verifier.Value() {
		t.Fatal("flow payload did not bind state and verifier")
	}
	setCookie := strings.Join(recorder.Header().Values("Set-Cookie"), "\n")
	if !strings.Contains(setCookie, "issuescout_oauth_flow=v1.encrypted-flow") ||
		!strings.Contains(setCookie, "HttpOnly") ||
		!strings.Contains(setCookie, "SameSite=Lax") {
		t.Fatalf("Set-Cookie = %s", setCookie)
	}
}

func TestAuthCallbackRejectsStateMismatchBeforeUsecase(t *testing.T) {
	flow := authcrypto.FlowPayload{
		State:      testCredential(1),
		Verifier:   testCredential(2),
		ReturnPath: "/workspace",
		ExpiresAt:  testAuthTime().Add(10 * time.Minute),
	}
	service := &authenticationHandlerStub{}
	handler := newTestAuthHandler(
		t,
		true,
		service,
		&flowCodecStub{opened: flow},
		mustURL(t, "https://issuescout.example"),
	)
	request := callbackRequest(
		t,
		"/api/auth/github/callback?state="+
			url.QueryEscape(testCredential(9).Value())+
			"&code=one-time-code",
	)
	recorder := serveAuthHandler(
		t,
		http.MethodGet,
		request.URL.String(),
		handler.Callback,
		request,
	)

	if recorder.Code != http.StatusBadRequest ||
		!strings.Contains(recorder.Body.String(), "INVALID_AUTH_STATE") {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if service.completeCalls != 0 || service.denyCalls != 0 {
		t.Fatal("mismatched state reached the authentication service")
	}
	if !strings.Contains(
		strings.Join(recorder.Header().Values("Set-Cookie"), "\n"),
		"issuescout_oauth_flow=;",
	) {
		t.Fatal("callback did not clear the flow cookie")
	}
}

func TestAuthCallbackCreatesSessionAndUsesFixedFrontendOrigin(t *testing.T) {
	flow := authcrypto.FlowPayload{
		State:      testCredential(1),
		Verifier:   testCredential(2),
		ReturnPath: "/workspace?tab=saved",
		ExpiresAt:  testAuthTime().Add(10 * time.Minute),
	}
	session := testPrincipal(t).Session
	service := &authenticationHandlerStub{
		completeOutput: usecase.AuthSessionOutput{
			Session:      session,
			SessionToken: testCredential(3),
			CSRFToken:    testCredential(4),
		},
	}
	handler := newTestAuthHandler(
		t,
		true,
		service,
		&flowCodecStub{opened: flow},
		mustURL(t, "https://issuescout.example"),
	)
	request := callbackRequest(
		t,
		"/api/auth/github/callback?state="+
			url.QueryEscape(flow.State.Value())+
			"&code=one-time-code",
	)
	recorder := serveAuthHandler(
		t,
		http.MethodGet,
		request.URL.String(),
		handler.Callback,
		request,
	)

	if recorder.Code != http.StatusFound ||
		recorder.Header().Get("Location") !=
			"https://issuescout.example/workspace?auth=success&tab=saved" {
		t.Fatalf(
			"redirect = %d %q",
			recorder.Code,
			recorder.Header().Get("Location"),
		)
	}
	if service.completeInput.Code.Value() != "one-time-code" ||
		service.completeInput.CodeVerifier.Value() != flow.Verifier.Value() {
		t.Fatal("callback did not preserve the one-time OAuth binding")
	}
	setCookies := strings.Join(recorder.Header().Values("Set-Cookie"), "\n")
	for _, fragment := range []string{
		"issuescout_oauth_flow=;",
		"issuescout_session=" + service.completeOutput.SessionToken.Value(),
		"issuescout_csrf=" + service.completeOutput.CSRFToken.Value(),
	} {
		if !strings.Contains(setCookies, fragment) {
			t.Errorf("Set-Cookie missing %q: %s", fragment, setCookies)
		}
	}
}

func TestAuthCallbackConsumesDeniedAuthorization(t *testing.T) {
	flow := authcrypto.FlowPayload{
		State:      testCredential(1),
		Verifier:   testCredential(2),
		ReturnPath: "/",
		ExpiresAt:  testAuthTime().Add(10 * time.Minute),
	}
	service := &authenticationHandlerStub{denyReturnPath: "/"}
	handler := newTestAuthHandler(
		t,
		true,
		service,
		&flowCodecStub{opened: flow},
		mustURL(t, "https://issuescout.example"),
	)
	request := callbackRequest(
		t,
		"/api/auth/github/callback?state="+
			url.QueryEscape(flow.State.Value())+
			"&error=access_denied&error_description=must-not-be-reflected",
	)
	recorder := serveAuthHandler(
		t,
		http.MethodGet,
		request.URL.String(),
		handler.Callback,
		request,
	)

	if recorder.Code != http.StatusFound ||
		recorder.Header().Get("Location") !=
			"https://issuescout.example/?auth=denied" {
		t.Fatalf(
			"redirect = %d %q",
			recorder.Code,
			recorder.Header().Get("Location"),
		)
	}
	if service.denyCalls != 1 || service.completeCalls != 0 {
		t.Fatalf(
			"Deny() calls = %d Complete() calls = %d",
			service.denyCalls,
			service.completeCalls,
		)
	}
	if strings.Contains(recorder.Body.String(), "must-not-be-reflected") {
		t.Fatal("callback reflected the provider error description")
	}
}

func TestAuthRefreshAndLogoutUseContextPrincipal(t *testing.T) {
	principal := testPrincipal(t)
	refreshed := usecase.AuthSessionOutput{
		Session:      principal.Session,
		SessionToken: testCredential(5),
		CSRFToken:    testCredential(6),
	}
	service := &authenticationHandlerStub{refreshOutput: refreshed}
	handler := newTestAuthHandler(
		t,
		true,
		service,
		&flowCodecStub{},
		mustURL(t, "https://issuescout.example"),
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/session/refresh",
		nil,
	)
	request = request.WithContext(requestcontext.WithPrincipal(
		request.Context(),
		principal,
	))
	refreshRecorder := serveAuthHandler(
		t,
		http.MethodPost,
		request.URL.String(),
		handler.Refresh,
		request,
	)
	if refreshRecorder.Code != http.StatusOK ||
		!strings.Contains(
			refreshRecorder.Body.String(),
			refreshed.CSRFToken.Value(),
		) {
		t.Fatalf(
			"refresh response = %d %s",
			refreshRecorder.Code,
			refreshRecorder.Body.String(),
		)
	}

	logoutRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
		nil,
	)
	logoutRequest = logoutRequest.WithContext(requestcontext.WithPrincipal(
		logoutRequest.Context(),
		principal,
	))
	logoutRecorder := serveAuthHandler(
		t,
		http.MethodPost,
		logoutRequest.URL.String(),
		handler.Logout,
		logoutRequest,
	)
	if logoutRecorder.Code != http.StatusOK ||
		service.logoutCalls != 1 ||
		!strings.Contains(logoutRecorder.Body.String(), `"loggedOut":true`) {
		t.Fatalf(
			"logout response = %d calls = %d",
			logoutRecorder.Code,
			service.logoutCalls,
		)
	}
	if len(logoutRecorder.Header().Values("Set-Cookie")) != 2 {
		t.Fatal("logout did not clear both session cookies")
	}
}

type authenticationHandlerStub struct {
	startOutput       usecase.OAuthStartOutput
	startErr          error
	startReturnPath   string
	completeOutput    usecase.AuthSessionOutput
	completeInput     usecase.CompleteOAuthInput
	completeErr       error
	completeCalls     int
	denyReturnPath    string
	denyErr           error
	denyCalls         int
	principal         auth.Principal
	authenticateErr   error
	authenticateCalls int
	validateErr       error
	refreshOutput     usecase.AuthSessionOutput
	refreshErr        error
	logoutErr         error
	logoutCalls       int
}

func (stub *authenticationHandlerStub) Start(
	_ context.Context,
	returnPath string,
) (usecase.OAuthStartOutput, error) {
	stub.startReturnPath = returnPath
	return stub.startOutput, stub.startErr
}

func (stub *authenticationHandlerStub) Complete(
	_ context.Context,
	input usecase.CompleteOAuthInput,
) (usecase.AuthSessionOutput, error) {
	stub.completeCalls++
	stub.completeInput = input
	return stub.completeOutput, stub.completeErr
}

func (stub *authenticationHandlerStub) Deny(
	_ context.Context,
	_ auth.Secret,
	_ string,
) (string, error) {
	stub.denyCalls++
	return stub.denyReturnPath, stub.denyErr
}

func (stub *authenticationHandlerStub) Authenticate(
	_ context.Context,
	_ auth.Secret,
	_ auth.Secret,
) (auth.Principal, error) {
	stub.authenticateCalls++
	return stub.principal, stub.authenticateErr
}

func (stub *authenticationHandlerStub) ValidateCSRF(
	_ auth.Principal,
	_ auth.Secret,
) error {
	return stub.validateErr
}

func (stub *authenticationHandlerStub) Refresh(
	_ context.Context,
	_ auth.Principal,
) (usecase.AuthSessionOutput, error) {
	return stub.refreshOutput, stub.refreshErr
}

func (stub *authenticationHandlerStub) Logout(
	_ context.Context,
	_ auth.Principal,
) error {
	stub.logoutCalls++
	return stub.logoutErr
}

type flowCodecStub struct {
	sealed      string
	sealPayload authcrypto.FlowPayload
	sealErr     error
	opened      authcrypto.FlowPayload
	openErr     error
}

func (stub *flowCodecStub) Seal(
	payload authcrypto.FlowPayload,
) (string, error) {
	stub.sealPayload = payload
	return stub.sealed, stub.sealErr
}

func (stub *flowCodecStub) Open(
	_ string,
	_ time.Time,
) (authcrypto.FlowPayload, error) {
	return stub.opened, stub.openErr
}

func newTestAuthHandler(
	t *testing.T,
	configured bool,
	service usecase.Authentication,
	codec flowCodec,
	frontend *url.URL,
) *AuthHandler {
	t.Helper()
	handler, err := NewAuthHandler(
		configured,
		service,
		codec,
		authhttp.NewPolicy(false),
		frontend,
		response.NewResponder(),
	)
	if err != nil {
		t.Fatalf("NewAuthHandler() error = %v", err)
	}
	handler.now = testAuthTime
	return handler
}

func serveAuthHandler(
	t *testing.T,
	method string,
	target string,
	handler gin.HandlerFunc,
	request *http.Request,
) *httptest.ResponseRecorder {
	t.Helper()
	engine := gin.New()
	engine.Handle(method, "/api/auth/*path", handler)
	if request == nil {
		request = httptest.NewRequest(method, target, nil)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func callbackRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.AddCookie(authHandlerRequestCookie(
		authhttp.NewPolicy(false).Names().Flow,
		"v1.encrypted-flow",
	))
	return request
}

func authHandlerRequestCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func testPrincipal(t *testing.T) auth.Principal {
	t.Helper()
	accountID, err := account.ParseID("6ca6dfc4-0114-44fb-a9f8-d703f8c9a8b2")
	if err != nil {
		t.Fatal(err)
	}
	login, err := user.ParseUsername("octocat")
	if err != nil {
		t.Fatal(err)
	}
	csrf := testCredential(4)
	return auth.Principal{
		Session: auth.Session{
			ID:        "d9449f89-c7aa-48ed-8c61-57d0be9725d6",
			AccountID: accountID,
			TokenHash: auth.Hash(testCredential(3).Value()),
			CSRFHash:  auth.Hash(csrf.Value()),
			ExpiresAt: testAuthTime().Add(time.Hour),
			Identity: auth.GitHubIdentity{
				UserID:     583231,
				Login:      login,
				AvatarURL:  "https://avatars.githubusercontent.com/u/583231",
				ProfileURL: "https://github.com/octocat",
			},
		},
		CSRFToken: csrf,
	}
}

func testCredential(fill byte) auth.Secret {
	return auth.NewSecret(base64.RawURLEncoding.EncodeToString(
		bytes.Repeat([]byte{fill}, 32),
	))
}

func testAuthTime() time.Time {
	return time.Date(2026, time.August, 1, 5, 0, 0, 0, time.UTC)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
