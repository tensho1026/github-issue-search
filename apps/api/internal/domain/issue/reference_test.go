package issue

import (
	"errors"
	"testing"
)

func TestNewReferenceNormalizesCacheKeyWithoutChangingDisplayValues(
	t *testing.T,
) {
	t.Parallel()
	reference, err := NewReference("OpenAI", "Issue_Scout.go", 42)
	if err != nil {
		t.Fatalf("NewReference() error = %v", err)
	}
	if reference.Owner() != "OpenAI" ||
		reference.RepositoryName() != "Issue_Scout.go" ||
		reference.Number() != 42 ||
		reference.CacheKey() !=
			"github:issue-detail:openai/issue_scout.go#42" {
		t.Fatalf("reference = %+v, key = %q", reference, reference.CacheKey())
	}
}

func TestNewReferenceRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		owner      string
		repository string
		number     int
	}{
		{owner: "", repository: "repo", number: 1},
		{owner: "-owner", repository: "repo", number: 1},
		{owner: "owner-", repository: "repo", number: 1},
		{owner: "own--er", repository: "repo", number: 1},
		{owner: ".", repository: "repo", number: 1},
		{owner: "owner", repository: "", number: 1},
		{owner: "owner", repository: ".", number: 1},
		{owner: "owner", repository: "..", number: 1},
		{owner: "owner", repository: "repo/name", number: 1},
		{owner: "owner", repository: "repo", number: 0},
	}
	for _, test := range tests {
		_, err := NewReference(test.owner, test.repository, test.number)
		if !errors.Is(err, ErrInvalidReference) {
			t.Fatalf(
				"NewReference(%q, %q, %d) error = %v",
				test.owner,
				test.repository,
				test.number,
				err,
			)
		}
	}
}
