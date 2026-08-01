package bootstrap

import (
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
)

func TestNewAuthenticationLeavesAnonymousRuntimeDatabaseIndependent(
	t *testing.T,
) {
	dependencies, err := NewAuthentication(config.Config{}, nil)
	if err != nil {
		t.Fatalf("NewAuthentication() error = %v", err)
	}
	if dependencies.Service != nil || dependencies.FlowCodec != nil {
		t.Fatalf("dependencies = %+v", dependencies)
	}
}

func TestNewAuthenticationRequiresDatabaseWhenEnabled(t *testing.T) {
	_, err := NewAuthentication(config.Config{AuthEnabled: true}, nil)
	if err == nil {
		t.Fatal("NewAuthentication() error = nil")
	}
}
