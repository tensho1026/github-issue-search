package response_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

func ExampleResponder_DataWithMeta() {
	gin.SetMode(gin.TestMode)
	responder := response.NewResponderWithClock(func() time.Time {
		return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	})
	remaining := 7
	engine := gin.New()
	engine.GET("/", func(ctx *gin.Context) {
		responder.DataWithMeta(
			ctx,
			http.StatusOK,
			gin.H{"status": "ok"},
			response.MetaOptions{RateLimitRemaining: &remaining},
		)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(requestcontext.WithRequestID(
		request.Context(),
		"req_example",
	))
	engine.ServeHTTP(recorder, request)
	fmt.Print(recorder.Body.String())

	// Output:
	// {"data":{"status":"ok"},"meta":{"requestId":"req_example","timestamp":"2026-08-01T00:00:00Z","rateLimitRemaining":7}}
}

func ExampleResponder_Error() {
	gin.SetMode(gin.TestMode)
	responder := response.NewResponderWithClock(func() time.Time {
		return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	})
	engine := gin.New()
	engine.GET("/", func(ctx *gin.Context) {
		responder.Error(ctx, apperror.Wrap(
			apperror.CodeDatabaseUnavailable,
			"Account storage is temporarily unavailable",
			http.StatusServiceUnavailable,
			errors.New("internal connection failure"),
		))
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(requestcontext.WithRequestID(
		request.Context(),
		"req_example",
	))
	engine.ServeHTTP(recorder, request)
	fmt.Print(recorder.Body.String())

	// Output:
	// {"error":{"code":"DATABASE_UNAVAILABLE","message":"Account storage is temporarily unavailable"},"meta":{"requestId":"req_example","timestamp":"2026-08-01T00:00:00Z"}}
}
