package port

import (
	"context"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/account"
)

// DatabaseHealth probes the optional authenticated-feature database.
type DatabaseHealth interface {
	Ping(context.Context) error
}

// AccountRepository owns account-scoped persistence and deletion semantics.
type AccountRepository interface {
	OwnedDataSummary(
		ctx context.Context,
		accountID account.ID,
	) (account.OwnedDataSummary, error)
	Delete(ctx context.Context, accountID account.ID) error
}
