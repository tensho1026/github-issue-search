package bootstrap

import (
	"fmt"

	"github.com/tensho1026/github-issue-search/apps/api/internal/database/postgres"
	"github.com/tensho1026/github-issue-search/apps/api/internal/usecase"
)

// NewAccountWorkspace composes optional account-owned feature persistence.
// A nil pool is the expected anonymous-only runtime and yields a nil service.
func NewAccountWorkspace(
	databasePool *postgres.Pool,
) (usecase.AccountWorkspace, error) {
	if databasePool == nil {
		return nil, nil
	}
	repository, err := postgres.NewAccountRepository(databasePool)
	if err != nil {
		return nil, fmt.Errorf("compose account repository: %w", err)
	}
	return usecase.NewAccountWorkspace(repository), nil
}
