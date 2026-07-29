package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewReturnsNotFoundForUnknownRoute(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/unknown", nil)

	New().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
