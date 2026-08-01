// Command migrate applies and verifies IssueScout's forward-only PostgreSQL
// migration catalog without requiring Docker or another container runtime.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/bootstrap"
	"github.com/tensho1026/github-issue-search/apps/api/internal/config"
	"github.com/tensho1026/github-issue-search/apps/api/internal/database/postgres"
)

const migrationCommandTimeout = 2 * time.Minute

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) != 1 {
		printUsage(stderr)
		return 2
	}
	command := arguments[0]
	if command != "up" && command != "status" && command != "verify" {
		printUsage(stderr)
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, "Migration configuration is invalid.")
		return 1
	}
	if !cfg.DatabaseURL.IsSet() {
		fmt.Fprintln(stderr, "DATABASE_URL is required for migration commands.")
		return 1
	}
	signalContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, migrationCommandTimeout)
	defer cancel()
	pool, configured, err := bootstrap.NewDatabasePool(ctx, cfg)
	if err != nil || !configured || pool == nil {
		fmt.Fprintln(stderr, "Database pool configuration failed.")
		return 1
	}
	defer pool.Close()

	switch command {
	case "up":
		if err := pool.Migrate(ctx); err != nil {
			fmt.Fprintln(stderr, safeMigrationMessage(err))
			return 1
		}
		fmt.Fprintln(stdout, "All database migrations are applied.")
	case "status":
		statuses, err := pool.MigrationStatus(ctx)
		if err != nil {
			fmt.Fprintln(stderr, safeMigrationMessage(err))
			return 1
		}
		for _, status := range statuses {
			state := "pending"
			if status.AppliedAt != nil {
				state = "applied"
			}
			fmt.Fprintf(
				stdout,
				"%06d %-32s %s %s\n",
				status.Version,
				status.Name,
				state,
				status.Checksum,
			)
		}
	case "verify":
		statuses, err := pool.MigrationStatus(ctx)
		if err != nil {
			fmt.Fprintln(stderr, safeMigrationMessage(err))
			return 1
		}
		for _, status := range statuses {
			if status.AppliedAt == nil {
				fmt.Fprintln(stderr, "Database has pending migrations.")
				return 1
			}
		}
		fmt.Fprintln(stdout, "Database migration checksums are verified.")
	}

	return 0
}

func safeMigrationMessage(err error) string {
	switch {
	case errors.Is(err, postgres.ErrMigrationDrift):
		return "Database migration verification failed because schema history drifted."
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return "Database migration command was cancelled or timed out."
	default:
		return "Database migration command failed."
	}
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: go run ./cmd/migrate <up|status|verify>")
}
