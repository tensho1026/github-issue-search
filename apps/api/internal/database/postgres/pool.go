// Package postgres provides the server-only PostgreSQL adapter used by
// authenticated IssueScout capabilities.
package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrInvalidConfiguration reports unusable pool configuration without
	// including the connection string or its credentials.
	ErrInvalidConfiguration = errors.New("invalid database configuration")
	// ErrUnavailable reports a failed readiness probe without forwarding driver
	// details that may contain connection parameters.
	ErrUnavailable = errors.New("database unavailable")
)

// PoolSettings contains non-sensitive, bounded connection-pool policy.
type PoolSettings struct {
	ApplicationName   string
	ConnectTimeout    time.Duration
	HealthCheckPeriod time.Duration
	MaxConnectionIdle time.Duration
	MaxConnectionLife time.Duration
	MaxConnections    int
	MinConnections    int
	QueryTimeout      time.Duration
}

// Pool owns the authenticated-feature connection pool. Anonymous feature
// handlers do not depend on this type.
type Pool struct {
	client       *pgxpool.Pool
	queryTimeout time.Duration
}

// Open validates and constructs a lazy PostgreSQL pool. It deliberately does
// not ping the database so a temporary database outage cannot prevent the
// anonymous API from starting.
func Open(
	ctx context.Context,
	databaseURL string,
	settings PoolSettings,
) (*Pool, error) {
	poolConfig, err := buildPoolConfig(databaseURL, settings)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	client, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}

	return &Pool{
		client:       client,
		queryTimeout: settings.QueryTimeout,
	}, nil
}

// Ping performs a bounded readiness probe. The returned error is intentionally
// generic so hostnames, usernames, and database parameters cannot reach logs
// or API responses.
func (pool *Pool) Ping(ctx context.Context) error {
	if pool == nil || pool.client == nil {
		return ErrUnavailable
	}
	probeContext, cancel := context.WithTimeout(ctx, pool.queryTimeout)
	defer cancel()
	if err := pool.client.Ping(probeContext); err != nil {
		return ErrUnavailable
	}

	return nil
}

// Close releases connections owned by the authenticated persistence adapter.
func (pool *Pool) Close() {
	if pool != nil && pool.client != nil {
		pool.client.Close()
	}
}

func buildPoolConfig(
	databaseURL string,
	settings PoolSettings,
) (*pgxpool.Config, error) {
	if databaseURL == "" ||
		settings.ApplicationName == "" ||
		settings.ConnectTimeout <= 0 ||
		settings.HealthCheckPeriod <= 0 ||
		settings.MaxConnectionIdle <= 0 ||
		settings.MaxConnectionLife <= 0 ||
		settings.MaxConnections < 1 ||
		settings.MaxConnections > 100 ||
		settings.MinConnections < 0 ||
		settings.MinConnections > settings.MaxConnections ||
		settings.QueryTimeout <= 0 {
		return nil, ErrInvalidConfiguration
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	if sslMode := poolConfig.ConnConfig.Config.RuntimeParams["sslmode"]; sslMode != "" {
		// sslmode is consumed by pgx while parsing and is normally absent from
		// RuntimeParams. This guard exists only for future parser behavior.
		delete(poolConfig.ConnConfig.Config.RuntimeParams, "sslmode")
	}
	poolConfig.ConnConfig.ConnectTimeout = settings.ConnectTimeout
	poolConfig.ConnConfig.RuntimeParams["application_name"] =
		settings.ApplicationName
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] =
		durationMilliseconds(settings.QueryTimeout)
	poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] =
		durationMilliseconds(settings.QueryTimeout)
	poolConfig.ConnConfig.RuntimeParams["lock_timeout"] =
		durationMilliseconds(min(settings.QueryTimeout, 2*time.Second))
	poolConfig.MaxConns = int32(settings.MaxConnections)
	poolConfig.MinConns = int32(settings.MinConnections)
	poolConfig.MaxConnLifetime = settings.MaxConnectionLife
	poolConfig.MaxConnIdleTime = settings.MaxConnectionIdle
	poolConfig.HealthCheckPeriod = settings.HealthCheckPeriod

	return poolConfig, nil
}

func durationMilliseconds(value time.Duration) string {
	return strconv.FormatInt(value.Milliseconds(), 10)
}
