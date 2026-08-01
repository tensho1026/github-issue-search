package repository

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
)

// Repository-discovery defaults and hard limits bound qualifier size,
// pagination, enrichment fan-out, and numeric filters.
const (
	DefaultDiscoveryMinimumStars      = 10
	DefaultDiscoveryUpdatedWithinDays = 365
	DefaultDiscoveryPage              = 1
	DefaultDiscoveryPerPage           = 20

	MaximumDiscoveryFilterValues      = 10
	MaximumDiscoveryFilterValueRunes  = 64
	MaximumDiscoveryFilterValueBytes  = 128
	MaximumDiscoveryUpdatedWithinDays = 3650
	MaximumDiscoveryCandidateResults  = 50
	MaximumDiscoveryEnrichmentResults = 20
	MaximumDiscoveryPage              = 50
	MaximumDiscoveryPageSize          = 50
	MaximumDiscoveryCountFilter       = 10_000_000
)

// ErrInvalidDiscoveryCriteria reports an unsafe or unsupported discovery
// option.
var ErrInvalidDiscoveryCriteria = errors.New(
	"invalid repository discovery criteria",
)

// FilterValue is a validated language or technology selector.
type FilterValue string

// ParseFilterValue rejects characters that could escape a quoted GitHub
// search qualifier. The client still owns final GraphQL encoding.
func ParseFilterValue(raw string) (FilterValue, error) {
	value := strings.TrimSpace(raw)
	if value == "" ||
		utf8.RuneCountInString(value) > MaximumDiscoveryFilterValueRunes ||
		len(value) > MaximumDiscoveryFilterValueBytes {
		return "", fmt.Errorf(
			"%w: filter values must contain 1-%d characters and at most %d bytes",
			ErrInvalidDiscoveryCriteria,
			MaximumDiscoveryFilterValueRunes,
			MaximumDiscoveryFilterValueBytes,
		)
	}
	for _, character := range value {
		if character == '"' || character == '\\' || unicode.IsControl(character) {
			return "", fmt.Errorf(
				"%w: filter values cannot contain quotes, backslashes, or control characters",
				ErrInvalidDiscoveryCriteria,
			)
		}
	}
	return FilterValue(value), nil
}

// String returns the validated, trimmed repository filter value.
func (value FilterValue) String() string {
	return string(value)
}

// SPDXLicense is one explicitly supported SPDX identifier. A bounded
// allowlist prevents a syntactically valid but unsupported identifier from
// producing misleading GitHub search behavior.
type SPDXLicense string

var supportedSPDXLicenses = map[string]SPDXLicense{
	"0bsd":         "0BSD",
	"agpl-3.0":     "AGPL-3.0",
	"apache-2.0":   "Apache-2.0",
	"bsd-2-clause": "BSD-2-Clause",
	"bsd-3-clause": "BSD-3-Clause",
	"bsl-1.0":      "BSL-1.0",
	"cc0-1.0":      "CC0-1.0",
	"epl-2.0":      "EPL-2.0",
	"gpl-2.0":      "GPL-2.0",
	"gpl-3.0":      "GPL-3.0",
	"isc":          "ISC",
	"lgpl-2.1":     "LGPL-2.1",
	"lgpl-3.0":     "LGPL-3.0",
	"mit":          "MIT",
	"mpl-2.0":      "MPL-2.0",
	"unlicense":    "Unlicense",
}

// ParseSPDXLicense resolves a case-insensitive input against the supported
// SPDX allowlist.
func ParseSPDXLicense(raw string) (SPDXLicense, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	license, supported := supportedSPDXLicenses[key]
	if !supported {
		return "", fmt.Errorf(
			"%w: unsupported SPDX license %q",
			ErrInvalidDiscoveryCriteria,
			raw,
		)
	}
	return license, nil
}

// String returns the canonical case-preserving SPDX identifier.
func (license SPDXLicense) String() string {
	return string(license)
}

// Category is an explainable, preliminary OSS project classification.
type Category string

// Category values form the supported preliminary OSS classifications.
const (
	CategoryApplication    Category = "application"
	CategoryData           Category = "data"
	CategoryDocumentation  Category = "documentation"
	CategoryEducation      Category = "education"
	CategoryFramework      Category = "framework"
	CategoryInfrastructure Category = "infrastructure"
	CategoryLibrary        Category = "library"
	CategorySecurity       Category = "security"
	CategoryTooling        Category = "tooling"
)

var validCategories = map[Category]struct{}{
	CategoryApplication:    {},
	CategoryData:           {},
	CategoryDocumentation:  {},
	CategoryEducation:      {},
	CategoryFramework:      {},
	CategoryInfrastructure: {},
	CategoryLibrary:        {},
	CategorySecurity:       {},
	CategoryTooling:        {},
}

// ParseCategory validates a case-insensitive OSS project category.
func ParseCategory(raw string) (Category, error) {
	category := Category(strings.ToLower(strings.TrimSpace(raw)))
	if _, valid := validCategories[category]; !valid {
		return "", fmt.Errorf(
			"%w: unsupported OSS category %q",
			ErrInvalidDiscoveryCriteria,
			raw,
		)
	}
	return category, nil
}

// ForkPolicy defines whether original repositories, forks, or both are
// eligible. GitHub excludes forks by default, so the policy is explicit.
type ForkPolicy string

// ForkPolicy values define original-and-fork eligibility.
const (
	ForkPolicyExclude ForkPolicy = "exclude"
	ForkPolicyInclude ForkPolicy = "include"
	ForkPolicyOnly    ForkPolicy = "only"
)

// ParseForkPolicy validates the closed, case-insensitive fork vocabulary.
func ParseForkPolicy(raw string) (ForkPolicy, error) {
	policy := ForkPolicy(strings.ToLower(strings.TrimSpace(raw)))
	switch policy {
	case ForkPolicyExclude, ForkPolicyInclude, ForkPolicyOnly:
		return policy, nil
	default:
		return "", fmt.Errorf(
			"%w: forkPolicy must be exclude, include, or only",
			ErrInvalidDiscoveryCriteria,
		)
	}
}

// DiscoveryCriteriaOptions contains transport-independent repository filters.
type DiscoveryCriteriaOptions struct {
	Languages         []string
	Technologies      []string
	Licenses          []string
	Categories        []string
	MinimumStars      *int
	MinimumForks      *int
	MinimumOpenIssues *int
	MaximumOpenIssues *int
	UpdatedWithinDays *int
	MaximumDifficulty *int
	MinimumReadiness  *int
	HasJapaneseREADME *bool
	ForkPolicy        *string
	ExcludeArchived   *bool
}

// DiscoveryCriteria is immutable canonical repository-discovery state.
type DiscoveryCriteria struct {
	languages         []FilterValue
	technologies      []FilterValue
	licenses          []SPDXLicense
	categories        []Category
	minimumStars      int
	minimumForks      int
	minimumOpenIssues int
	maximumOpenIssues *int
	updatedWithinDays int
	maximumDifficulty int
	minimumReadiness  int
	hasJapaneseREADME *bool
	forkPolicy        ForkPolicy
	excludeArchived   bool
}

// NewDiscoveryCriteria validates, defaults, deduplicates, and canonicalizes
// repository filters. Returned slice accessors do not expose internal storage.
func NewDiscoveryCriteria(
	options DiscoveryCriteriaOptions,
) (DiscoveryCriteria, error) {
	languages, err := normalizeDiscoveryFilterValues(
		"languages",
		options.Languages,
	)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	technologies, err := normalizeDiscoveryFilterValues(
		"technologies",
		options.Technologies,
	)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	licenses, err := normalizeLicenses(options.Licenses)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	categories, err := normalizeCategories(options.Categories)
	if err != nil {
		return DiscoveryCriteria{}, err
	}

	minimumStars, err := boundedCount(
		"minimumStars",
		options.MinimumStars,
		DefaultDiscoveryMinimumStars,
	)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	minimumForks, err := boundedCount("minimumForks", options.MinimumForks, 0)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	minimumOpenIssues, err := boundedCount(
		"minimumOpenIssues",
		options.MinimumOpenIssues,
		0,
	)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	maximumOpenIssues, err := optionalBoundedCount(
		"maximumOpenIssues",
		options.MaximumOpenIssues,
	)
	if err != nil {
		return DiscoveryCriteria{}, err
	}
	if maximumOpenIssues != nil && *maximumOpenIssues < minimumOpenIssues {
		return DiscoveryCriteria{}, fmt.Errorf(
			"%w: maximumOpenIssues cannot be less than minimumOpenIssues",
			ErrInvalidDiscoveryCriteria,
		)
	}

	updatedWithinDays := intOrDefault(
		options.UpdatedWithinDays,
		DefaultDiscoveryUpdatedWithinDays,
	)
	if updatedWithinDays < 1 ||
		updatedWithinDays > MaximumDiscoveryUpdatedWithinDays {
		return DiscoveryCriteria{}, fmt.Errorf(
			"%w: updatedWithinDays must be between 1 and %d",
			ErrInvalidDiscoveryCriteria,
			MaximumDiscoveryUpdatedWithinDays,
		)
	}
	maximumDifficulty := intOrDefault(options.MaximumDifficulty, 5)
	if maximumDifficulty < 1 || maximumDifficulty > 5 {
		return DiscoveryCriteria{}, fmt.Errorf(
			"%w: maximumDifficulty must be between 1 and 5",
			ErrInvalidDiscoveryCriteria,
		)
	}
	minimumReadiness := intOrDefault(options.MinimumReadiness, 0)
	if minimumReadiness < 0 || minimumReadiness > 100 {
		return DiscoveryCriteria{}, fmt.Errorf(
			"%w: minimumReadiness must be between 0 and 100",
			ErrInvalidDiscoveryCriteria,
		)
	}

	forkPolicy := ForkPolicyExclude
	if options.ForkPolicy != nil {
		forkPolicy, err = ParseForkPolicy(*options.ForkPolicy)
		if err != nil {
			return DiscoveryCriteria{}, err
		}
	}

	return DiscoveryCriteria{
		languages:         languages,
		technologies:      technologies,
		licenses:          licenses,
		categories:        categories,
		minimumStars:      minimumStars,
		minimumForks:      minimumForks,
		minimumOpenIssues: minimumOpenIssues,
		maximumOpenIssues: cloneIntPointer(maximumOpenIssues),
		updatedWithinDays: updatedWithinDays,
		maximumDifficulty: maximumDifficulty,
		minimumReadiness:  minimumReadiness,
		hasJapaneseREADME: cloneBoolPointer(options.HasJapaneseREADME),
		forkPolicy:        forkPolicy,
		excludeArchived:   boolOrDefault(options.ExcludeArchived, true),
	}, nil
}

// Languages returns a defensive copy of canonical language filters.
func (criteria DiscoveryCriteria) Languages() []FilterValue {
	return slices.Clone(criteria.languages)
}

// Technologies returns a defensive copy of canonical technology filters.
func (criteria DiscoveryCriteria) Technologies() []FilterValue {
	return slices.Clone(criteria.technologies)
}

// Licenses returns a defensive copy of canonical SPDX filters.
func (criteria DiscoveryCriteria) Licenses() []SPDXLicense {
	return slices.Clone(criteria.licenses)
}

// Categories returns a defensive copy of canonical category filters.
func (criteria DiscoveryCriteria) Categories() []Category {
	return slices.Clone(criteria.categories)
}

// MinimumStars returns the inclusive repository star threshold.
func (criteria DiscoveryCriteria) MinimumStars() int {
	return criteria.minimumStars
}

// MinimumForks returns the inclusive repository fork threshold.
func (criteria DiscoveryCriteria) MinimumForks() int {
	return criteria.minimumForks
}

// MinimumOpenIssues returns the inclusive lower open-issue threshold.
func (criteria DiscoveryCriteria) MinimumOpenIssues() int {
	return criteria.minimumOpenIssues
}

// MaximumOpenIssues returns the optional inclusive upper open-issue threshold.
func (criteria DiscoveryCriteria) MaximumOpenIssues() (int, bool) {
	if criteria.maximumOpenIssues == nil {
		return 0, false
	}
	return *criteria.maximumOpenIssues, true
}

// UpdatedWithinDays returns the repository freshness window.
func (criteria DiscoveryCriteria) UpdatedWithinDays() int {
	return criteria.updatedWithinDays
}

// MaximumDifficulty returns the inclusive preliminary difficulty ceiling.
func (criteria DiscoveryCriteria) MaximumDifficulty() int {
	return criteria.maximumDifficulty
}

// MinimumReadiness returns the inclusive contribution-readiness threshold.
func (criteria DiscoveryCriteria) MinimumReadiness() int {
	return criteria.minimumReadiness
}

// HasJapaneseREADME returns the optional README-language requirement.
func (criteria DiscoveryCriteria) HasJapaneseREADME() (bool, bool) {
	if criteria.hasJapaneseREADME == nil {
		return false, false
	}
	return *criteria.hasJapaneseREADME, true
}

// ForkPolicy returns the validated original-and-fork selection policy.
func (criteria DiscoveryCriteria) ForkPolicy() ForkPolicy {
	return criteria.forkPolicy
}

// ExcludesArchived reports whether archived repositories are rejected.
func (criteria DiscoveryCriteria) ExcludesArchived() bool {
	return criteria.excludeArchived
}

// CacheKey includes every filter that can change the bounded normalized result.
// Pagination is deliberately excluded.
func (criteria DiscoveryCriteria) CacheKey() string {
	var canonical strings.Builder
	appendCanonicalValues(&canonical, "languages", criteria.languages)
	appendCanonicalValues(&canonical, "technologies", criteria.technologies)
	for _, license := range criteria.licenses {
		appendCanonical(&canonical, "license", license.String())
	}
	for _, category := range criteria.categories {
		appendCanonical(&canonical, "category", string(category))
	}
	appendCanonical(
		&canonical,
		"minimumStars",
		strconv.Itoa(criteria.minimumStars),
	)
	appendCanonical(
		&canonical,
		"minimumForks",
		strconv.Itoa(criteria.minimumForks),
	)
	appendCanonical(
		&canonical,
		"minimumOpenIssues",
		strconv.Itoa(criteria.minimumOpenIssues),
	)
	if criteria.maximumOpenIssues != nil {
		appendCanonical(
			&canonical,
			"maximumOpenIssues",
			strconv.Itoa(*criteria.maximumOpenIssues),
		)
	}
	appendCanonical(
		&canonical,
		"updatedWithinDays",
		strconv.Itoa(criteria.updatedWithinDays),
	)
	appendCanonical(
		&canonical,
		"maximumDifficulty",
		strconv.Itoa(criteria.maximumDifficulty),
	)
	appendCanonical(
		&canonical,
		"minimumReadiness",
		strconv.Itoa(criteria.minimumReadiness),
	)
	if criteria.hasJapaneseREADME != nil {
		appendCanonical(
			&canonical,
			"hasJapaneseREADME",
			strconv.FormatBool(*criteria.hasJapaneseREADME),
		)
	}
	appendCanonical(&canonical, "forkPolicy", string(criteria.forkPolicy))
	appendCanonical(
		&canonical,
		"excludeArchived",
		strconv.FormatBool(criteria.excludeArchived),
	)

	hash := sha256.Sum256([]byte(canonical.String()))
	return "github:repository-discovery:" + hex.EncodeToString(hash[:])
}

// DiscoveryPagination is a validated application page over the bounded result.
type DiscoveryPagination struct {
	Page    int
	PerPage int
}

// NewDiscoveryPagination validates an application page over the bounded
// repository result set.
func NewDiscoveryPagination(page, perPage int) (DiscoveryPagination, error) {
	if page < 1 || page > MaximumDiscoveryPage {
		return DiscoveryPagination{}, fmt.Errorf(
			"%w: page must be between 1 and %d",
			ErrInvalidDiscoveryCriteria,
			MaximumDiscoveryPage,
		)
	}
	if perPage < 1 || perPage > MaximumDiscoveryPageSize {
		return DiscoveryPagination{}, fmt.Errorf(
			"%w: perPage must be between 1 and %d",
			ErrInvalidDiscoveryCriteria,
			MaximumDiscoveryPageSize,
		)
	}
	return DiscoveryPagination{Page: page, PerPage: perPage}, nil
}

func normalizeDiscoveryFilterValues(
	field string,
	rawValues []string,
) ([]FilterValue, error) {
	if len(rawValues) > MaximumDiscoveryFilterValues {
		return nil, fmt.Errorf(
			"%w: %s cannot contain more than %d values",
			ErrInvalidDiscoveryCriteria,
			field,
			MaximumDiscoveryFilterValues,
		)
	}
	values := make([]FilterValue, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, raw := range rawValues {
		value, err := ParseFilterValue(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		key := strings.ToLower(value.String())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, value)
	}
	slices.SortFunc(values, func(left, right FilterValue) int {
		return strings.Compare(
			strings.ToLower(left.String()),
			strings.ToLower(right.String()),
		)
	})
	return values, nil
}

func normalizeLicenses(raw []string) ([]SPDXLicense, error) {
	if len(raw) > MaximumDiscoveryFilterValues {
		return nil, fmt.Errorf(
			"%w: licenses cannot contain more than %d values",
			ErrInvalidDiscoveryCriteria,
			MaximumDiscoveryFilterValues,
		)
	}
	values := make([]SPDXLicense, 0, len(raw))
	seen := make(map[SPDXLicense]struct{}, len(raw))
	for _, item := range raw {
		value, err := ParseSPDXLicense(item)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	slices.Sort(values)
	return values, nil
}

func normalizeCategories(raw []string) ([]Category, error) {
	if len(raw) > MaximumDiscoveryFilterValues {
		return nil, fmt.Errorf(
			"%w: categories cannot contain more than %d values",
			ErrInvalidDiscoveryCriteria,
			MaximumDiscoveryFilterValues,
		)
	}
	values := make([]Category, 0, len(raw))
	seen := make(map[Category]struct{}, len(raw))
	for _, item := range raw {
		value, err := ParseCategory(item)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	slices.Sort(values)
	return values, nil
}

func boundedCount(field string, value *int, fallback int) (int, error) {
	parsed := intOrDefault(value, fallback)
	if parsed < 0 || parsed > MaximumDiscoveryCountFilter {
		return 0, fmt.Errorf(
			"%w: %s must be between 0 and %d",
			ErrInvalidDiscoveryCriteria,
			field,
			MaximumDiscoveryCountFilter,
		)
	}
	return parsed, nil
}

func optionalBoundedCount(field string, value *int) (*int, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := boundedCount(field, value, 0)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func intOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func appendCanonical(builder *strings.Builder, key, value string) {
	builder.WriteString(key)
	builder.WriteByte('=')
	builder.WriteString(strconv.Itoa(len(value)))
	builder.WriteByte(':')
	builder.WriteString(value)
	builder.WriteByte(';')
}

func appendCanonicalValues(
	builder *strings.Builder,
	key string,
	values []FilterValue,
) {
	for _, value := range values {
		appendCanonical(builder, key, strings.ToLower(value.String()))
	}
}
