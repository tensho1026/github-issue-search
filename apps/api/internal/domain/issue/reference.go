package issue

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidReference reports an unsafe owner, repository, or issue number.
var ErrInvalidReference = errors.New("invalid issue reference")

// Reference is a validated public GitHub owner, repository, and issue number.
// Its canonical key is safe for bounded in-memory caches and singleflight.
type Reference struct {
	owner          string
	repositoryName string
	number         int
}

// NewReference validates GitHub owner and repository syntax and requires a
// positive issue number.
func NewReference(
	owner string,
	repositoryName string,
	number int,
) (Reference, error) {
	if !validGitHubOwner(owner) {
		return Reference{}, fmt.Errorf(
			"%w: GitHub repository owner is invalid",
			ErrInvalidReference,
		)
	}
	if !validGitHubRepositoryName(repositoryName) {
		return Reference{}, fmt.Errorf(
			"%w: GitHub repository name is invalid",
			ErrInvalidReference,
		)
	}
	if number < 1 {
		return Reference{}, fmt.Errorf(
			"%w: GitHub issue number must be positive",
			ErrInvalidReference,
		)
	}
	return Reference{
		owner:          owner,
		repositoryName: repositoryName,
		number:         number,
	}, nil
}

// Owner returns the validated, case-preserving repository owner.
func (reference Reference) Owner() string {
	return reference.owner
}

// RepositoryName returns the validated, case-preserving repository name.
func (reference Reference) RepositoryName() string {
	return reference.repositoryName
}

// Number returns the positive GitHub issue number.
func (reference Reference) Number() int {
	return reference.number
}

// CacheKey returns the canonical case-insensitive issue-detail cache key.
func (reference Reference) CacheKey() string {
	return "github:issue-detail:" +
		strings.ToLower(reference.owner) + "/" +
		strings.ToLower(reference.repositoryName) + "#" +
		strconv.Itoa(reference.number)
}

func validGitHubOwner(value string) bool {
	if value == "" || len(value) > 39 ||
		strings.TrimSpace(value) != value ||
		strings.HasPrefix(value, "-") ||
		strings.HasSuffix(value, "-") ||
		strings.Contains(value, "--") {
		return false
	}
	for _, character := range value {
		if isASCIILetterOrDigit(character) || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validGitHubRepositoryName(value string) bool {
	if value == "" || len(value) > 100 ||
		strings.TrimSpace(value) != value ||
		value == "." ||
		value == ".." {
		return false
	}
	for _, character := range value {
		if isASCIILetterOrDigit(character) ||
			character == '-' ||
			character == '_' ||
			character == '.' {
			continue
		}
		return false
	}
	return true
}

func isASCIILetterOrDigit(character rune) bool {
	return (character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}
