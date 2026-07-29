package usecase

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

const (
	warningLanguageUnavailable = "language_data_unavailable"
	warningManifestUnavailable = "manifest_data_unavailable"
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
	maxConcurrency  int
	requests        singleflight.Group
}

type repositoryAnalysis struct {
	languages map[string]int64
	manifests []profile.Manifest
	warnings  []profile.Warning
	rateLimit port.RateLimit
}

func NewAnalyzeGitHubProfile(
	reader port.GitHubProfileAnalysisReader,
	cache port.ProfileAnalysisCache,
	repositoryLimit int,
	manifestLimit int,
	maxConcurrency int,
) AnalyzeGitHubProfile {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	return &analyzeGitHubProfile{
		reader:          reader,
		cache:           cache,
		repositoryLimit: repositoryLimit,
		manifestLimit:   manifestLimit,
		maxConcurrency:  maxConcurrency,
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
	userResult, err := u.reader.GetUser(ctx, username)
	if err != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}

	repositories, repositoryRateLimit, err := u.reader.ListRepositories(
		ctx,
		username,
		u.repositoryLimit,
	)
	if err != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}
	selected := profile.SelectRepositories(repositories, u.repositoryLimit)
	results := make([]repositoryAnalysis, len(selected))

	group, groupContext := errgroup.WithContext(ctx)
	group.SetLimit(u.maxConcurrency)
	for index, item := range selected {
		index := index
		item := item
		group.Go(func() error {
			result, analysisErr := u.analyzeRepository(groupContext, item)
			if analysisErr != nil {
				return analysisErr
			}
			results[index] = result
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return AnalyzeGitHubProfileOutput{}, mapGitHubUserError(err)
	}

	languageBytes := make([]map[string]int64, 0, len(results))
	manifests := make([]profile.Manifest, 0)
	warnings := make([]profile.Warning, 0)
	rateLimit := mergeRateLimits(userResult.RateLimit, repositoryRateLimit)
	for _, result := range results {
		languageBytes = append(languageBytes, result.languages)
		manifests = append(manifests, result.manifests...)
		warnings = append(warnings, result.warnings...)
		rateLimit = mergeRateLimits(rateLimit, result.rateLimit)
	}

	analysis := profile.Analysis{
		Username:             userResult.Profile.Login,
		Languages:            profile.AggregateLanguages(languageBytes),
		Frameworks:           profile.InferFrameworks(manifests),
		RepositoriesAnalyzed: len(selected),
		Warnings:             warnings,
	}
	entry := port.ProfileAnalysisCacheEntry{
		Analysis:  analysis,
		RateLimit: rateLimit,
	}
	_ = u.cache.Set(ctx, username, entry)

	return outputFromCache(entry), nil
}

func (u *analyzeGitHubProfile) analyzeRepository(
	ctx context.Context,
	item repository.Summary,
) (repositoryAnalysis, error) {
	result := repositoryAnalysis{
		languages: make(map[string]int64),
		manifests: make([]profile.Manifest, 0),
		warnings:  make([]profile.Warning, 0),
	}

	languages, err := u.reader.GetRepositoryLanguages(
		ctx,
		item.Owner,
		item.Name,
	)
	if err != nil {
		if ctx.Err() != nil {
			return repositoryAnalysis{}, ctx.Err()
		}
		result.warnings = append(result.warnings, profile.Warning{
			Code:       warningLanguageUnavailable,
			Message:    "Language data could not be retrieved",
			Repository: item.FullName,
		})
	} else {
		result.languages = languages.Languages
		result.rateLimit = mergeRateLimits(result.rateLimit, languages.RateLimit)
	}

	for _, manifestPath := range profile.ManifestCandidates(
		item.MainLanguage,
		u.manifestLimit,
	) {
		file, fileErr := u.reader.GetRepositoryFile(
			ctx,
			item.Owner,
			item.Name,
			manifestPath,
		)
		if fileErr != nil {
			if ctx.Err() != nil {
				return repositoryAnalysis{}, ctx.Err()
			}
			result.warnings = append(result.warnings, profile.Warning{
				Code:       warningManifestUnavailable,
				Message:    "A framework manifest could not be retrieved",
				Repository: item.FullName,
			})
			continue
		}
		result.rateLimit = mergeRateLimits(result.rateLimit, file.RateLimit)
		if file.Exists {
			result.manifests = append(result.manifests, profile.Manifest{
				Path:    manifestPath,
				Content: file.Content,
			})
		}
	}

	return result, nil
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
