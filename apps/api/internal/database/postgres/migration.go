package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	migrationAdvisoryLock int64 = 7_152_048_587_679_201_021
	migrationDirectory          = "migrations"
)

var (
	// ErrMigrationFailed reports a database execution failure without
	// forwarding driver text that may contain connection details.
	ErrMigrationFailed = errors.New("database migration failed")
	// ErrMigrationDrift reports an unexpected version or checksum.
	ErrMigrationDrift = errors.New("database migration drift")
	migrationName     = regexp.MustCompile(`^(\d{6})_([a-z0-9_]+)\.sql$`)
	//go:embed migrations/*.sql
	migrationFiles embed.FS
)

// Migration describes one immutable forward-only schema change.
type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

// MigrationStatus describes whether an embedded migration has been applied.
type MigrationStatus struct {
	Version   int64
	Name      string
	Checksum  string
	AppliedAt *time.Time
}

// Migrate applies every pending migration in order under a PostgreSQL advisory
// lock. Each migration and checksum record commit atomically.
func (pool *Pool) Migrate(ctx context.Context) error {
	if pool == nil || pool.client == nil {
		return ErrUnavailable
	}
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	migrationContext, cancel := context.WithTimeout(ctx, pool.queryTimeout)
	defer cancel()
	connection, err := pool.client.Acquire(migrationContext)
	if err != nil {
		return ErrMigrationFailed
	}
	defer connection.Release()
	if _, err := connection.Exec(migrationContext, migrationTableSQL); err != nil {
		return ErrMigrationFailed
	}
	if _, err := connection.Exec(
		migrationContext,
		"SELECT pg_advisory_lock($1)",
		migrationAdvisoryLock,
	); err != nil {
		return ErrMigrationFailed
	}
	defer func() {
		unlockContext, unlockCancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			pool.queryTimeout,
		)
		defer unlockCancel()
		_, _ = connection.Exec(
			unlockContext,
			"SELECT pg_advisory_unlock($1)",
			migrationAdvisoryLock,
		)
	}()

	applied, err := readAppliedMigrations(migrationContext, connection)
	if err != nil {
		return err
	}
	if err := verifyMigrationSet(migrations, applied); err != nil {
		return err
	}
	for _, migration := range migrations {
		if _, exists := applied[migration.Version]; exists {
			continue
		}
		if err := applyMigration(migrationContext, connection, migration); err != nil {
			return err
		}
	}

	return nil
}

// MigrationStatus returns the ordered embedded migration catalog and its
// applied timestamps after verifying checksums.
func (pool *Pool) MigrationStatus(
	ctx context.Context,
) ([]MigrationStatus, error) {
	if pool == nil || pool.client == nil {
		return nil, ErrUnavailable
	}
	migrations, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	statusContext, cancel := context.WithTimeout(ctx, pool.queryTimeout)
	defer cancel()
	if _, err := pool.client.Exec(statusContext, migrationTableSQL); err != nil {
		return nil, ErrMigrationFailed
	}
	applied, err := readAppliedMigrations(statusContext, pool.client)
	if err != nil {
		return nil, err
	}
	if err := verifyMigrationSet(migrations, applied); err != nil {
		return nil, err
	}

	statuses := make([]MigrationStatus, 0, len(migrations))
	for _, migration := range migrations {
		status := MigrationStatus{
			Version:  migration.Version,
			Name:     migration.Name,
			Checksum: migration.Checksum,
		}
		if record, exists := applied[migration.Version]; exists {
			appliedAt := record.AppliedAt
			status.AppliedAt = &appliedAt
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

type appliedMigration struct {
	Checksum  string
	AppliedAt time.Time
}

type migrationQuerier interface {
	Query(
		ctx context.Context,
		sql string,
		args ...any,
	) (pgx.Rows, error)
}

type migrationConnection interface {
	migrationQuerier
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(
		ctx context.Context,
		sql string,
		arguments ...any,
	) (pgconn.CommandTag, error)
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, migrationDirectory)
	if err != nil {
		return nil, ErrMigrationDrift
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, ErrMigrationDrift
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if len(matches) != 3 {
			return nil, ErrMigrationDrift
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, ErrMigrationDrift
		}
		sqlBytes, err := migrationFiles.ReadFile(
			path.Join(migrationDirectory, entry.Name()),
		)
		if err != nil || len(sqlBytes) == 0 {
			return nil, ErrMigrationDrift
		}
		checksum := sha256.Sum256(sqlBytes)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     matches[2],
			SQL:      string(sqlBytes),
			Checksum: hex.EncodeToString(checksum[:]),
		})
	}
	sort.Slice(migrations, func(left, right int) bool {
		return migrations[left].Version < migrations[right].Version
	})
	for index, migration := range migrations {
		expected := int64(index + 1)
		if migration.Version != expected {
			return nil, ErrMigrationDrift
		}
	}

	return migrations, nil
}

func readAppliedMigrations(
	ctx context.Context,
	querier migrationQuerier,
) (map[int64]appliedMigration, error) {
	rows, err := querier.Query(
		ctx,
		`SELECT version, checksum, applied_at
		 FROM schema_migrations
		 ORDER BY version`,
	)
	if err != nil {
		return nil, ErrMigrationFailed
	}
	defer rows.Close()
	applied := make(map[int64]appliedMigration)
	for rows.Next() {
		var version int64
		var record appliedMigration
		if err := rows.Scan(
			&version,
			&record.Checksum,
			&record.AppliedAt,
		); err != nil {
			return nil, ErrMigrationFailed
		}
		applied[version] = record
	}
	if rows.Err() != nil {
		return nil, ErrMigrationFailed
	}

	return applied, nil
}

func verifyMigrationSet(
	migrations []Migration,
	applied map[int64]appliedMigration,
) error {
	known := make(map[int64]Migration, len(migrations))
	for _, migration := range migrations {
		known[migration.Version] = migration
	}
	for version, record := range applied {
		migration, exists := known[version]
		if !exists || migration.Checksum != record.Checksum {
			return fmt.Errorf(
				"%w: migration %06d does not match the embedded catalog",
				ErrMigrationDrift,
				version,
			)
		}
	}
	for version := int64(1); version <= int64(len(applied)); version++ {
		if _, exists := applied[version]; !exists {
			return fmt.Errorf(
				"%w: migration %06d is missing from the applied prefix",
				ErrMigrationDrift,
				version,
			)
		}
	}

	return nil
}

func applyMigration(
	ctx context.Context,
	connection migrationConnection,
	migration Migration,
) error {
	tx, err := connection.Begin(ctx)
	if err != nil {
		return ErrMigrationFailed
	}
	defer func() {
		rollbackContext, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			5*time.Second,
		)
		defer cancel()
		_ = tx.Rollback(rollbackContext)
	}()
	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf(
			"%w: apply migration %06d",
			ErrMigrationFailed,
			migration.Version,
		)
	}
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO schema_migrations (version, name, checksum)
		 VALUES ($1, $2, $3)`,
		migration.Version,
		migration.Name,
		migration.Checksum,
	); err != nil {
		return fmt.Errorf(
			"%w: record migration %06d",
			ErrMigrationFailed,
			migration.Version,
		)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf(
			"%w: commit migration %06d",
			ErrMigrationFailed,
			migration.Version,
		)
	}

	return nil
}

const migrationTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint PRIMARY KEY,
    name text NOT NULL UNIQUE,
    checksum char(64) NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`
