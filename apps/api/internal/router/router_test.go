package router

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

func TestHealthRouteUsesStandardEnvelopeAndHeaders(t *testing.T) {
	router := newTestRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("X-Request-ID", "req_health")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("X-Request-ID"); got != "req_health" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := recorder.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	var body struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Meta response.Meta `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Status != "ok" || body.Meta.RequestID != "req_health" {
		t.Fatalf("body = %+v", body)
	}
}

func TestUnknownRouteUsesSafeErrorEnvelope(t *testing.T) {
	router := newTestRouter(t)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/unknown", nil),
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestNewRequiresLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig(t)

	_, err := New(Dependencies{
		Config:    cfg,
		Responder: response.NewResponder(),
	})

	if err == nil {
		t.Fatalf("New() error = nil")
	}
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router, err := New(Dependencies{
		Config:    testConfig(t),
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		Responder: response.NewResponder(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return router
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("ALLOWED_ORIGINS", "https://issuescout.example")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}
