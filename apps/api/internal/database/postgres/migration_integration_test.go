package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMigrationsAgainstConfiguredPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	settings := integrationPoolSettings()
	adminPool, err := Open(ctx, databaseURL, settings)
	if err != nil {
		t.Fatal("Open() rejected TEST_DATABASE_URL")
	}
	t.Cleanup(adminPool.Close)
	schema := integrationSchemaName(t)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.client.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal("create isolated migration schema")
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(
			context.Background(),
			30*time.Second,
		)
		defer cleanupCancel()
		if _, err := adminPool.client.Exec(
			cleanupContext,
			"DROP SCHEMA "+identifier+" CASCADE",
		); err != nil {
			t.Error("drop isolated migration schema")
		}
	})

	isolatedURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal("parse TEST_DATABASE_URL")
	}
	parameters := isolatedURL.Query()
	parameters.Set("search_path", schema)
	isolatedURL.RawQuery = parameters.Encode()
	isolatedPool, err := Open(ctx, isolatedURL.String(), settings)
	if err != nil {
		t.Fatal("open isolated migration pool")
	}
	t.Cleanup(isolatedPool.Close)

	if err := isolatedPool.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := isolatedPool.Migrate(ctx); err != nil {
		t.Fatalf("idempotent Migrate() error = %v", err)
	}
	statuses, err := isolatedPool.MigrationStatus(ctx)
	if err != nil {
		t.Fatalf("MigrationStatus() error = %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("MigrationStatus() count = %d", len(statuses))
	}
	for _, status := range statuses {
		if status.AppliedAt == nil {
			t.Fatalf("migration %06d is pending", status.Version)
		}
	}
	var accountTable string
	if err := isolatedPool.client.QueryRow(
		ctx,
		"SELECT to_regclass('accounts')::text",
	).Scan(&accountTable); err != nil || accountTable != "accounts" {
		t.Fatal("migrated accounts table is unavailable")
	}
}

func integrationPoolSettings() PoolSettings {
	return PoolSettings{
		ApplicationName:   "issuescout-migration-integration-test",
		ConnectTimeout:    10 * time.Second,
		HealthCheckPeriod: 30 * time.Second,
		MaxConnectionIdle: time.Minute,
		MaxConnectionLife: 5 * time.Minute,
		MaxConnections:    2,
		MinConnections:    0,
		QueryTimeout:      30 * time.Second,
	}
}

func integrationSchemaName(t *testing.T) string {
	t.Helper()
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatal("generate migration schema name")
	}
	return "issuescout_test_" + hex.EncodeToString(random)
}
