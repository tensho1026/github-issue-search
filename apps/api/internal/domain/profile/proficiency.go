package profile

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

type technologyAccumulator struct {
	name             string
	kind             TechnologyKind
	lastUsedAt       time.Time
	repositories     map[string]struct{}
	sources          map[RepositorySource]struct{}
	ownedCount       int
	contributedCount int
	forkedCount      int
	recentCount      int
	frameworkCount   int
	languageShare    int
}

func analyzeRecentTechnologies(
	snapshot ProfileSnapshot,
	windowStart time.Time,
) []RecentTechnology {
	accumulators := make(map[string]*technologyAccumulator)
	accumulateCollectionTechnologies(
		accumulators,
		snapshot.Owned,
		RepositoryOwned,
		windowStart,
		true,
	)
	accumulateCollectionTechnologies(
		accumulators,
		snapshot.Contributed,
		RepositoryContributed,
		windowStart,
		false,
	)
	accumulateCollectionTechnologies(
		accumulators,
		snapshot.Forked,
		RepositoryForked,
		windowStart,
		false,
	)

	result := make([]RecentTechnology, 0, len(accumulators))
	for _, accumulator := range accumulators {
		sources := make([]RepositorySource, 0, len(accumulator.sources))
		for source := range accumulator.sources {
			sources = append(sources, source)
		}
		slices.Sort(sources)
		confidence := confidenceForEvidence(len(accumulator.repositories), false)
		result = append(result, RecentTechnology{
			Name:              accumulator.name,
			Kind:              accumulator.kind,
			LastUsedAt:        accumulator.lastUsedAt,
			RepositoryCount:   len(accumulator.repositories),
			RepositorySources: sources,
			Confidence:        confidence,
		})
	}
	slices.SortFunc(result, func(left, right RecentTechnology) int {
		if order := right.LastUsedAt.Compare(left.LastUsedAt); order != 0 {
			return order
		}
		if order := cmp.Compare(
			right.RepositoryCount,
			left.RepositoryCount,
		); order != 0 {
			return order
		}
		return cmp.Compare(left.Name, right.Name)
	})
	if len(result) > MaximumTechnologyResults {
		result = result[:MaximumTechnologyResults]
	}
	return result
}

func accumulateCollectionTechnologies(
	accumulators map[string]*technologyAccumulator,
	collection RepositoryCollection,
	source RepositorySource,
	windowStart time.Time,
	includeFrameworks bool,
) {
	for _, observation := range collection.Repositories {
		if observation.Repository.IsArchived {
			continue
		}
		lastUsed := repositoryLastUsed(observation.Repository)
		if lastUsed.Before(windowStart) {
			continue
		}
		languages := observation.Languages
		if len(languages) == 0 &&
			strings.TrimSpace(observation.Repository.MainLanguage) != "" {
			languages = map[string]int64{
				observation.Repository.MainLanguage: 1,
			}
		}
		for language, count := range languages {
			if count <= 0 || strings.TrimSpace(language) == "" {
				continue
			}
			accumulateTechnology(
				accumulators,
				language,
				TechnologyLanguage,
				observation.Repository.FullName,
				source,
				lastUsed,
			)
		}
		if !includeFrameworks {
			continue
		}
		for _, framework := range InferFrameworks(observation.Manifests) {
			accumulateTechnology(
				accumulators,
				framework,
				TechnologyFramework,
				observation.Repository.FullName,
				source,
				lastUsed,
			)
		}
	}
}

func accumulateTechnology(
	accumulators map[string]*technologyAccumulator,
	name string,
	kind TechnologyKind,
	repositoryName string,
	source RepositorySource,
	lastUsed time.Time,
) {
	key := string(kind) + "\x00" + strings.ToLower(name)
	accumulator, exists := accumulators[key]
	if !exists {
		accumulator = &technologyAccumulator{
			name:         name,
			kind:         kind,
			repositories: make(map[string]struct{}),
			sources:      make(map[RepositorySource]struct{}),
		}
		accumulators[key] = accumulator
	}
	accumulator.repositories[repositoryName] = struct{}{}
	accumulator.sources[source] = struct{}{}
	if lastUsed.After(accumulator.lastUsedAt) {
		accumulator.lastUsedAt = lastUsed
	}
}

func collectionFrameworkEvidence(
	collection RepositoryCollection,
) map[string]int {
	counts := make(map[string]int)
	for _, observation := range collection.Repositories {
		for _, framework := range InferFrameworks(observation.Manifests) {
			counts[framework]++
		}
	}
	return counts
}

func sortedFrameworkNames(evidence map[string]int) []string {
	names := make([]string, 0, len(evidence))
	for name := range evidence {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func buildTechnologyProficiency(
	snapshot ProfileSnapshot,
	languages []LanguageShare,
	frameworkEvidence map[string]int,
	recent []RecentTechnology,
) []TechnologyProficiency {
	accumulators := make(map[string]*technologyAccumulator)
	for _, language := range languages {
		if language.Name == "Other" {
			continue
		}
		accumulator := ensureTechnologyAccumulator(
			accumulators,
			language.Name,
			TechnologyLanguage,
		)
		accumulator.languageShare = language.Percentage
	}
	accumulateProficiencyRepositories(
		accumulators,
		snapshot.Owned,
		RepositoryOwned,
	)
	accumulateProficiencyRepositories(
		accumulators,
		snapshot.Contributed,
		RepositoryContributed,
	)
	accumulateProficiencyRepositories(
		accumulators,
		snapshot.Forked,
		RepositoryForked,
	)
	for name, count := range frameworkEvidence {
		accumulator := ensureTechnologyAccumulator(
			accumulators,
			name,
			TechnologyFramework,
		)
		accumulator.frameworkCount = count
		accumulator.ownedCount = max(accumulator.ownedCount, count)
	}
	for _, technology := range recent {
		accumulator := ensureTechnologyAccumulator(
			accumulators,
			technology.Name,
			technology.Kind,
		)
		accumulator.recentCount = technology.RepositoryCount
	}

	result := make([]TechnologyProficiency, 0, len(accumulators))
	incomplete := collectionIncomplete(snapshot.Owned) ||
		collectionIncomplete(snapshot.Contributed) ||
		collectionIncomplete(snapshot.Forked)
	for _, accumulator := range accumulators {
		score := proficiencyScore(*accumulator)
		level, label := proficiencyLevel(score)
		evidence := proficiencyEvidence(*accumulator, snapshot)
		result = append(result, TechnologyProficiency{
			Name:  accumulator.name,
			Kind:  accumulator.kind,
			Level: level,
			Label: label,
			Score: score,
			Confidence: confidenceForEvidence(
				accumulator.ownedCount+
					accumulator.contributedCount+
					accumulator.forkedCount,
				incomplete,
			),
			Evidence: evidence,
		})
	}
	slices.SortFunc(result, func(left, right TechnologyProficiency) int {
		if order := cmp.Compare(right.Score, left.Score); order != 0 {
			return order
		}
		if order := cmp.Compare(left.Kind, right.Kind); order != 0 {
			return order
		}
		return cmp.Compare(left.Name, right.Name)
	})
	if len(result) > MaximumTechnologyResults {
		result = result[:MaximumTechnologyResults]
	}
	return result
}

func ensureTechnologyAccumulator(
	accumulators map[string]*technologyAccumulator,
	name string,
	kind TechnologyKind,
) *technologyAccumulator {
	key := string(kind) + "\x00" + strings.ToLower(name)
	accumulator, exists := accumulators[key]
	if !exists {
		accumulator = &technologyAccumulator{name: name, kind: kind}
		accumulators[key] = accumulator
	}
	return accumulator
}

func accumulateProficiencyRepositories(
	accumulators map[string]*technologyAccumulator,
	collection RepositoryCollection,
	source RepositorySource,
) {
	for _, observation := range collection.Repositories {
		languages := observation.Languages
		if len(languages) == 0 &&
			strings.TrimSpace(observation.Repository.MainLanguage) != "" {
			languages = map[string]int64{
				observation.Repository.MainLanguage: 1,
			}
		}
		for language, bytes := range languages {
			if bytes <= 0 {
				continue
			}
			accumulator := ensureTechnologyAccumulator(
				accumulators,
				language,
				TechnologyLanguage,
			)
			switch source {
			case RepositoryOwned:
				accumulator.ownedCount++
			case RepositoryContributed:
				accumulator.contributedCount++
			case RepositoryForked:
				accumulator.forkedCount++
			}
		}
	}
}

func proficiencyScore(evidence technologyAccumulator) int {
	score := min(35, int(mathRound(float64(evidence.languageShare)*0.35)))
	score += min(20, evidence.ownedCount*5)
	score += min(20, evidence.contributedCount*5)
	score += min(15, evidence.recentCount*3)
	score += min(10, evidence.frameworkCount*5)
	score += min(5, evidence.forkedCount)
	return min(100, score)
}

func mathRound(value float64) float64 {
	if value < 0 {
		return float64(int(value - 0.5))
	}
	return float64(int(value + 0.5))
}

func proficiencyLevel(score int) (int, string) {
	switch {
	case score >= 80:
		return 5, "expert"
	case score >= 60:
		return 4, "advanced"
	case score >= 40:
		return 3, "intermediate"
	case score >= 20:
		return 2, "developing"
	default:
		return 1, "exploring"
	}
}

func proficiencyEvidence(
	accumulator technologyAccumulator,
	snapshot ProfileSnapshot,
) []TechnologyEvidence {
	return []TechnologyEvidence{
		{
			Kind:   "language_share_percentage",
			Value:  accumulator.languageShare,
			Status: languageEvidenceStatus(snapshot.Owned),
		},
		{
			Kind:   "owned_repositories",
			Value:  accumulator.ownedCount,
			Status: collectionStatus(snapshot.Owned),
		},
		{
			Kind:   "contributed_repositories",
			Value:  accumulator.contributedCount,
			Status: collectionStatus(snapshot.Contributed),
		},
		{
			Kind:   "forked_repositories",
			Value:  accumulator.forkedCount,
			Status: collectionStatus(snapshot.Forked),
		},
		{
			Kind:   "recent_repositories",
			Value:  accumulator.recentCount,
			Status: recentEvidenceStatus(snapshot),
		},
		{
			Kind:   "framework_manifests",
			Value:  accumulator.frameworkCount,
			Status: collectionStatus(snapshot.Owned),
		},
	}
}

func collectionStatus(collection RepositoryCollection) EvidenceStatus {
	if !collection.Available {
		return EvidenceUnavailable
	}
	if collectionIncomplete(collection) {
		return EvidenceSampled
	}
	return EvidenceExact
}

func languageEvidenceStatus(
	collection RepositoryCollection,
) EvidenceStatus {
	if !collection.Available {
		return EvidenceUnavailable
	}
	if collectionIncomplete(collection) {
		return EvidenceSampled
	}
	for _, observation := range collection.Repositories {
		if !observation.LanguagesComplete {
			return EvidenceSampled
		}
	}
	return EvidenceExact
}

func collectionIncomplete(collection RepositoryCollection) bool {
	return !collection.Available ||
		collection.HasMore ||
		!collection.TotalKnown ||
		(collection.TotalKnown &&
			collection.Total > len(collection.Repositories))
}

func recentEvidenceStatus(snapshot ProfileSnapshot) EvidenceStatus {
	if !snapshot.Owned.Available &&
		!snapshot.Contributed.Available &&
		!snapshot.Forked.Available {
		return EvidenceUnavailable
	}
	if collectionIncomplete(snapshot.Owned) ||
		collectionIncomplete(snapshot.Contributed) ||
		collectionIncomplete(snapshot.Forked) {
		return EvidenceSampled
	}
	return EvidenceExact
}

func confidenceForEvidence(count int, incomplete bool) Confidence {
	switch {
	case count >= 5 && !incomplete:
		return ConfidenceHigh
	case count >= 2:
		return ConfidenceMedium
	case count >= 1:
		return ConfidenceLow
	default:
		return ConfidenceUnavailable
	}
}

func analyzeOSSExperience(
	contributions ContributionAnalysis,
) OSSExperience {
	if contributions.PullRequestsOpened.Status == EvidenceUnavailable &&
		contributions.RepositoriesTouched.Status == EvidenceUnavailable {
		return OSSExperience{
			Level:      "unavailable",
			Confidence: ConfidenceUnavailable,
			PublicOnly: true,
			Evidence:   []TechnologyEvidence{},
		}
	}
	score := min(40, contributions.PullRequestsOpened.Value*4)
	score += min(30, contributions.RepositoriesTouched.Value*5)
	score += min(20, contributions.PullRequestReviews.Value*2)
	score += min(10, contributions.IssuesOpened.Value)

	level := "no_public_evidence"
	switch {
	case score >= 70:
		level = "sustained"
	case score >= 40:
		level = "active"
	case score >= 15:
		level = "contributing"
	case score > 0:
		level = "emerging"
	}
	confidence := ConfidenceLow
	if contributions.PullRequestsOpened.Status == EvidenceExact &&
		contributions.RepositoriesTouched.Status != EvidenceUnavailable &&
		score >= 40 {
		confidence = ConfidenceHigh
	} else if score > 0 {
		confidence = ConfidenceMedium
	}
	return OSSExperience{
		Level:      level,
		Confidence: confidence,
		PublicOnly: true,
		Evidence: []TechnologyEvidence{
			{
				Kind:   "authored_pull_requests",
				Value:  contributions.PullRequestsOpened.Value,
				Status: contributions.PullRequestsOpened.Status,
			},
			{
				Kind:   "contributed_repositories",
				Value:  contributions.RepositoriesTouched.Value,
				Status: contributions.RepositoriesTouched.Status,
			},
			{
				Kind:   "pull_request_reviews",
				Value:  contributions.PullRequestReviews.Value,
				Status: contributions.PullRequestReviews.Status,
			},
		},
	}
}
