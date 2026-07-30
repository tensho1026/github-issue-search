package usecase

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/sync/singleflight"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

type AnalyzeGitHubProfileOutput struct {
	Analysis  profile.Analysis
	RateLimit port.RateLimit
}

type AnalyzeGitHubProfile interface {
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
	requests        singleflight.Group
}

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
		return outputFromCache(cached), nil
	} else if err != nil && ctx.Err() != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}

	key := strings.ToLower(username.String())
	resultChannel := u.requests.DoChan(key, func() (any, error) {
		if cached, found, err := u.cache.Get(ctx, username); err == nil && found {
			return outputFromCache(cached), nil
		} else if err != nil && ctx.Err() != nil {
			return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
		}
		return u.analyze(ctx, username)
	})

	select {
	case <-ctx.Done():
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(ctx.Err())
	case result := <-resultChannel:
		if result.Err != nil {
			return AnalyzeGitHubProfileOutput{}, result.Err
		}
		output, ok := result.Val.(AnalyzeGitHubProfileOutput)
		if !ok {
			return AnalyzeGitHubProfileOutput{}, errors.New(
				"invalid profile analysis result",
			)
		}
		return output, nil
	}
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

	return outputFromCache(entry), nil
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
) AnalyzeGitHubProfileOutput {
	return AnalyzeGitHubProfileOutput{
		Analysis:  entry.Analysis,
		RateLimit: entry.RateLimit,
	}
}

var _ AnalyzeGitHubProfile = (*analyzeGitHubProfile)(nil)
