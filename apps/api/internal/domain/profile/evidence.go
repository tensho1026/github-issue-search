package profile

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

const (
	AnalysisWindowDays       = 365
	MaximumTechnologyResults = 20
	maximumLanguageResults   = 10
)

type EvidenceStatus string

const (
	EvidenceExact       EvidenceStatus = "exact"
	EvidenceSampled     EvidenceStatus = "sampled"
	EvidenceUnavailable EvidenceStatus = "unavailable"
)

type Confidence string

const (
	ConfidenceHigh        Confidence = "high"
	ConfidenceMedium      Confidence = "medium"
	ConfidenceLow         Confidence = "low"
	ConfidenceUnavailable Confidence = "unavailable"
)

type TechnologyKind string

const (
	TechnologyLanguage  TechnologyKind = "language"
	TechnologyFramework TechnologyKind = "framework"
)

type RepositorySource string

const (
	RepositoryOwned       RepositorySource = "owned"
	RepositoryContributed RepositorySource = "contributed"
	RepositoryStarred     RepositorySource = "starred"
	RepositoryForked      RepositorySource = "forked"
)

type AnalysisWindow struct {
	From       time.Time
	To         time.Time
	Days       int
	PublicOnly bool
}

type CountEvidence struct {
	Available bool
	Value     int
	Complete  bool
}

type CountMetric struct {
	Value  int
	Status EvidenceStatus
}

type RepositoryObservation struct {
	Repository repository.Summary
	Languages  map[string]int64
	Manifests  []Manifest
}

type RepositoryCollection struct {
	Available    bool
	Repositories []RepositoryObservation
	Total        int
	TotalKnown   bool
	Limit        int
	HasMore      bool
}

type ProfileSnapshot struct {
	Username      user.Username
	WindowFrom    time.Time
	WindowTo      time.Time
	Owned         RepositoryCollection
	Contributed   RepositoryCollection
	Starred       RepositoryCollection
	Forked        RepositoryCollection
	Contributions ContributionSnapshot
	Warnings      []Warning
}

type ContributionSnapshot struct {
	Available           bool
	Commits             CountEvidence
	IssuesOpened        CountEvidence
	PullRequestsOpened  CountEvidence
	PullRequestReviews  CountEvidence
	RepositoriesTouched CountEvidence
}

type RepositorySample struct {
	Status              EvidenceStatus
	Observed            int
	Total               *int
	Limit               int
	ActiveInWindow      int
	PrimaryTechnologies []LanguageShare
}

type RepositoryEvidence struct {
	Owned       RepositorySample
	Contributed RepositorySample
	Starred     RepositorySample
	Forked      RepositorySample
}

type ContributionAnalysis struct {
	WindowDays          int
	Commits             CountMetric
	IssuesOpened        CountMetric
	PullRequestsOpened  CountMetric
	PullRequestReviews  CountMetric
	RepositoriesTouched CountMetric
}

type TechnologyEvidence struct {
	Kind   string
	Value  int
	Status EvidenceStatus
}

type OSSExperience struct {
	Level      string
	Confidence Confidence
	PublicOnly bool
	Evidence   []TechnologyEvidence
}

type RecentTechnology struct {
	Name              string
	Kind              TechnologyKind
	LastUsedAt        time.Time
	RepositoryCount   int
	RepositorySources []RepositorySource
	Confidence        Confidence
}

type TechnologyProficiency struct {
	Name       string
	Kind       TechnologyKind
	Level      int
	Label      string
	Score      int
	Confidence Confidence
	Evidence   []TechnologyEvidence
}

// AnalyzeSnapshot derives bounded, explainable profile evidence without
// performing I/O. Private activity is intentionally absent from ProfileSnapshot.
func AnalyzeSnapshot(snapshot ProfileSnapshot) Analysis {
	window := normalizeWindow(snapshot.WindowFrom, snapshot.WindowTo)
	ownedLanguages := topLanguageShares(
		AggregateLanguages(collectionLanguageBytes(snapshot.Owned)),
		maximumLanguageResults,
	)
	frameworkEvidence := collectionFrameworkEvidence(snapshot.Owned)
	frameworks := sortedFrameworkNames(frameworkEvidence)
	recent := analyzeRecentTechnologies(snapshot, window.From)
	contributions := analyzeContributions(snapshot.Contributions)
	repositoryEvidence := RepositoryEvidence{
		Owned:       analyzeRepositoryCollection(snapshot.Owned, window.From),
		Contributed: analyzeRepositoryCollection(snapshot.Contributed, window.From),
		Starred:     analyzeRepositoryCollection(snapshot.Starred, window.From),
		Forked:      analyzeRepositoryCollection(snapshot.Forked, window.From),
	}

	return Analysis{
		Username:           snapshot.Username,
		Languages:          ownedLanguages,
		Frameworks:         frameworks,
		RecentTechnologies: recent,
		Contributions:      contributions,
		OSSExperience:      analyzeOSSExperience(contributions),
		RepositoryEvidence: repositoryEvidence,
		Proficiency: buildTechnologyProficiency(
			snapshot,
			ownedLanguages,
			frameworkEvidence,
			recent,
		),
		Window:               window,
		RepositoriesAnalyzed: len(snapshot.Owned.Repositories),
		Warnings:             slices.Clone(snapshot.Warnings),
	}
}

func normalizeWindow(from, to time.Time) AnalysisWindow {
	to = to.UTC()
	if to.IsZero() {
		to = time.Now().UTC()
	}
	from = from.UTC()
	if from.IsZero() || !from.Before(to) {
		from = to.AddDate(0, 0, -AnalysisWindowDays)
	}
	return AnalysisWindow{
		From:       from,
		To:         to,
		Days:       int(math.Ceil(to.Sub(from).Hours() / 24)),
		PublicOnly: true,
	}
}

func analyzeContributions(snapshot ContributionSnapshot) ContributionAnalysis {
	if !snapshot.Available {
		unavailable := CountMetric{Status: EvidenceUnavailable}
		return ContributionAnalysis{
			WindowDays:          AnalysisWindowDays,
			Commits:             unavailable,
			IssuesOpened:        unavailable,
			PullRequestsOpened:  unavailable,
			PullRequestReviews:  unavailable,
			RepositoriesTouched: unavailable,
		}
	}
	return ContributionAnalysis{
		WindowDays:          AnalysisWindowDays,
		Commits:             countMetric(snapshot.Commits),
		IssuesOpened:        countMetric(snapshot.IssuesOpened),
		PullRequestsOpened:  countMetric(snapshot.PullRequestsOpened),
		PullRequestReviews:  countMetric(snapshot.PullRequestReviews),
		RepositoriesTouched: countMetric(snapshot.RepositoriesTouched),
	}
}

func countMetric(evidence CountEvidence) CountMetric {
	if !evidence.Available {
		return CountMetric{Status: EvidenceUnavailable}
	}
	status := EvidenceSampled
	if evidence.Complete {
		status = EvidenceExact
	}
	return CountMetric{Value: max(0, evidence.Value), Status: status}
}

func analyzeRepositoryCollection(
	collection RepositoryCollection,
	windowStart time.Time,
) RepositorySample {
	if !collection.Available {
		return RepositorySample{
			Status:              EvidenceUnavailable,
			PrimaryTechnologies: []LanguageShare{},
		}
	}
	status := EvidenceExact
	if collection.HasMore ||
		!collection.TotalKnown ||
		(collection.TotalKnown &&
			collection.Total > len(collection.Repositories)) {
		status = EvidenceSampled
	}
	var total *int
	if collection.TotalKnown {
		value := max(0, collection.Total)
		total = &value
	}
	active := 0
	for _, observation := range collection.Repositories {
		if !repositoryLastUsed(observation.Repository).Before(windowStart) {
			active++
		}
	}
	return RepositorySample{
		Status:         status,
		Observed:       len(collection.Repositories),
		Total:          total,
		Limit:          collection.Limit,
		ActiveInWindow: active,
		PrimaryTechnologies: topLanguageShares(
			AggregateLanguages(collectionLanguageBytes(collection)),
			maximumLanguageResults,
		),
	}
}

func collectionLanguageBytes(
	collection RepositoryCollection,
) []map[string]int64 {
	result := make([]map[string]int64, 0, len(collection.Repositories))
	for _, observation := range collection.Repositories {
		if len(observation.Languages) > 0 {
			result = append(result, observation.Languages)
			continue
		}
		if language := strings.TrimSpace(
			observation.Repository.MainLanguage,
		); language != "" {
			result = append(result, map[string]int64{language: 1})
		}
	}
	return result
}

func topLanguageShares(
	languages []LanguageShare,
	limit int,
) []LanguageShare {
	if limit <= 0 || len(languages) == 0 {
		return []LanguageShare{}
	}
	if len(languages) <= limit {
		return slices.Clone(languages)
	}
	kept := slices.Clone(languages[:limit-1])
	other := 0
	for _, language := range languages[limit-1:] {
		other += language.Percentage
	}
	return append(kept, LanguageShare{Name: "Other", Percentage: other})
}

func repositoryLastUsed(item repository.Summary) time.Time {
	return maxTime(item.PushedAt, item.UpdatedAt).UTC()
}
