package bootstrap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
)

func TestNewDatabasePoolKeepsAnonymousRuntimePoolFree(t *testing.T) {
	pool, configured, err := NewDatabasePool(
		context.Background(),
		config.Config{},
	)
	if err != nil {
		t.Fatalf("NewDatabasePool() error = %v", err)
	}
	if configured || pool != nil {
		t.Fatal("anonymous-only runtime unexpectedly created a database pool")
	}
}

func TestNewDatabasePoolBuildsLazyBoundedPool(t *testing.T) {
	t.Setenv(
		"DATABASE_URL",
		fmt.Sprintf(
			"postgresql://owner:%s@127.0.0.1:1/issuescout?sslmode=require",
			"bootstrap-test-value",
		),
	)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.DatabaseConnectTimeout = 10 * time.Millisecond
	pool, configured, err := NewDatabasePool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewDatabasePool() error = %v", err)
	}
	if !configured || pool == nil {
		t.Fatal("configured runtime did not create a database pool")
	}
	pool.Close()
}
