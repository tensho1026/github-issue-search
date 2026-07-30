package postgres

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedMigrationsAreOrderedAndForwardOnly(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("migration count = %d, want 2", len(migrations))
	}
	for index, migration := range migrations {
		if migration.Version != int64(index+1) {
			t.Fatalf("migration version = %d", migration.Version)
		}
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(migration.Checksum) {
			t.Fatalf("migration checksum = %q", migration.Checksum)
		}
		normalized := strings.ToUpper(migration.SQL)
		for _, forbidden := range []string{
			"BEGIN;",
			"COMMIT;",
			"DROP TABLE",
			"TRUNCATE ",
			"POSTGRESQL://",
			"POSTGRES://",
		} {
			if strings.Contains(normalized, forbidden) {
				t.Fatalf(
					"migration %06d contains forbidden %q",
					migration.Version,
					forbidden,
				)
			}
		}
	}
}

func TestVerifyMigrationSetRejectsChecksumAndUnknownVersion(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	appliedAt := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		applied map[int64]appliedMigration
	}{
		{
			name: "checksum mismatch",
			applied: map[int64]appliedMigration{
				1: {AppliedAt: appliedAt, Checksum: strings.Repeat("0", 64)},
			},
		},
		{
			name: "unknown version",
			applied: map[int64]appliedMigration{
				99: {AppliedAt: appliedAt, Checksum: strings.Repeat("0", 64)},
			},
		},
		{
			name: "non-contiguous applied set",
			applied: map[int64]appliedMigration{
				2: {
					AppliedAt: appliedAt,
					Checksum:  migrations[1].Checksum,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifyMigrationSet(migrations, test.applied)
			if !errors.Is(err, ErrMigrationDrift) {
				t.Fatalf("verifyMigrationSet() error = %v", err)
			}
		})
	}
}

func TestVerifyMigrationSetAcceptsKnownPrefix(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	err = verifyMigrationSet(
		migrations,
		map[int64]appliedMigration{
			migrations[0].Version: {
				AppliedAt: time.Now(),
				Checksum:  migrations[0].Checksum,
			},
		},
	)
	if err != nil {
		t.Fatalf("verifyMigrationSet() error = %v", err)
	}
}
