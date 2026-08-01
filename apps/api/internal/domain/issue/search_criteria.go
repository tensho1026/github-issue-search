package issue

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

// Issue-search defaults and hard limits bound filtering, pagination, and
// upstream candidate work.
const (
	DefaultMinimumStars      = 10
	DefaultMaximumDifficulty = 3
	DefaultUpdatedWithinDays = 180
	DefaultPage              = 1
	DefaultPerPage           = 20

	MaximumFilterValues      = 10
	MaximumFilterValueRunes  = 64
	MaximumFilterValueBytes  = 128
	MaximumUpdatedWithinDays = 3650
	MaximumCandidateResults  = 50
	MaximumPageSize          = 50
)

// ErrInvalidSearchCriteria reports an unsafe or unsupported search option.
var ErrInvalidSearchCriteria = errors.New("invalid issue search criteria")

var defaultLabels = []string{"good first issue", "help wanted"}

// FilterValue is a validated, trimmed user-selectable search term.
//
// Quotation marks, backslashes, and control characters are rejected so a
// value can be quoted by the GitHub client without becoming a search
// qualifier. The client remains responsible for URL encoding the final query.
type FilterValue string

// ParseFilterValue validates a bounded GitHub search qualifier value.
func ParseFilterValue(raw string) (FilterValue, error) {
	value := strings.TrimSpace(raw)
	if value == "" ||
		utf8.RuneCountInString(value) > MaximumFilterValueRunes ||
		len(value) > MaximumFilterValueBytes {
		return "", fmt.Errorf(
			"%w: filter values must contain 1-%d characters and at most %d bytes",
			ErrInvalidSearchCriteria,
			MaximumFilterValueRunes,
			MaximumFilterValueBytes,
		)
	}

	for _, character := range value {
		if character == '"' || character == '\\' || unicode.IsControl(character) {
			return "", fmt.Errorf(
				"%w: filter values cannot contain quotes, backslashes, or control characters",
				ErrInvalidSearchCriteria,
			)
		}
	}

	return FilterValue(value), nil
}

// String returns the validated, trimmed filter value.
func (value FilterValue) String() string {
	return string(value)
}

// Difficulty is the bounded, five-level effort scale used by IssueScout.
type Difficulty uint8

// ParseDifficulty validates a difficulty level in the inclusive range 1-5.
func ParseDifficulty(value int) (Difficulty, error) {
	if value < 1 || value > 5 {
		return 0, fmt.Errorf(
			"%w: maximumDifficulty must be between 1 and 5",
			ErrInvalidSearchCriteria,
		)
	}
	return Difficulty(value), nil
}

// Int returns the validated numeric difficulty level.
func (difficulty Difficulty) Int() int {
	return int(difficulty)
}

// SearchCriteriaOptions contains transport-independent, optional issue search
// inputs. Nil scalar pointers receive the documented MVP defaults.
type SearchCriteriaOptions struct {
	Username             string
	Languages            []string
	Frameworks           []string
	Labels               []string
	MinimumStars         *int
	MaximumDifficulty    *int
	MaximumEffort        *string
	UpdatedWithinDays    *int
	IncludeDocumentation *bool
	IncludeEnglish       *bool
	ExcludeArchived      *bool
}

// SearchCriteria is an immutable, canonical issue discovery condition.
// Collection accessors return copies so cache keys cannot diverge after
// validation.
type SearchCriteria struct {
	username             user.Username
	languages            []FilterValue
	frameworks           []FilterValue
	labels               []FilterValue
	minimumStars         int
	maximumDifficulty    Difficulty
	maximumEffort        *EffortBand
	updatedWithinDays    int
	includeDocumentation bool
	includeEnglish       bool
	excludeArchived      bool
}

// NewSearchCriteria validates, defaults, deduplicates, and canonicalizes issue
// discovery options. Returned slice accessors never expose internal storage.
func NewSearchCriteria(options SearchCriteriaOptions) (SearchCriteria, error) {
	username, err := user.ParseUsername(options.Username)
	if err != nil {
		return SearchCriteria{}, fmt.Errorf(
			"%w: username is invalid",
			ErrInvalidSearchCriteria,
		)
	}

	languages, err := normalizeFilterValues("languages", options.Languages)
	if err != nil {
		return SearchCriteria{}, err
	}
	frameworks, err := normalizeFilterValues("frameworks", options.Frameworks)
	if err != nil {
		return SearchCriteria{}, err
	}

	includeDocumentation := valueOrDefault(options.IncludeDocumentation, false)
	rawLabels := append([]string(nil), options.Labels...)
	if len(rawLabels) == 0 {
		rawLabels = append(rawLabels, defaultLabels...)
	}
	if includeDocumentation && !containsFold(rawLabels, "documentation") {
		rawLabels = append(rawLabels, "documentation")
	}
	labels, err := normalizeFilterValues("labels", rawLabels)
	if err != nil {
		return SearchCriteria{}, err
	}

	minimumStars := intOrDefault(
		options.MinimumStars,
		DefaultMinimumStars,
	)
	if minimumStars < 0 {
		return SearchCriteria{}, fmt.Errorf(
			"%w: minimumStars cannot be negative",
			ErrInvalidSearchCriteria,
		)
	}

	maximumDifficultyValue := intOrDefault(
		options.MaximumDifficulty,
		DefaultMaximumDifficulty,
	)
	maximumDifficulty, err := ParseDifficulty(maximumDifficultyValue)
	if err != nil {
		return SearchCriteria{}, err
	}

	var maximumEffort *EffortBand
	if options.MaximumEffort != nil {
		parsed, parseErr := ParseEffortBand(*options.MaximumEffort)
		if parseErr != nil {
			return SearchCriteria{}, parseErr
		}
		maximumEffort = &parsed
	}

	updatedWithinDays := intOrDefault(
		options.UpdatedWithinDays,
		DefaultUpdatedWithinDays,
	)
	if updatedWithinDays < 1 ||
		updatedWithinDays > MaximumUpdatedWithinDays {
		return SearchCriteria{}, fmt.Errorf(
			"%w: updatedWithinDays must be between 1 and %d",
			ErrInvalidSearchCriteria,
			MaximumUpdatedWithinDays,
		)
	}

	return SearchCriteria{
		username:             username,
		languages:            languages,
		frameworks:           frameworks,
		labels:               labels,
		minimumStars:         minimumStars,
		maximumDifficulty:    maximumDifficulty,
		maximumEffort:        maximumEffort,
		updatedWithinDays:    updatedWithinDays,
		includeDocumentation: includeDocumentation,
		includeEnglish:       valueOrDefault(options.IncludeEnglish, true),
		excludeArchived:      valueOrDefault(options.ExcludeArchived, true),
	}, nil
}

// Username returns the validated GitHub login used for discovery.
func (criteria SearchCriteria) Username() user.Username {
	return criteria.username
}

// Languages returns a defensive copy of canonical language filters.
func (criteria SearchCriteria) Languages() []FilterValue {
	return slices.Clone(criteria.languages)
}

// Frameworks returns a defensive copy of canonical framework filters.
func (criteria SearchCriteria) Frameworks() []FilterValue {
	return slices.Clone(criteria.frameworks)
}

// Labels returns a defensive copy of canonical issue-label filters.
func (criteria SearchCriteria) Labels() []FilterValue {
	return slices.Clone(criteria.labels)
}

// MinimumStars returns the inclusive repository popularity threshold.
func (criteria SearchCriteria) MinimumStars() int {
	return criteria.minimumStars
}

// MaximumDifficulty returns the inclusive rule-derived difficulty ceiling.
func (criteria SearchCriteria) MaximumDifficulty() Difficulty {
	return criteria.maximumDifficulty
}

// MaximumEffort returns the optional inclusive effort ceiling.
func (criteria SearchCriteria) MaximumEffort() (EffortBand, bool) {
	if criteria.maximumEffort == nil {
		return "", false
	}
	return *criteria.maximumEffort, true
}

// UpdatedWithinDays returns the repository and issue freshness window.
func (criteria SearchCriteria) UpdatedWithinDays() int {
	return criteria.updatedWithinDays
}

// IncludesDocumentation reports whether documentation issues are eligible.
func (criteria SearchCriteria) IncludesDocumentation() bool {
	return criteria.includeDocumentation
}

// IncludesEnglish reports whether English-only issues are eligible.
func (criteria SearchCriteria) IncludesEnglish() bool {
	return criteria.includeEnglish
}

// ExcludesArchived reports whether archived repositories are rejected.
func (criteria SearchCriteria) ExcludesArchived() bool {
	return criteria.excludeArchived
}

// CacheKey returns a stable hash for the validated discovery criteria.
// Pagination and maximum effort are intentionally excluded because both are
// applied after the reusable GitHub candidate window is loaded.
func (criteria SearchCriteria) CacheKey() string {
	var canonical strings.Builder
	appendCanonical(&canonical, "username", strings.ToLower(criteria.username.String()))
	appendCanonicalValues(&canonical, "languages", criteria.languages)
	appendCanonicalValues(&canonical, "frameworks", criteria.frameworks)
	appendCanonicalValues(&canonical, "labels", criteria.labels)
	appendCanonical(&canonical, "minimumStars", strconv.Itoa(criteria.minimumStars))
	appendCanonical(
		&canonical,
		"maximumDifficulty",
		strconv.Itoa(criteria.maximumDifficulty.Int()),
	)
	appendCanonical(
		&canonical,
		"updatedWithinDays",
		strconv.Itoa(criteria.updatedWithinDays),
	)
	appendCanonical(
		&canonical,
		"includeDocumentation",
		strconv.FormatBool(criteria.includeDocumentation),
	)
	appendCanonical(
		&canonical,
		"includeEnglish",
		strconv.FormatBool(criteria.includeEnglish),
	)
	appendCanonical(
		&canonical,
		"excludeArchived",
		strconv.FormatBool(criteria.excludeArchived),
	)

	hash := sha256.Sum256([]byte(canonical.String()))
	return "github:issue-search:" + hex.EncodeToString(hash[:])
}

// Pagination is a validated page window over the bounded candidate result.
type Pagination struct {
	Page    int
	PerPage int
}

// NewPagination validates an application page over the bounded candidate set.
func NewPagination(page, perPage int) (Pagination, error) {
	if page < 1 {
		return Pagination{}, fmt.Errorf(
			"%w: page must be at least 1",
			ErrInvalidSearchCriteria,
		)
	}
	if perPage < 1 || perPage > MaximumPageSize {
		return Pagination{}, fmt.Errorf(
			"%w: perPage must be between 1 and %d",
			ErrInvalidSearchCriteria,
			MaximumPageSize,
		)
	}
	return Pagination{Page: page, PerPage: perPage}, nil
}

func normalizeFilterValues(
	field string,
	rawValues []string,
) ([]FilterValue, error) {
	if len(rawValues) > MaximumFilterValues {
		return nil, fmt.Errorf(
			"%w: %s cannot contain more than %d values",
			ErrInvalidSearchCriteria,
			field,
			MaximumFilterValues,
		)
	}

	normalized := make([]FilterValue, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, rawValue := range rawValues {
		value, err := ParseFilterValue(rawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		key := strings.ToLower(value.String())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}
	slices.SortFunc(normalized, func(left, right FilterValue) int {
		return strings.Compare(
			strings.ToLower(left.String()),
			strings.ToLower(right.String()),
		)
	})
	return normalized, nil
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func intOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func valueOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func appendCanonical(builder *strings.Builder, name, value string) {
	builder.WriteString(name)
	builder.WriteByte('=')
	builder.WriteString(value)
	builder.WriteByte(0)
}

func appendCanonicalValues(
	builder *strings.Builder,
	name string,
	values []FilterValue,
) {
	builder.WriteString(name)
	builder.WriteByte('=')
	for _, value := range values {
		builder.WriteString(strings.ToLower(value.String()))
		builder.WriteByte(0x1f)
	}
	builder.WriteByte(0)
}
