package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildPoolConfigAppliesBoundedPolicy(t *testing.T) {
	databaseURL := testDatabaseURL("pool-test-value")
	settings := testPoolSettings()

	poolConfig, err := buildPoolConfig(databaseURL, settings)
	if err != nil {
		t.Fatalf("buildPoolConfig() error = %v", err)
	}

	if poolConfig.MaxConns != 7 || poolConfig.MinConns != 1 {
		t.Fatalf(
			"connections = %d/%d, want 1/7",
			poolConfig.MinConns,
			poolConfig.MaxConns,
		)
	}
	if poolConfig.ConnConfig.ConnectTimeout != 4*time.Second ||
		poolConfig.MaxConnLifetime != 20*time.Minute ||
		poolConfig.MaxConnIdleTime != 3*time.Minute ||
		poolConfig.HealthCheckPeriod != 15*time.Second {
		t.Fatal("pool durations did not preserve the bounded settings")
	}
	parameters := poolConfig.ConnConfig.RuntimeParams
	if parameters["application_name"] != "issuescout-test" ||
		parameters["statement_timeout"] != "3000" ||
		parameters["idle_in_transaction_session_timeout"] != "3000" ||
		parameters["lock_timeout"] != "2000" {
		t.Fatalf("runtime parameters = %v", parameters)
	}
	if poolConfig.ConnConfig.Config.Password != "pool-test-value" {
		t.Fatal("driver config did not receive the credential")
	}
}

func TestBuildPoolConfigRejectsUnboundedSettingsWithoutCredentialLeak(
	t *testing.T,
) {
	const password = "pool-sensitive-value"
	settings := testPoolSettings()
	settings.MaxConnections = 0

	_, err := buildPoolConfig(testDatabaseURL(password), settings)
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("buildPoolConfig() error = %v", err)
	}
	if strings.Contains(err.Error(), password) {
		t.Fatal("configuration error exposed the database password")
	}
}

func TestPingMapsCancellationToUnavailable(t *testing.T) {
	settings := testPoolSettings()
	settings.MinConnections = 0
	pool, err := Open(
		context.Background(),
		testDatabaseURL("cancel-test-value"),
		settings,
	)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pool.Ping(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Ping() error = %v, want unavailable", err)
	}
}

func TestNilPoolIsUnavailable(t *testing.T) {
	var pool *Pool
	if err := pool.Ping(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Ping() error = %v, want unavailable", err)
	}
	pool.Close()
}

func testDatabaseURL(password string) string {
	return fmt.Sprintf(
		"postgresql://owner:%s@127.0.0.1:1/issuescout?sslmode=require",
		password,
	)
}

func testPoolSettings() PoolSettings {
	return PoolSettings{
		ApplicationName:   "issuescout-test",
		ConnectTimeout:    4 * time.Second,
		HealthCheckPeriod: 15 * time.Second,
		MaxConnectionIdle: 3 * time.Minute,
		MaxConnectionLife: 20 * time.Minute,
		MaxConnections:    7,
		MinConnections:    1,
		QueryTimeout:      3 * time.Second,
	}
}
