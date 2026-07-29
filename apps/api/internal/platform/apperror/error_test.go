package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestFromPreservesApplicationError(t *testing.T) {
	cause := errors.New("upstream detail")
	want := Wrap(CodeGitHubAPI, "GitHub is unavailable", http.StatusBadGateway, cause)

	got := From(want)

	if got != want {
		t.Fatalf("From() returned a different application error")
	}
	if !errors.Is(got, cause) {
		t.Fatalf("From() did not retain the wrapped cause")
	}
}

func TestFromHidesUnknownError(t *testing.T) {
	cause := errors.New("dial tcp private detail")

	got := From(cause)

	if got.Code != CodeInternal || got.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("From() = %+v, want internal error", got)
	}
	if got.Message == cause.Error() {
		t.Fatalf("From() exposed the underlying error")
	}
	if !errors.Is(got, cause) {
		t.Fatalf("From() did not retain the underlying cause")
	}
}
