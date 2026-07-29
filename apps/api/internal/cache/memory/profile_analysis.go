package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// ProfileAnalysis is a bounded, concurrency-safe LRU cache. Its port can be
// replaced by a distributed cache without changing the analysis usecase.
type ProfileAnalysis struct {
	store *lruCache[string, port.ProfileAnalysisCacheEntry]
}

func NewProfileAnalysis(
	capacity int,
	ttl time.Duration,
) (*ProfileAnalysis, error) {
	store, err := newLRUCache[string, port.ProfileAnalysisCacheEntry](
		capacity,
		ttl,
		cloneProfileAnalysisEntry,
	)
	if err != nil {
		return nil, fmt.Errorf("create profile analysis cache: %w", err)
	}
	return &ProfileAnalysis{store: store}, nil
}

func (c *ProfileAnalysis) Get(
	ctx context.Context,
	username user.Username,
) (port.ProfileAnalysisCacheEntry, bool, error) {
	return c.store.get(ctx, profileAnalysisKey(username))
}

func (c *ProfileAnalysis) Set(
	ctx context.Context,
	username user.Username,
	entry port.ProfileAnalysisCacheEntry,
) error {
	return c.store.set(ctx, profileAnalysisKey(username), entry)
}

func profileAnalysisKey(username user.Username) string {
	return "github:profile-analysis:" + strings.ToLower(username.String())
}

func cloneProfileAnalysisEntry(
	entry port.ProfileAnalysisCacheEntry,
) port.ProfileAnalysisCacheEntry {
	cloned := entry
	cloned.Analysis = profile.Analysis{
		Username:             entry.Analysis.Username,
		Languages:            append([]profile.LanguageShare(nil), entry.Analysis.Languages...),
		Frameworks:           append([]string(nil), entry.Analysis.Frameworks...),
		RepositoriesAnalyzed: entry.Analysis.RepositoriesAnalyzed,
		Warnings:             append([]profile.Warning(nil), entry.Analysis.Warnings...),
	}
	return cloned
}

var _ port.ProfileAnalysisCache = (*ProfileAnalysis)(nil)
