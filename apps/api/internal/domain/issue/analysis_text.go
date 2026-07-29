package issue

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type normalizedAnalysisInput struct {
	title                 string
	body                  string
	combined              string
	labels                []string
	repositoryLanguage    string
	dependencies          []string
	commentCount          int
	hasMaintainerGuidance bool
	bodyRuneCount         int
	bodyWasTruncated      bool
}

func normalizeAnalysisInput(input AnalysisInput) normalizedAnalysisInput {
	title, titleTruncated := normalizeAnalysisText(
		input.Candidate.Issue.Title,
		MaximumAnalysisTextBytes/4,
	)
	body, bodyTruncated := normalizeAnalysisText(
		input.Candidate.Issue.Body,
		MaximumAnalysisTextBytes,
	)
	labels := normalizeAnalysisValues(
		input.Candidate.Issue.Labels,
		MaximumAnalysisLabels,
	)
	dependencies := normalizeAnalysisValues(
		input.Dependencies,
		MaximumAnalysisDependencies,
	)
	repositoryLanguage, _ := normalizeAnalysisText(
		input.Candidate.Repository.MainLanguage,
		128,
	)

	return normalizedAnalysisInput{
		title:                 title,
		body:                  body,
		combined:              strings.TrimSpace(title + "\n" + body),
		labels:                labels,
		repositoryLanguage:    repositoryLanguage,
		dependencies:          dependencies,
		commentCount:          max(input.Candidate.Issue.Comments, 0),
		hasMaintainerGuidance: input.HasMaintainerGuidance,
		bodyRuneCount:         utf8.RuneCountInString(body),
		bodyWasTruncated:      titleTruncated || bodyTruncated,
	}
}

func normalizeAnalysisText(raw string, byteLimit int) (string, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if len(value) <= byteLimit {
		return value, false
	}
	value = value[:byteLimit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value), true
}

func normalizeAnalysisValues(values []string, limit int) []string {
	if limit <= 0 || len(values) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, min(len(values), limit))
	seen := make(map[string]struct{}, min(len(values), limit))
	for _, value := range values {
		if len(normalized) == limit {
			break
		}
		item, _ := normalizeAnalysisText(value, 256)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	slices.Sort(normalized)
	return normalized
}

func containsAnyTerm(text string, terms ...string) bool {
	for _, term := range terms {
		if containsNormalizedTerm(text, term) {
			return true
		}
	}
	return false
}

func containsNormalizedTerm(text string, rawTerm string) bool {
	term := strings.ToLower(strings.TrimSpace(rawTerm))
	if text == "" || term == "" {
		return false
	}
	if containsNonASCII(term) {
		return strings.Contains(text, term)
	}
	for offset := 0; offset < len(text); {
		index := strings.Index(text[offset:], term)
		if index < 0 {
			return false
		}
		index += offset
		beforeOK := index == 0 ||
			!isNormalizedTermCharacter(decodeRuneBefore(text, index))
		afterIndex := index + len(term)
		afterOK := afterIndex == len(text) ||
			!isNormalizedTermCharacter(decodeRuneAt(text, afterIndex))
		if beforeOK && afterOK {
			return true
		}
		offset = index + 1
	}
	return false
}

func containsNonASCII(value string) bool {
	for _, character := range value {
		if character > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func decodeRuneBefore(value string, index int) rune {
	character, _ := utf8.DecodeLastRuneInString(value[:index])
	return character
}

func decodeRuneAt(value string, index int) rune {
	character, _ := utf8.DecodeRuneInString(value[index:])
	return character
}

func isNormalizedTermCharacter(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsDigit(character) ||
		character == '_'
}

func hasAnyLabel(input normalizedAnalysisInput, labels ...string) bool {
	for _, actual := range input.labels {
		for _, expected := range labels {
			if actual == expected {
				return true
			}
		}
	}
	return false
}
