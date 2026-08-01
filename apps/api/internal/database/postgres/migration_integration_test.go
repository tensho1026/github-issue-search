package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationsAgainstConfiguredPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	settings := integrationPoolSettings()
	adminPool, openErr := Open(ctx, databaseURL, settings)
	if openErr != nil {
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

	isolatedPool := openIntegrationPoolInSchema(
		t,
		ctx,
		databaseURL,
		schema,
	)

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

func openIntegrationPoolInSchema(
	t *testing.T,
	ctx context.Context,
	databaseURL string,
	schema string,
) *Pool {
	t.Helper()
	settings := integrationPoolSettings()
	directURL, err := directIntegrationDatabaseURL(databaseURL)
	if err != nil {
		t.Fatal("derive direct integration database endpoint")
	}
	poolConfig, err := buildPoolConfig(directURL, settings)
	if err != nil {
		t.Fatal("build isolated integration pool configuration")
	}
	schemaIdentifier := pgx.Identifier{schema}.Sanitize()
	poolConfig.AfterConnect = func(
		connectionContext context.Context,
		connection *pgx.Conn,
	) error {
		_, execErr := connection.Exec(
			connectionContext,
			"SET search_path TO "+schemaIdentifier,
		)
		return execErr
	}
	client, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal("open isolated integration pool")
	}
	pool := &Pool{
		client:       client,
		queryTimeout: settings.QueryTimeout,
	}
	t.Cleanup(pool.Close)
	var currentSchema string
	if err := pool.client.QueryRow(
		ctx,
		"SELECT current_schema()",
	).Scan(&currentSchema); err != nil || currentSchema != schema {
		t.Fatalf(
			"isolated integration search path resolved to %q",
			currentSchema,
		)
	}
	return pool
}

func directIntegrationDatabaseURL(databaseURL string) (string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	hostname := strings.Replace(parsed.Hostname(), "-pooler.", ".", 1)
	if hostname == "" {
		return "", ErrInvalidConfiguration
	}
	if port := parsed.Port(); port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else {
		parsed.Host = hostname
	}
	return parsed.String(), nil
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
