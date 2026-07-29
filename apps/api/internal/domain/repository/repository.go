package repository

import "time"

// Summary is the normalized public repository data exposed by the user
// profile flow and reused by later analysis usecases.
type Summary struct {
	ID            int64
	Owner         string
	Name          string
	FullName      string
	Description   string
	URL           string
	MainLanguage  string
	Stars         int
	Forks         int
	OpenIssues    int
	IsFork        bool
	IsArchived    bool
	DefaultBranch string
	UpdatedAt     time.Time
	PushedAt      time.Time
}
