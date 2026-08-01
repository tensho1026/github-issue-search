package apidocs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/swaggest/swgui/v5emb"
)

func TestNewRejectsMissingAssets(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil")
	}
}

func TestHandlerServesSelfContainedDocuments(t *testing.T) {
	handler := newTestHandler(t)
	tests := []struct {
		path        string
		contentType string
		contains    []string
	}{
		{
			path:        IndexPath,
			contentType: "text/html; charset=utf-8",
			contains: []string{
				"IssueScout API reference",
				`href="/openapi.yaml"`,
				`src="/docs/swagger-ui-bundle.js"`,
				`src="/docs/issuescout-swagger.js"`,
			},
		},
		{
			path:        javascriptPath,
			contentType: "text/javascript; charset=utf-8",
			contains: []string{
				`url: "/openapi.yaml"`,
				"tryItOutEnabled: true",
				"persistAuthorization: false",
				`request.headers["X-Request-ID"]`,
			},
		},
		{
			path:        stylesheetPath,
			contentType: "text/css; charset=utf-8",
			contains: []string{
				".skip-link:focus",
				"@media (max-width: 760px)",
				"prefers-reduced-motion",
			},
		},
		{
			path:        OpenAPIPath,
			contentType: "application/yaml; charset=utf-8",
			contains: []string{
				"openapi: 3.1.0",
				"title: IssueScout API",
				"/api/issues/search:",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d", recorder.Code)
			}
			if got := recorder.Header().Get("Content-Type"); got != test.contentType {
				t.Fatalf("Content-Type = %q", got)
			}
			for _, fragment := range test.contains {
				if !strings.Contains(recorder.Body.String(), fragment) {
					t.Errorf("body does not contain %q", fragment)
				}
			}
			if recorder.Header().Get("ETag") == "" {
				t.Error("ETag is empty")
			}
			if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(
				got,
				"script-src 'self'",
			) || strings.Contains(got, "script-src 'self' 'unsafe-inline'") {
				t.Errorf("Content-Security-Policy = %q", got)
			}
		})
	}
}

func TestHandlerSupportsConditionalAndHeadRequests(t *testing.T) {
	handler := newTestHandler(t)
	first := httptest.NewRecorder()
	handler.ServeHTTP(
		first,
		httptest.NewRequest(http.MethodGet, OpenAPIPath, nil),
	)

	conditional := httptest.NewRequest(http.MethodGet, OpenAPIPath, nil)
	conditional.Header.Set("If-None-Match", first.Header().Get("ETag"))
	conditionalRecorder := httptest.NewRecorder()
	handler.ServeHTTP(conditionalRecorder, conditional)
	if conditionalRecorder.Code != http.StatusNotModified ||
		conditionalRecorder.Body.Len() != 0 {
		t.Fatalf(
			"conditional response = %d %q",
			conditionalRecorder.Code,
			conditionalRecorder.Body.String(),
		)
	}

	for _, condition := range []string{
		`W/` + first.Header().Get("ETag"),
		"*",
	} {
		request := httptest.NewRequest(http.MethodGet, OpenAPIPath, nil)
		request.Header.Set("If-None-Match", condition)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotModified ||
			recorder.Body.Len() != 0 {
			t.Errorf(
				"If-None-Match %q response = %d %q",
				condition,
				recorder.Code,
				recorder.Body.String(),
			)
		}
	}

	headRecorder := httptest.NewRecorder()
	handler.ServeHTTP(
		headRecorder,
		httptest.NewRequest(http.MethodHead, OpenAPIPath, nil),
	)
	if headRecorder.Code != http.StatusOK ||
		headRecorder.Body.Len() != 0 ||
		headRecorder.Header().Get("Content-Length") == "" {
		t.Fatalf(
			"HEAD response = %d length=%q body=%q",
			headRecorder.Code,
			headRecorder.Header().Get("Content-Length"),
			headRecorder.Body.String(),
		)
	}
}

func TestHandlerRedirectsCanonicalIndexAndRejectsUnknownRequests(t *testing.T) {
	var assetCalls atomic.Int32
	handler, err := New(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		assetCalls.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	redirectRecorder := httptest.NewRecorder()
	handler.ServeHTTP(
		redirectRecorder,
		httptest.NewRequest(http.MethodGet, RootPath, nil),
	)
	if redirectRecorder.Code != http.StatusPermanentRedirect ||
		redirectRecorder.Header().Get("Location") != IndexPath {
		t.Fatalf(
			"redirect = %d %q",
			redirectRecorder.Code,
			redirectRecorder.Header().Get("Location"),
		)
	}

	for _, path := range []string{"/docs/unknown.js", "/other"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, path, nil),
		)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s status = %d", path, recorder.Code)
		}
	}
	if assetCalls.Load() != 0 {
		t.Fatalf("unknown requests reached assets %d time(s)", assetCalls.Load())
	}

	methodRecorder := httptest.NewRecorder()
	handler.ServeHTTP(
		methodRecorder,
		httptest.NewRequest(http.MethodPost, IndexPath, nil),
	)
	if methodRecorder.Code != http.StatusMethodNotAllowed ||
		methodRecorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf(
			"method response = %d allow=%q",
			methodRecorder.Code,
			methodRecorder.Header().Get("Allow"),
		)
	}
}

func TestEmbeddedSwaggerAssetsAreAvailableWithoutNetwork(t *testing.T) {
	handler := newTestHandler(t)
	for _, path := range []string{
		"/docs/swagger-ui.css",
		"/docs/swagger-ui-bundle.js",
		"/docs/favicon-32x32.png",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Accept-Encoding", "identity")
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK || recorder.Body.Len() == 0 {
			t.Errorf(
				"%s response = %d, %d bytes",
				path,
				recorder.Code,
				recorder.Body.Len(),
			)
		}
		if got := recorder.Header().Get("Cache-Control"); got != staticMaxAge {
			t.Errorf("%s Cache-Control = %q", path, got)
		}
	}
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	assets := v5emb.New(
		"IssueScout API",
		OpenAPIPath,
		IndexPath,
	)
	handler, err := New(assets)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return handler
}
