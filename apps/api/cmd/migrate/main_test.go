package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/database/postgres"
)

func TestRunRequiresOneKnownCommand(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	for _, arguments := range [][]string{nil, {"unknown"}, {"status", "extra"}} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := run(arguments, &stdout, &stderr)
		if exitCode != 2 {
			t.Fatalf("run(%v) exit code = %d", arguments, exitCode)
		}
		if !strings.Contains(stderr.String(), "Usage:") {
			t.Fatalf("run(%v) stderr = %q", arguments, stderr.String())
		}
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"status"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("run() exit code = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL is required") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
}

func TestSafeMigrationMessageContainsNoDriverDetail(t *testing.T) {
	const driverDetail = "sensitive-host.example"
	message := safeMigrationMessage(errors.Join(
		postgres.ErrMigrationFailed,
		errors.New(driverDetail),
	))
	if strings.Contains(message, driverDetail) {
		t.Fatal("safeMigrationMessage() exposed driver details")
	}
}
