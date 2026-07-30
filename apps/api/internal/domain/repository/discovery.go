package repository

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaximumREADMEAnalysisBytes = 64 << 10

// EvidenceStatus communicates whether a discovery signal is complete,
// sampled, or unavailable. Zero is meaningful only when status is not
// unavailable.
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

type ReadinessBand string

const (
	ReadinessNeedsWork ReadinessBand = "needs_work"
	ReadinessPromising ReadinessBand = "promising"
	ReadinessReady     ReadinessBand = "ready"
)

type DiscoveryWarning string

const (
	WarningEnrichmentUnavailable DiscoveryWarning = "enrichment_unavailable"
	WarningREADMEContentSampled  DiscoveryWarning = "readme_content_sampled"
)

// DiscoveryCandidate is the bounded normalized repository search result before
// README and contribution-file enrichment.
type DiscoveryCandidate struct {
	Repository        Summary
	Topics            []string
	License           SPDXLicense
	LicenseName       string
	LicenseKnown      bool
	Watchers          int
	GoodFirstIssues   int
	HelpWantedIssues  int
	HasIssuesEnabled  bool
	HasDiscussions    bool
	HasCodeOfConduct  bool
	HasSecurityPolicy bool
}

// DiscoveryEnrichment is optional public documentation evidence for one
// shortlist repository.
type DiscoveryEnrichment struct {
	Available              bool
	READMEAvailable        bool
	READMEContentAvailable bool
	READMEText             string
	READMEContentSampled   bool
	ContributingAvailable  bool
	GoodFirstIssues        int
	HelpWantedIssues       int
	HasCodeOfConduct       bool
	HasSecurityPolicy      bool
}

type JapaneseREADMEEvidence struct {
	Detected      bool
	Status        EvidenceStatus
	Confidence    Confidence
	JapaneseRunes int
	LetterRunes   int
	SampledBytes  int
}

type DocumentationSignals struct {
	READMEAvailable       bool
	ContributingAvailable bool
	CodeOfConduct         bool
	SecurityPolicy        bool
	JapaneseREADME        JapaneseREADMEEvidence
	Status                EvidenceStatus
}

type PreliminaryDifficulty struct {
	Level   int
	Label   string
	Reasons []string
}

type ContributionReadiness struct {
	Score   int
	Band    ReadinessBand
	Reasons []string
}

// DiscoveryResult is the explainable, normalized repository returned by the
// application after bounded enrichment and deterministic analysis.
type DiscoveryResult struct {
	Repository       Summary
	Topics           []string
	License          SPDXLicense
	LicenseName      string
	LicenseKnown     bool
	Watchers         int
	GoodFirstIssues  int
	HelpWantedIssues int
	HasIssuesEnabled bool
	HasDiscussions   bool
	Category         Category
	Technologies     []string
	Documentation    DocumentationSignals
	Difficulty       PreliminaryDifficulty
	Readiness        ContributionReadiness
	Warnings         []DiscoveryWarning
}

// AnalyzeDiscovery turns a shortlist candidate into stable rule-based
// evidence. The rules are deliberately conservative and do not claim
// maintainer intent or task complexity.
func AnalyzeDiscovery(
	candidate DiscoveryCandidate,
	enrichment DiscoveryEnrichment,
	requestedTechnologies []FilterValue,
	now time.Time,
) DiscoveryResult {
	enrichment.READMEText, enrichment.READMEContentSampled =
		boundREADMEContent(
			enrichment.READMEText,
			enrichment.READMEContentSampled,
		)
	documentation := analyzeDocumentation(candidate, enrichment)
	category := ClassifyDiscoveryCategory(candidate)
	technologies := detectRequestedTechnologies(
		candidate.Topics,
		enrichment.READMEText,
		requestedTechnologies,
	)
	difficulty := preliminaryDifficulty(
		candidate,
		documentation,
	)
	readiness := contributionReadiness(
		candidate,
		documentation,
		now.UTC(),
	)

	warnings := make([]DiscoveryWarning, 0, 2)
	if !enrichment.Available {
		warnings = append(warnings, WarningEnrichmentUnavailable)
	}
	if enrichment.READMEContentSampled {
		warnings = append(warnings, WarningREADMEContentSampled)
	}

	return DiscoveryResult{
		Repository:       candidate.Repository,
		Topics:           slices.Clone(candidate.Topics),
		License:          candidate.License,
		LicenseName:      candidate.LicenseName,
		LicenseKnown:     candidate.LicenseKnown,
		Watchers:         candidate.Watchers,
		GoodFirstIssues:  candidate.GoodFirstIssues,
		HelpWantedIssues: candidate.HelpWantedIssues,
		HasIssuesEnabled: candidate.HasIssuesEnabled,
		HasDiscussions:   candidate.HasDiscussions,
		Category:         category,
		Technologies:     technologies,
		Documentation:    documentation,
		Difficulty:       difficulty,
		Readiness:        readiness,
		Warnings:         warnings,
	}
}

func analyzeDocumentation(
	candidate DiscoveryCandidate,
	enrichment DiscoveryEnrichment,
) DocumentationSignals {
	status := EvidenceExact
	if !enrichment.Available {
		status = EvidenceUnavailable
	}
	if enrichment.READMEContentSampled {
		status = EvidenceSampled
	}
	return DocumentationSignals{
		READMEAvailable:       enrichment.READMEAvailable,
		ContributingAvailable: enrichment.ContributingAvailable,
		CodeOfConduct:         candidate.HasCodeOfConduct,
		SecurityPolicy:        candidate.HasSecurityPolicy,
		JapaneseREADME: detectJapaneseREADME(
			enrichment,
		),
		Status: status,
	}
}

func detectJapaneseREADME(
	enrichment DiscoveryEnrichment,
) JapaneseREADMEEvidence {
	if !enrichment.Available {
		return JapaneseREADMEEvidence{
			Status:     EvidenceUnavailable,
			Confidence: ConfidenceUnavailable,
		}
	}
	status := EvidenceExact
	if enrichment.READMEContentSampled {
		status = EvidenceSampled
	}
	if !enrichment.READMEAvailable {
		return JapaneseREADMEEvidence{
			Status:     status,
			Confidence: ConfidenceHigh,
		}
	}
	if !enrichment.READMEContentAvailable {
		return JapaneseREADMEEvidence{
			Status:     EvidenceUnavailable,
			Confidence: ConfidenceUnavailable,
		}
	}

	japaneseRunes := 0
	letterRunes := 0
	for _, character := range enrichment.READMEText {
		if unicode.IsLetter(character) {
			letterRunes++
		}
		if unicode.In(
			character,
			unicode.Han,
			unicode.Hiragana,
			unicode.Katakana,
		) {
			japaneseRunes++
		}
	}
	ratio := 0.0
	if letterRunes > 0 {
		ratio = float64(japaneseRunes) / float64(letterRunes)
	}
	detected := japaneseRunes >= 20 && ratio >= 0.05
	confidence := ConfidenceLow
	switch {
	case !enrichment.READMEContentSampled &&
		japaneseRunes >= 100 &&
		ratio >= 0.20:
		confidence = ConfidenceHigh
	case japaneseRunes >= 40 && ratio >= 0.10:
		confidence = ConfidenceMedium
	case !detected && japaneseRunes < 10:
		confidence = ConfidenceMedium
	}
	return JapaneseREADMEEvidence{
		Detected:      detected,
		Status:        status,
		Confidence:    confidence,
		JapaneseRunes: japaneseRunes,
		LetterRunes:   letterRunes,
		SampledBytes:  len(enrichment.READMEText),
	}
}

// ClassifyDiscoveryCategory maps bounded topics and description evidence to
// one deterministic preliminary OSS category.
func ClassifyDiscoveryCategory(candidate DiscoveryCandidate) Category {
	corpus := strings.ToLower(strings.Join(append(
		slices.Clone(candidate.Topics),
		candidate.Repository.Description,
	), " "))
	rules := []struct {
		category Category
		terms    []string
	}{
		{CategorySecurity, []string{
			"security", "cryptography", "authentication", "authorization",
		}},
		{CategoryData, []string{
			"machine-learning", "data-science", "database", "analytics", "ai",
		}},
		{CategoryInfrastructure, []string{
			"devops", "infrastructure", "kubernetes", "cloud", "terraform",
		}},
		{CategoryDocumentation, []string{
			"documentation", "docs", "reference",
		}},
		{CategoryEducation, []string{
			"education", "tutorial", "learning", "course",
		}},
		{CategoryFramework, []string{
			"framework", "web-framework", "application-framework",
		}},
		{CategoryLibrary, []string{
			"library", "sdk", "package", "component-library",
		}},
		{CategoryTooling, []string{
			"cli", "developer-tools", "tooling", "linter", "formatter",
		}},
	}
	for _, rule := range rules {
		for _, term := range rule.terms {
			if containsTerm(corpus, term) {
				return rule.category
			}
		}
	}
	return CategoryApplication
}

func detectRequestedTechnologies(
	topics []string,
	readme string,
	requested []FilterValue,
) []string {
	if len(requested) == 0 {
		return []string{}
	}
	topicCorpus := strings.ToLower(strings.Join(topics, " "))
	readmeCorpus := strings.ToLower(readme)
	detected := make([]string, 0, len(requested))
	for _, technology := range requested {
		term := strings.ToLower(technology.String())
		if containsTerm(topicCorpus, term) || containsTerm(readmeCorpus, term) {
			detected = append(detected, technology.String())
		}
	}
	return detected
}

func preliminaryDifficulty(
	candidate DiscoveryCandidate,
	documentation DocumentationSignals,
) PreliminaryDifficulty {
	level := 3
	reasons := make([]string, 0, 4)
	if candidate.Repository.IsArchived {
		return PreliminaryDifficulty{
			Level:   5,
			Label:   "very_high",
			Reasons: []string{"repository_archived"},
		}
	}
	if !documentation.READMEAvailable {
		level++
		reasons = append(reasons, "readme_unavailable")
	}
	if documentation.ContributingAvailable {
		level--
		reasons = append(reasons, "contributing_guide_available")
	}
	if candidate.GoodFirstIssues > 0 {
		level--
		reasons = append(reasons, "good_first_issues_available")
	}
	level = max(1, min(5, level))
	return PreliminaryDifficulty{
		Level:   level,
		Label:   difficultyLabel(level),
		Reasons: reasons,
	}
}

func contributionReadiness(
	candidate DiscoveryCandidate,
	documentation DocumentationSignals,
	now time.Time,
) ContributionReadiness {
	if candidate.Repository.IsArchived {
		return ContributionReadiness{
			Score:   0,
			Band:    ReadinessNeedsWork,
			Reasons: []string{"repository_archived"},
		}
	}
	score := 0
	reasons := make([]string, 0, 8)
	age := now.Sub(candidate.Repository.PushedAt.UTC())
	switch {
	case age <= 30*24*time.Hour:
		score += 25
		reasons = append(reasons, "pushed_within_30_days")
	case age <= 90*24*time.Hour:
		score += 18
		reasons = append(reasons, "pushed_within_90_days")
	case age <= 365*24*time.Hour:
		score += 8
		reasons = append(reasons, "pushed_within_365_days")
	}
	if documentation.READMEAvailable {
		score += 20
		reasons = append(reasons, "readme_available")
	}
	if documentation.ContributingAvailable {
		score += 20
		reasons = append(reasons, "contributing_guide_available")
	}
	if candidate.GoodFirstIssues > 0 {
		score += min(15, 5+candidate.GoodFirstIssues*2)
		reasons = append(reasons, "good_first_issues_available")
	}
	if candidate.HelpWantedIssues > 0 {
		score += min(10, 4+candidate.HelpWantedIssues)
		reasons = append(reasons, "help_wanted_issues_available")
	}
	if documentation.CodeOfConduct {
		score += 5
		reasons = append(reasons, "code_of_conduct_available")
	}
	if documentation.SecurityPolicy {
		score += 5
		reasons = append(reasons, "security_policy_available")
	}
	if candidate.HasDiscussions {
		score += 5
		reasons = append(reasons, "discussions_enabled")
	}
	score = min(100, score)
	band := ReadinessNeedsWork
	switch {
	case score >= 75:
		band = ReadinessReady
	case score >= 50:
		band = ReadinessPromising
	}
	return ContributionReadiness{Score: score, Band: band, Reasons: reasons}
}

func difficultyLabel(level int) string {
	switch level {
	case 1:
		return "very_low"
	case 2:
		return "low"
	case 3:
		return "medium"
	case 4:
		return "high"
	default:
		return "very_high"
	}
}

// SortDiscoveryResults applies the only public ordering: readiness, stars,
// freshness, then canonical repository name.
func SortDiscoveryResults(results []DiscoveryResult) {
	slices.SortStableFunc(results, func(left, right DiscoveryResult) int {
		if order := cmp.Compare(right.Readiness.Score, left.Readiness.Score); order != 0 {
			return order
		}
		if order := cmp.Compare(
			right.Repository.Stars,
			left.Repository.Stars,
		); order != 0 {
			return order
		}
		if order := right.Repository.PushedAt.Compare(
			left.Repository.PushedAt,
		); order != 0 {
			return order
		}
		return strings.Compare(
			strings.ToLower(left.Repository.FullName),
			strings.ToLower(right.Repository.FullName),
		)
	})
}

func containsTerm(corpus, term string) bool {
	if term == "" {
		return false
	}
	index := strings.Index(corpus, term)
	for index >= 0 {
		beforeBoundary := index == 0 ||
			!isTermRune(rune(corpus[index-1]))
		after := index + len(term)
		afterBoundary := after == len(corpus) ||
			!isTermRune(rune(corpus[after]))
		if beforeBoundary && afterBoundary {
			return true
		}
		next := strings.Index(corpus[index+1:], term)
		if next < 0 {
			return false
		}
		index += next + 1
	}
	return false
}

func isTermRune(character rune) bool {
	return unicode.IsLetter(character) ||
		unicode.IsDigit(character) ||
		character == '-' ||
		character == '_' ||
		character == '.'
}

func boundREADMEContent(content string, alreadySampled bool) (string, bool) {
	if len(content) <= MaximumREADMEAnalysisBytes {
		return content, alreadySampled
	}
	content = content[:MaximumREADMEAnalysisBytes]
	for !utf8.ValidString(content) {
		content = content[:len(content)-1]
	}
	return content, true
}
