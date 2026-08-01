package issue

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

// GitHub candidate constants normalize upstream state and actor type values.
const (
	StateOpen   = "open"
	AuthorBot   = "Bot"
	AuthorHuman = "User"
)

// Candidate is normalized GitHub issue and repository data evaluated during
// bounded discovery. It deliberately excludes transport- and GitHub-specific
// response types.
type Candidate struct {
	Repository repository.Summary
	Issue      Summary
}

// Summary is the issue data needed for eligibility filtering and the search
// result list. Full detail analysis is intentionally deferred to a later flow.
type Summary struct {
	Number        int
	Title         string
	Body          string
	URL           string
	State         string
	Labels        []string
	Assignees     []string
	AuthorLogin   string
	AuthorType    string
	Comments      int
	IsPullRequest bool
	Locked        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ExclusionReason is a stable, observable reason that a GitHub candidate did
// not satisfy the validated IssueScout search criteria.
type ExclusionReason string

// ExclusionReason values are stable diagnostics returned in search metadata.
const (
	ExclusionPullRequest             ExclusionReason = "pull_request"
	ExclusionNotOpen                 ExclusionReason = "not_open"
	ExclusionArchivedRepository      ExclusionReason = "archived_repository"
	ExclusionAlreadyAssigned         ExclusionReason = "already_assigned"
	ExclusionStale                   ExclusionReason = "stale"
	ExclusionBotGenerated            ExclusionReason = "bot_generated"
	ExclusionSensitiveContent        ExclusionReason = "suspected_sensitive_content"
	ExclusionInsufficientDescription ExclusionReason = "insufficient_description"
	ExclusionBelowMinimumStars       ExclusionReason = "below_minimum_stars"
	ExclusionLanguageMismatch        ExclusionReason = "language_mismatch"
	ExclusionFrameworkMismatch       ExclusionReason = "framework_mismatch"
	ExclusionLabelMismatch           ExclusionReason = "label_mismatch"
	ExclusionAboveMaximumDifficulty  ExclusionReason = "above_maximum_difficulty"
	ExclusionEnglishNotAllowed       ExclusionReason = "english_not_allowed"
)

var sensitiveContentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(
		`(?i)\b(password|passwd|secret|access[_ -]?token)\s*[:=]\s*["']?[^\s"']{8,}`,
	),
}

// ExclusionReasons applies deterministic, inexpensive eligibility checks to a
// candidate. The returned order is stable for logs, tests, and API metadata.
func ExclusionReasons(
	criteria SearchCriteria,
	candidate Candidate,
	now time.Time,
) []ExclusionReason {
	reasons := make([]ExclusionReason, 0)
	if candidate.Issue.IsPullRequest {
		reasons = append(reasons, ExclusionPullRequest)
	}
	if !strings.EqualFold(candidate.Issue.State, StateOpen) {
		reasons = append(reasons, ExclusionNotOpen)
	}
	if criteria.ExcludesArchived() && candidate.Repository.IsArchived {
		reasons = append(reasons, ExclusionArchivedRepository)
	}
	if len(candidate.Issue.Assignees) > 0 {
		reasons = append(reasons, ExclusionAlreadyAssigned)
	}

	cutoff := now.UTC().AddDate(0, 0, -criteria.UpdatedWithinDays())
	if candidate.Issue.UpdatedAt.Before(cutoff) ||
		candidate.Repository.UpdatedAt.Before(cutoff) {
		reasons = append(reasons, ExclusionStale)
	}
	if IsBotGenerated(candidate.Issue) {
		reasons = append(reasons, ExclusionBotGenerated)
	}
	if ContainsSensitiveContent(candidate.Issue.Title, candidate.Issue.Body) {
		reasons = append(reasons, ExclusionSensitiveContent)
	}
	if HasInsufficientDescription(candidate.Issue.Body) {
		reasons = append(reasons, ExclusionInsufficientDescription)
	}
	if candidate.Repository.Stars < criteria.MinimumStars() {
		reasons = append(reasons, ExclusionBelowMinimumStars)
	}
	if !matchesAnyValue(
		candidate.Repository.MainLanguage,
		criteria.Languages(),
	) {
		reasons = append(reasons, ExclusionLanguageMismatch)
	}
	if !matchesFrameworks(candidate, criteria.Frameworks()) {
		reasons = append(reasons, ExclusionFrameworkMismatch)
	}
	if !matchesAnyLabel(candidate.Issue.Labels, criteria.Labels()) {
		reasons = append(reasons, ExclusionLabelMismatch)
	}
	if EstimateDifficulty(candidate.Issue.Labels) >
		criteria.MaximumDifficulty() {
		reasons = append(reasons, ExclusionAboveMaximumDifficulty)
	}
	if !criteria.IncludesEnglish() &&
		IsProbablyEnglish(candidate.Issue.Title+" "+candidate.Issue.Body) {
		reasons = append(reasons, ExclusionEnglishNotAllowed)
	}

	return reasons
}

// EstimateDifficulty derives a conservative preliminary difficulty solely
// from explicit labels. Issue #6 replaces this discovery-time estimate with
// full rule-based content and effort analysis.
func EstimateDifficulty(labels []string) Difficulty {
	var estimated Difficulty
	for _, label := range labels {
		normalized := normalizeLabel(label)
		value := labeledDifficulty(normalized)
		if value > estimated {
			estimated = value
		}
	}
	if estimated == 0 {
		return Difficulty(DefaultMaximumDifficulty)
	}
	return estimated
}

func labeledDifficulty(label string) Difficulty {
	switch label {
	case "good first issue", "first timers only", "first-timers-only":
		return 1
	case "beginner", "starter", "easy", "documentation":
		return 2
	case "intermediate", "medium":
		return 3
	case "hard", "complex":
		return 4
	case "advanced", "expert":
		return 5
	}

	for _, prefix := range []string{"difficulty ", "difficulty:", "difficulty/"} {
		raw, found := strings.CutPrefix(label, prefix)
		if !found {
			continue
		}
		if numeric, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil &&
			numeric >= 1 &&
			numeric <= 5 {
			return Difficulty(numeric)
		}
		return labeledDifficulty(strings.TrimSpace(raw))
	}
	return 0
}

// IsBotGenerated detects GitHub bot actors and common dependency automation
// identities without relying only on a display-name convention.
func IsBotGenerated(summary Summary) bool {
	login := strings.ToLower(strings.TrimSpace(summary.AuthorLogin))
	authorType := strings.ToLower(strings.TrimSpace(summary.AuthorType))
	if authorType == "bot" ||
		strings.HasSuffix(login, "[bot]") ||
		login == "dependabot" ||
		login == "renovate" ||
		strings.HasPrefix(login, "dependabot-") ||
		strings.HasPrefix(login, "renovate-") {
		return true
	}

	title := strings.ToLower(strings.TrimSpace(summary.Title))
	return strings.HasPrefix(title, "chore(deps)") ||
		strings.HasPrefix(title, "chore(deps-dev)") ||
		strings.HasPrefix(title, "[dependabot]") ||
		strings.HasPrefix(title, "[renovate]")
}

// ContainsSensitiveContent identifies credential-shaped text that should not
// be surfaced as a contribution recommendation.
func ContainsSensitiveContent(title, body string) bool {
	content := title + "\n" + body
	for _, pattern := range sensitiveContentPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// HasInsufficientDescription rejects nearly empty issues while supporting
// languages that do not separate every word with spaces.
func HasInsufficientDescription(body string) bool {
	meaningfulRunes := 0
	wordCount := 0
	insideWord := false
	for _, character := range body {
		isWord := unicode.IsLetter(character) || unicode.IsDigit(character)
		if isWord {
			meaningfulRunes++
			if !insideWord {
				wordCount++
			}
		}
		insideWord = isWord
	}
	return meaningfulRunes < 40 && wordCount < 8
}

// IsProbablyEnglish uses a deterministic script heuristic. It intentionally
// treats predominantly Latin prose as English-compatible because discovery
// cannot justify an extra upstream language-detection service.
func IsProbablyEnglish(value string) bool {
	latinLetters := 0
	otherLetters := 0
	for _, character := range value {
		if !unicode.IsLetter(character) {
			continue
		}
		if unicode.In(character, unicode.Latin) {
			latinLetters++
		} else {
			otherLetters++
		}
	}
	return latinLetters >= 12 && latinLetters >= otherLetters*3
}

func matchesAnyValue(value string, accepted []FilterValue) bool {
	if len(accepted) == 0 {
		return true
	}
	for _, candidate := range accepted {
		if strings.EqualFold(strings.TrimSpace(value), candidate.String()) {
			return true
		}
	}
	return false
}

func matchesAnyLabel(labels []string, accepted []FilterValue) bool {
	if len(accepted) == 0 {
		return true
	}
	for _, label := range labels {
		if matchesAnyValue(label, accepted) {
			return true
		}
	}
	return false
}

func matchesFrameworks(
	candidate Candidate,
	frameworks []FilterValue,
) bool {
	if len(frameworks) == 0 {
		return true
	}
	searchable := strings.Join(
		append(
			[]string{
				candidate.Issue.Title,
				candidate.Issue.Body,
				candidate.Repository.Name,
				candidate.Repository.Description,
			},
			candidate.Issue.Labels...,
		),
		" ",
	)
	for _, framework := range frameworks {
		if containsTerm(searchable, framework.String()) {
			return true
		}
	}
	return false
}

func containsTerm(content, term string) bool {
	contentRunes := []rune(strings.ToLower(content))
	termRunes := []rune(strings.ToLower(strings.TrimSpace(term)))
	if len(termRunes) == 0 || len(termRunes) > len(contentRunes) {
		return false
	}

	for index := 0; index <= len(contentRunes)-len(termRunes); index++ {
		if !runesEqual(contentRunes[index:index+len(termRunes)], termRunes) {
			continue
		}
		beforeBoundary := index == 0 ||
			!isTermCharacter(contentRunes[index-1])
		after := index + len(termRunes)
		afterBoundary := after == len(contentRunes) ||
			!isTermCharacter(contentRunes[after])
		if beforeBoundary && afterBoundary {
			return true
		}
	}
	return false
}

func runesEqual(left, right []rune) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isTermCharacter(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsDigit(character)
}

func normalizeLabel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", " ")
	return strings.Join(strings.Fields(normalized), " ")
}
