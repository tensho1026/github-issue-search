package usecase

import (
	"context"
	"strings"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/coalesce"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

// AnalyzeGitHubProfileOutput combines derived profile evidence with the
// GitHub quota snapshot observed by the underlying load.
type AnalyzeGitHubProfileOutput struct {
	Analysis  profile.Analysis
	RateLimit port.RateLimit
	CacheHit  bool
}

// AnalyzeGitHubProfile returns cached or freshly derived public profile
// evidence. Implementations must honor ctx and collapse concurrent misses for
// the same username.
type AnalyzeGitHubProfile interface {
	// Execute returns cached or freshly loaded analysis, honors ctx, and maps
	// upstream failures to safe application errors.
	Execute(
		ctx context.Context,
		username user.Username,
	) (AnalyzeGitHubProfileOutput, error)
}

type analyzeGitHubProfile struct {
	reader          port.GitHubProfileAnalysisReader
	cache           port.ProfileAnalysisCache
	repositoryLimit int
	manifestLimit   int
	requests        coalesce.Group[string, AnalyzeGitHubProfileOutput]
}

// NewAnalyzeGitHubProfile composes a bounded reader and ownership-isolating
// cache. Limits control repository and manifest fan-out per cache miss.
func NewAnalyzeGitHubProfile(
	reader port.GitHubProfileAnalysisReader,
	cache port.ProfileAnalysisCache,
	repositoryLimit int,
	manifestLimit int,
) AnalyzeGitHubProfile {
	return &analyzeGitHubProfile{
		reader:          reader,
		cache:           cache,
		repositoryLimit: repositoryLimit,
		manifestLimit:   manifestLimit,
	}
}

func (u *analyzeGitHubProfile) Execute(
	ctx context.Context,
	username user.Username,
) (AnalyzeGitHubProfileOutput, error) {
	if cached, found, err := u.cache.Get(ctx, username); err == nil && found {
		return outputFromCache(cached, true), nil
	} else if err != nil && ctx.Err() != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}

	key := strings.ToLower(username.String())
	output, err := u.requests.Do(ctx, key, func(
		sharedContext context.Context,
	) (AnalyzeGitHubProfileOutput, error) {
		if cached, found, err := u.cache.Get(
			sharedContext,
			username,
		); err == nil && found {
			return outputFromCache(cached, true), nil
		} else if err != nil && sharedContext.Err() != nil {
			return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
		}
		return u.analyze(sharedContext, username)
	})
	if err != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}
	return output, nil
}

func (u *analyzeGitHubProfile) analyze(
	ctx context.Context,
	username user.Username,
) (AnalyzeGitHubProfileOutput, error) {
	result, err := u.reader.GetProfileAnalysis(
		ctx,
		username,
		u.repositoryLimit,
		u.manifestLimit,
	)
	if err != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}
	entry := port.ProfileAnalysisCacheEntry{
		Analysis:  profile.AnalyzeSnapshot(result.Snapshot),
		RateLimit: result.RateLimit,
	}
	_ = u.cache.Set(ctx, username, entry)

	return outputFromCache(entry, false), nil
}

func mergeRateLimits(left, right port.RateLimit) port.RateLimit {
	if !right.Known {
		return left
	}
	if !left.Known || right.Remaining < left.Remaining {
		return right
	}
	if right.Remaining == left.Remaining && right.Reset.After(left.Reset) {
		return right
	}
	return left
}

func outputFromCache(
	entry port.ProfileAnalysisCacheEntry,
	cacheHit bool,
) AnalyzeGitHubProfileOutput {
	return AnalyzeGitHubProfileOutput{
		Analysis:  entry.Analysis,
		RateLimit: entry.RateLimit,
		CacheHit:  cacheHit,
	}
}

var _ AnalyzeGitHubProfile = (*analyzeGitHubProfile)(nil)
