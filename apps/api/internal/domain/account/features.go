package account

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

const (
	// MaximumBookmarks bounds storage and list work for one account.
	MaximumBookmarks = 200
	// MaximumSavedSearches bounds stored filter documents for one account.
	MaximumSavedSearches = 50
	// MaximumSavedSearchNameRunes is the user-visible saved-search name limit.
	MaximumSavedSearchNameRunes = 80
	// MaximumSavedSearchFilterBytes matches the PostgreSQL JSON document limit.
	MaximumSavedSearchFilterBytes = 8192
	// DefaultPageSize is used when an account collection omits pagination.
	DefaultPageSize = 20
	// MaximumPageSize bounds account collection reads.
	MaximumPageSize = 50
	// MaximumPage bounds offset work because every collection is quota-bound.
	MaximumPage = 100
)

var (
	// ErrInvalidFeatureInput reports malformed account-owned feature data.
	ErrInvalidFeatureInput = errors.New("invalid account feature input")
	// ErrQuotaExceeded reports that an account-owned collection reached its
	// documented hard limit.
	ErrQuotaExceeded = errors.New("account feature quota exceeded")
	// ErrVersionConflict reports a stale optimistic-concurrency version.
	ErrVersionConflict = errors.New("account feature version conflict")
	// ErrDuplicateSavedSearch reports a case-insensitive saved-search name
	// collision within one account.
	ErrDuplicateSavedSearch = errors.New("saved search name already exists")
)

// ResourceID is a canonical UUID for an account-owned resource. It is
// intentionally distinct from ID so resource identifiers cannot be used as
// account ownership predicates accidentally.
type ResourceID [16]byte

// NewResourceID creates a random RFC 4122 version 4 resource identifier.
func NewResourceID() (ResourceID, error) {
	id, err := NewID()
	if err != nil {
		return ResourceID{}, err
	}
	return ResourceID(id), nil
}

// ParseResourceID accepts only the canonical lower-case UUID form.
func ParseResourceID(raw string) (ResourceID, error) {
	id, err := ParseID(raw)
	if err != nil {
		return ResourceID{}, fmt.Errorf("%w: resource id", ErrInvalidFeatureInput)
	}
	return ResourceID(id), nil
}

// String returns the canonical lower-case UUID representation.
func (id ResourceID) String() string {
	return ID(id).String()
}

// BookmarkTarget identifies the normalized public GitHub object referenced by
// a bookmark.
type BookmarkTarget string

const (
	// BookmarkTargetIssue identifies one repository-local GitHub issue.
	BookmarkTargetIssue BookmarkTarget = "issue"
	// BookmarkTargetRepository identifies one public GitHub repository.
	BookmarkTargetRepository BookmarkTarget = "repository"
)

// BookmarkReference contains a normalized GitHub reference rather than a
// copied upstream response. Stale or deleted upstream objects remain
// representable and are revalidated only through anonymous GitHub routes.
type BookmarkReference struct {
	TargetType      BookmarkTarget
	RepositoryOwner string
	RepositoryName  string
	IssueNumber     *int
}

// NewBookmarkReference validates and normalizes a bookmark target.
func NewBookmarkReference(
	targetType BookmarkTarget,
	owner string,
	repositoryName string,
	issueNumber *int,
) (BookmarkReference, error) {
	validationNumber := 1
	switch targetType {
	case BookmarkTargetIssue:
		if issueNumber == nil || *issueNumber < 1 {
			return BookmarkReference{}, fmt.Errorf(
				"%w: issueNumber is required for issue bookmarks",
				ErrInvalidFeatureInput,
			)
		}
		validationNumber = *issueNumber
	case BookmarkTargetRepository:
		if issueNumber != nil {
			return BookmarkReference{}, fmt.Errorf(
				"%w: issueNumber is not allowed for repository bookmarks",
				ErrInvalidFeatureInput,
			)
		}
	default:
		return BookmarkReference{}, fmt.Errorf(
			"%w: targetType must be issue or repository",
			ErrInvalidFeatureInput,
		)
	}
	reference, err := issue.NewReference(
		owner,
		repositoryName,
		validationNumber,
	)
	if err != nil {
		return BookmarkReference{}, fmt.Errorf(
			"%w: GitHub reference is invalid",
			ErrInvalidFeatureInput,
		)
	}
	var normalizedIssueNumber *int
	if targetType == BookmarkTargetIssue {
		number := reference.Number()
		normalizedIssueNumber = &number
	}
	return BookmarkReference{
		TargetType:      targetType,
		RepositoryOwner: strings.ToLower(reference.Owner()),
		RepositoryName:  strings.ToLower(reference.RepositoryName()),
		IssueNumber:     normalizedIssueNumber,
	}, nil
}

// Bookmark is one account-owned normalized GitHub reference.
type Bookmark struct {
	ID        ResourceID
	AccountID ID
	Reference BookmarkReference
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SearchType identifies which anonymous domain contract validates a saved
// filter document.
type SearchType string

const (
	// SearchTypeIssue uses the anonymous issue-search contract.
	SearchTypeIssue SearchType = "issue"
	// SearchTypeRepository uses the anonymous repository-discovery contract.
	SearchTypeRepository SearchType = "repository"
)

// SavedSearch is a named, normalized filter document owned by one account.
type SavedSearch struct {
	ID         ResourceID
	AccountID  ID
	SearchType SearchType
	Name       string
	Filters    json.RawMessage
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NormalizeSavedSearchName trims a bounded display name and rejects controls.
func NormalizeSavedSearchName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" ||
		utf8.RuneCountInString(name) > MaximumSavedSearchNameRunes ||
		len(name) > MaximumSavedSearchNameRunes*4 {
		return "", fmt.Errorf(
			"%w: name must contain 1-%d characters",
			ErrInvalidFeatureInput,
			MaximumSavedSearchNameRunes,
		)
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return "", fmt.Errorf(
				"%w: name cannot contain control characters",
				ErrInvalidFeatureInput,
			)
		}
	}
	return name, nil
}

// NormalizeSavedSearchFilters strictly decodes, validates, and canonicalizes a
// saved filter document through the same domain constructors as an anonymous
// issue or repository search.
func NormalizeSavedSearchFilters(
	searchType SearchType,
	raw json.RawMessage,
) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > MaximumSavedSearchFilterBytes {
		return nil, fmt.Errorf(
			"%w: filters must be a JSON object of at most %d bytes",
			ErrInvalidFeatureInput,
			MaximumSavedSearchFilterBytes,
		)
	}
	var normalized any
	var err error
	switch searchType {
	case SearchTypeIssue:
		normalized, err = normalizeIssueFilters(raw)
	case SearchTypeRepository:
		normalized, err = normalizeRepositoryFilters(raw)
	default:
		return nil, fmt.Errorf(
			"%w: searchType must be issue or repository",
			ErrInvalidFeatureInput,
		)
	}
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil || len(encoded) > MaximumSavedSearchFilterBytes {
		return nil, fmt.Errorf(
			"%w: filters cannot be normalized",
			ErrInvalidFeatureInput,
		)
	}
	return json.RawMessage(encoded), nil
}

// Theme is the persisted visual theme preference.
type Theme string

const (
	// ThemeLight requests a light color scheme.
	ThemeLight Theme = "light"
	// ThemeDark requests a dark color scheme.
	ThemeDark Theme = "dark"
	// ThemeSystem follows the operating-system preference.
	ThemeSystem Theme = "system"
)

// ReducedMotion is the persisted animation preference.
type ReducedMotion string

const (
	// ReducedMotionReduce requests reduced motion.
	ReducedMotionReduce ReducedMotion = "reduce"
	// ReducedMotionNoPreference allows normal motion.
	ReducedMotionNoPreference ReducedMotion = "no-preference"
	// ReducedMotionSystem follows the operating-system preference.
	ReducedMotionSystem ReducedMotion = "system"
)

// Preferences contains the bounded account display settings.
type Preferences struct {
	AccountID      ID
	Theme          Theme
	ReducedMotion  ReducedMotion
	ResultsPerPage int
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewPreferences validates display preferences independently of transport and
// persistence.
func NewPreferences(
	theme Theme,
	reducedMotion ReducedMotion,
	resultsPerPage int,
) (Preferences, error) {
	switch theme {
	case ThemeLight, ThemeDark, ThemeSystem:
	default:
		return Preferences{}, fmt.Errorf(
			"%w: theme must be light, dark, or system",
			ErrInvalidFeatureInput,
		)
	}
	switch reducedMotion {
	case ReducedMotionReduce,
		ReducedMotionNoPreference,
		ReducedMotionSystem:
	default:
		return Preferences{}, fmt.Errorf(
			"%w: reducedMotion is invalid",
			ErrInvalidFeatureInput,
		)
	}
	if resultsPerPage != 10 &&
		resultsPerPage != 20 &&
		resultsPerPage != 50 {
		return Preferences{}, fmt.Errorf(
			"%w: resultsPerPage must be 10, 20, or 50",
			ErrInvalidFeatureInput,
		)
	}
	return Preferences{
		Theme:          theme,
		ReducedMotion:  reducedMotion,
		ResultsPerPage: resultsPerPage,
	}, nil
}

// Page is a bounded deterministic page request over a quota-bound collection.
type Page struct {
	Number  int
	PerPage int
}

// NewPage validates account collection pagination.
func NewPage(number int, perPage int) (Page, error) {
	if number < 1 || number > MaximumPage ||
		perPage < 1 || perPage > MaximumPageSize {
		return Page{}, fmt.Errorf(
			"%w: page must be 1-%d and perPage must be 1-%d",
			ErrInvalidFeatureInput,
			MaximumPage,
			MaximumPageSize,
		)
	}
	return Page{Number: number, PerPage: perPage}, nil
}

// Offset returns the safe SQL offset for this bounded page.
func (page Page) Offset() int {
	return (page.Number - 1) * page.PerPage
}

// PageResult contains a deterministic collection window and its total count.
type PageResult[T any] struct {
	Items []T
	Page  Page
	Total int
}

// Export contains the complete bounded account-owned data set. It deliberately
// excludes sessions, credential hashes, audit identifiers, and GitHub payloads.
type Export struct {
	GeneratedAt   time.Time
	Bookmarks     []Bookmark
	SavedSearches []SavedSearch
	Preferences   *Preferences
}

type issueFilterDocument struct {
	Username             string   `json:"username"`
	Languages            []string `json:"languages,omitempty"`
	Frameworks           []string `json:"frameworks,omitempty"`
	Labels               []string `json:"labels,omitempty"`
	MinimumStars         *int     `json:"minimumStars,omitempty"`
	MaximumDifficulty    *int     `json:"maximumDifficulty,omitempty"`
	MaximumEffort        *string  `json:"maximumEffort,omitempty"`
	UpdatedWithinDays    *int     `json:"updatedWithinDays,omitempty"`
	IncludeDocumentation *bool    `json:"includeDocumentation,omitempty"`
	IncludeEnglish       *bool    `json:"includeEnglish,omitempty"`
	ExcludeArchived      *bool    `json:"excludeArchived,omitempty"`
}

type canonicalIssueFilterDocument struct {
	Username             string   `json:"username"`
	Languages            []string `json:"languages"`
	Frameworks           []string `json:"frameworks"`
	Labels               []string `json:"labels"`
	MinimumStars         int      `json:"minimumStars"`
	MaximumDifficulty    int      `json:"maximumDifficulty"`
	MaximumEffort        *string  `json:"maximumEffort,omitempty"`
	UpdatedWithinDays    int      `json:"updatedWithinDays"`
	IncludeDocumentation bool     `json:"includeDocumentation"`
	IncludeEnglish       bool     `json:"includeEnglish"`
	ExcludeArchived      bool     `json:"excludeArchived"`
}

func normalizeIssueFilters(raw json.RawMessage) (any, error) {
	var document issueFilterDocument
	if err := decodeStrictDocument(raw, &document); err != nil {
		return nil, err
	}
	criteria, err := issue.NewSearchCriteria(issue.SearchCriteriaOptions{
		Username:             document.Username,
		Languages:            document.Languages,
		Frameworks:           document.Frameworks,
		Labels:               document.Labels,
		MinimumStars:         document.MinimumStars,
		MaximumDifficulty:    document.MaximumDifficulty,
		MaximumEffort:        document.MaximumEffort,
		UpdatedWithinDays:    document.UpdatedWithinDays,
		IncludeDocumentation: document.IncludeDocumentation,
		IncludeEnglish:       document.IncludeEnglish,
		ExcludeArchived:      document.ExcludeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: issue filters", ErrInvalidFeatureInput)
	}
	var maximumEffort *string
	if value, ok := criteria.MaximumEffort(); ok {
		rawValue := string(value)
		maximumEffort = &rawValue
	}
	return canonicalIssueFilterDocument{
		Username:             criteria.Username().String(),
		Languages:            issueValues(criteria.Languages()),
		Frameworks:           issueValues(criteria.Frameworks()),
		Labels:               issueValues(criteria.Labels()),
		MinimumStars:         criteria.MinimumStars(),
		MaximumDifficulty:    criteria.MaximumDifficulty().Int(),
		MaximumEffort:        maximumEffort,
		UpdatedWithinDays:    criteria.UpdatedWithinDays(),
		IncludeDocumentation: criteria.IncludesDocumentation(),
		IncludeEnglish:       criteria.IncludesEnglish(),
		ExcludeArchived:      criteria.ExcludesArchived(),
	}, nil
}

type repositoryFilterDocument struct {
	Languages         []string `json:"languages,omitempty"`
	Technologies      []string `json:"technologies,omitempty"`
	Licenses          []string `json:"licenses,omitempty"`
	Categories        []string `json:"categories,omitempty"`
	MinimumStars      *int     `json:"minimumStars,omitempty"`
	MinimumForks      *int     `json:"minimumForks,omitempty"`
	MinimumOpenIssues *int     `json:"minimumOpenIssues,omitempty"`
	MaximumOpenIssues *int     `json:"maximumOpenIssues,omitempty"`
	UpdatedWithinDays *int     `json:"updatedWithinDays,omitempty"`
	MaximumDifficulty *int     `json:"maximumDifficulty,omitempty"`
	MinimumReadiness  *int     `json:"minimumReadiness,omitempty"`
	HasJapaneseREADME *bool    `json:"hasJapaneseREADME,omitempty"`
	ForkPolicy        *string  `json:"forkPolicy,omitempty"`
	ExcludeArchived   *bool    `json:"excludeArchived,omitempty"`
}

type canonicalRepositoryFilterDocument struct {
	Languages         []string `json:"languages"`
	Technologies      []string `json:"technologies"`
	Licenses          []string `json:"licenses"`
	Categories        []string `json:"categories"`
	MinimumStars      int      `json:"minimumStars"`
	MinimumForks      int      `json:"minimumForks"`
	MinimumOpenIssues int      `json:"minimumOpenIssues"`
	MaximumOpenIssues *int     `json:"maximumOpenIssues,omitempty"`
	UpdatedWithinDays int      `json:"updatedWithinDays"`
	MaximumDifficulty int      `json:"maximumDifficulty"`
	MinimumReadiness  int      `json:"minimumReadiness"`
	HasJapaneseREADME *bool    `json:"hasJapaneseREADME,omitempty"`
	ForkPolicy        string   `json:"forkPolicy"`
	ExcludeArchived   bool     `json:"excludeArchived"`
}

func normalizeRepositoryFilters(raw json.RawMessage) (any, error) {
	var document repositoryFilterDocument
	if err := decodeStrictDocument(raw, &document); err != nil {
		return nil, err
	}
	criteria, err := repository.NewDiscoveryCriteria(
		repository.DiscoveryCriteriaOptions{
			Languages:         document.Languages,
			Technologies:      document.Technologies,
			Licenses:          document.Licenses,
			Categories:        document.Categories,
			MinimumStars:      document.MinimumStars,
			MinimumForks:      document.MinimumForks,
			MinimumOpenIssues: document.MinimumOpenIssues,
			MaximumOpenIssues: document.MaximumOpenIssues,
			UpdatedWithinDays: document.UpdatedWithinDays,
			MaximumDifficulty: document.MaximumDifficulty,
			MinimumReadiness:  document.MinimumReadiness,
			HasJapaneseREADME: document.HasJapaneseREADME,
			ForkPolicy:        document.ForkPolicy,
			ExcludeArchived:   document.ExcludeArchived,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("%w: repository filters", ErrInvalidFeatureInput)
	}
	var maximumOpenIssues *int
	if value, ok := criteria.MaximumOpenIssues(); ok {
		maximumOpenIssues = &value
	}
	var hasJapaneseREADME *bool
	if value, ok := criteria.HasJapaneseREADME(); ok {
		hasJapaneseREADME = &value
	}
	return canonicalRepositoryFilterDocument{
		Languages:         stringValues(criteria.Languages()),
		Technologies:      stringValues(criteria.Technologies()),
		Licenses:          stringValues(criteria.Licenses()),
		Categories:        stringValues(criteria.Categories()),
		MinimumStars:      criteria.MinimumStars(),
		MinimumForks:      criteria.MinimumForks(),
		MinimumOpenIssues: criteria.MinimumOpenIssues(),
		MaximumOpenIssues: maximumOpenIssues,
		UpdatedWithinDays: criteria.UpdatedWithinDays(),
		MaximumDifficulty: criteria.MaximumDifficulty(),
		MinimumReadiness:  criteria.MinimumReadiness(),
		HasJapaneseREADME: hasJapaneseREADME,
		ForkPolicy:        string(criteria.ForkPolicy()),
		ExcludeArchived:   criteria.ExcludesArchived(),
	}, nil
}

func decodeStrictDocument(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: filters are invalid", ErrInvalidFeatureInput)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: filters contain trailing data", ErrInvalidFeatureInput)
	}
	return nil
}

func issueValues[T ~string](values []T) []string {
	return stringValues(values)
}

func stringValues[T ~string](values []T) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}
