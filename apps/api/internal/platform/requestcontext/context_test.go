package requestcontext

import (
	"context"
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

func TestRequestValuesRoundTripAndRemainOptional(t *testing.T) {
	base := context.Background()
	if RequestID(base) != "" {
		t.Fatal("empty context unexpectedly contained a request ID")
	}
	if _, ok := Principal(base); ok {
		t.Fatal("empty context unexpectedly contained a principal")
	}
	requestContext := WithRequestID(base, "req_context")
	requestContext = WithPrincipal(
		requestContext,
		auth.Principal{Session: auth.Session{ID: "session-context"}},
	)
	if RequestID(requestContext) != "req_context" {
		t.Fatalf("RequestID() = %q", RequestID(requestContext))
	}
	principal, ok := Principal(requestContext)
	if !ok || principal.Session.ID != "session-context" {
		t.Fatalf("Principal() = %+v, %t", principal, ok)
	}
}
