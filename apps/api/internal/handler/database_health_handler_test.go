package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

func TestDatabaseHealthHandlerReportsReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	health := &databaseHealthStub{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/health/database",
		nil,
	)

	NewDatabaseHealthHandler(
		health,
		true,
		response.NewResponder(),
	).Check(ctx)

	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"status":"ready"`) ||
		!strings.Contains(recorder.Body.String(), `"configured":true`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if health.calls.Load() != 1 {
		t.Fatalf("Ping() calls = %d", health.calls.Load())
	}
}

func TestDatabaseHealthHandlerFailsClosedWithoutExposingCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		configured bool
		health     *databaseHealthStub
	}{
		{
			name:       "not configured",
			configured: false,
			health:     &databaseHealthStub{},
		},
		{
			name:       "probe failed",
			configured: true,
			health: &databaseHealthStub{
				err: errors.New("sensitive-host.example: connection refused"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(
				http.MethodGet,
				"/api/health/database",
				nil,
			)

			NewDatabaseHealthHandler(
				test.health,
				test.configured,
				response.NewResponder(),
			).Check(ctx)

			if recorder.Code != http.StatusServiceUnavailable ||
				!strings.Contains(
					recorder.Body.String(),
					`"code":"DATABASE_UNAVAILABLE"`,
				) {
				t.Fatalf(
					"response = %d %s",
					recorder.Code,
					recorder.Body.String(),
				)
			}
			if strings.Contains(recorder.Body.String(), "sensitive-host") {
				t.Fatal("response exposed a database driver detail")
			}
			expectedCalls := int64(0)
			if test.configured {
				expectedCalls = 1
			}
			if test.health.calls.Load() != expectedCalls {
				t.Fatalf("Ping() calls = %d", test.health.calls.Load())
			}
		})
	}
}

type databaseHealthStub struct {
	calls atomic.Int64
	err   error
}

func (health *databaseHealthStub) Ping(context.Context) error {
	health.calls.Add(1)
	return health.err
}
