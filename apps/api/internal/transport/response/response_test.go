package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/requestcontext"
)

func TestResponderWritesSuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 30, 1, 2, 3, 0, time.UTC)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request = request.WithContext(
		requestcontext.WithRequestID(request.Context(), "req_test"),
	)

	NewResponderWithClock(func() time.Time { return now }).Data(
		ctx,
		http.StatusOK,
		gin.H{"status": "ok"},
	)

	var body struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Meta Meta `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Status != "ok" || body.Meta.RequestID != "req_test" ||
		!body.Meta.Timestamp.Equal(now) {
		t.Fatalf("response = %+v", body)
	}
}

func TestResponderHidesUnknownError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	NewResponder().Error(ctx, errors.New("private detail"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if body := recorder.Body.String(); body == "" ||
		contains(body, "private detail") {
		t.Fatalf("response exposed internal detail: %s", body)
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
