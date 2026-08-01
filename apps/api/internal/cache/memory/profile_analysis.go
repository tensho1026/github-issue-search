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

// NewProfileAnalysis constructs an LRU cache with positive capacity and TTL.
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

// Get returns a deep copy, reports misses without error, and honors a
// pre-cancelled context before acquiring the cache lock.
func (c *ProfileAnalysis) Get(
	ctx context.Context,
	username user.Username,
) (port.ProfileAnalysisCacheEntry, bool, error) {
	return c.store.get(ctx, profileAnalysisKey(username))
}

// Set stores a deep copy, refreshes TTL on replacement, and may evict the
// least-recently-used entry.
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
	cloned.Analysis = entry.Analysis
	cloned.Analysis.Languages = append(
		[]profile.LanguageShare(nil),
		entry.Analysis.Languages...,
	)
	cloned.Analysis.Frameworks = append(
		[]string(nil),
		entry.Analysis.Frameworks...,
	)
	cloned.Analysis.RecentTechnologies = cloneRecentTechnologies(
		entry.Analysis.RecentTechnologies,
	)
	cloned.Analysis.OSSExperience.Evidence = append(
		[]profile.TechnologyEvidence(nil),
		entry.Analysis.OSSExperience.Evidence...,
	)
	cloned.Analysis.RepositoryEvidence = cloneRepositoryEvidence(
		entry.Analysis.RepositoryEvidence,
	)
	cloned.Analysis.Proficiency = cloneTechnologyProficiency(
		entry.Analysis.Proficiency,
	)
	cloned.Analysis.Warnings = append(
		[]profile.Warning(nil),
		entry.Analysis.Warnings...,
	)
	return cloned
}

func cloneRecentTechnologies(
	source []profile.RecentTechnology,
) []profile.RecentTechnology {
	cloned := make([]profile.RecentTechnology, len(source))
	for index, technology := range source {
		cloned[index] = technology
		cloned[index].RepositorySources = append(
			[]profile.RepositorySource(nil),
			technology.RepositorySources...,
		)
	}
	return cloned
}

func cloneRepositoryEvidence(
	source profile.RepositoryEvidence,
) profile.RepositoryEvidence {
	return profile.RepositoryEvidence{
		Owned:       cloneRepositorySample(source.Owned),
		Contributed: cloneRepositorySample(source.Contributed),
		Starred:     cloneRepositorySample(source.Starred),
		Forked:      cloneRepositorySample(source.Forked),
	}
}

func cloneRepositorySample(
	source profile.RepositorySample,
) profile.RepositorySample {
	cloned := source
	if source.Total != nil {
		total := *source.Total
		cloned.Total = &total
	}
	cloned.PrimaryTechnologies = append(
		[]profile.LanguageShare(nil),
		source.PrimaryTechnologies...,
	)
	return cloned
}

func cloneTechnologyProficiency(
	source []profile.TechnologyProficiency,
) []profile.TechnologyProficiency {
	cloned := make([]profile.TechnologyProficiency, len(source))
	for index, technology := range source {
		cloned[index] = technology
		cloned[index].Evidence = append(
			[]profile.TechnologyEvidence(nil),
			technology.Evidence...,
		)
	}
	return cloned
}

var _ port.ProfileAnalysisCache = (*ProfileAnalysis)(nil)
