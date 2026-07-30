package bootstrap

import (
	"context"

	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/database/postgres"
)

// NewDatabasePool builds the optional authenticated-feature database pool.
// A nil pool with configured=false is an expected anonymous-only runtime.
func NewDatabasePool(
	ctx context.Context,
	cfg config.Config,
) (pool *postgres.Pool, configured bool, err error) {
	if !cfg.DatabaseURL.IsSet() {
		return nil, false, nil
	}
	pool, err = postgres.Open(
		ctx,
		cfg.DatabaseURL.Value(),
		postgres.PoolSettings{
			ApplicationName:   "issuescout-api",
			ConnectTimeout:    cfg.DatabaseConnectTimeout,
			HealthCheckPeriod: cfg.DatabaseHealthCheckPeriod,
			MaxConnectionIdle: cfg.DatabaseMaxConnectionIdleTime,
			MaxConnectionLife: cfg.DatabaseMaxConnectionLifetime,
			MaxConnections:    cfg.DatabaseMaxConnections,
			MinConnections:    cfg.DatabaseMinConnections,
			QueryTimeout:      cfg.DatabaseQueryTimeout,
		},
	)
	if err != nil {
		return nil, true, err
	}

	return pool, true, nil
}
