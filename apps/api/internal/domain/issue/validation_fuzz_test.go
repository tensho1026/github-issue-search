package issue

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func FuzzParseFilterValue(fuzzer *testing.F) {
	for _, seed := range []string{
		"TypeScript",
		" good first issue ",
		`language:"Go"`,
		"line\nbreak",
		"日本語",
		"",
	} {
		fuzzer.Add(seed)
	}

	fuzzer.Fuzz(func(t *testing.T, raw string) {
		value, err := ParseFilterValue(raw)
		if err != nil {
			return
		}
		normalized := value.String()
		if normalized != strings.TrimSpace(normalized) ||
			normalized == "" ||
			utf8.RuneCountInString(normalized) > MaximumFilterValueRunes ||
			len(normalized) > MaximumFilterValueBytes {
			t.Fatalf("accepted invalid filter value %q", normalized)
		}
		for _, character := range normalized {
			if character == '"' ||
				character == '\\' ||
				unicode.IsControl(character) {
				t.Fatalf(
					"accepted forbidden character %U in %q",
					character,
					normalized,
				)
			}
		}
	})
}

func FuzzNewReference(fuzzer *testing.F) {
	for _, seed := range []struct {
		owner      string
		repository string
		number     int
	}{
		{owner: "octocat", repository: "hello-world", number: 1},
		{owner: "a", repository: ".github", number: 42},
		{owner: "bad--owner", repository: "repository", number: 1},
		{owner: "owner", repository: "..", number: 1},
		{owner: "owner", repository: "repository", number: 0},
	} {
		fuzzer.Add(seed.owner, seed.repository, seed.number)
	}

	fuzzer.Fuzz(func(t *testing.T, owner, repository string, number int) {
		reference, err := NewReference(owner, repository, number)
		if err != nil {
			return
		}
		if reference.Owner() != owner ||
			reference.RepositoryName() != repository ||
			reference.Number() != number ||
			!strings.HasPrefix(reference.CacheKey(), "github:issue-detail:") {
			t.Fatalf("reference did not preserve validated input: %+v", reference)
		}
		repeated, repeatedErr := NewReference(owner, repository, number)
		if repeatedErr != nil || repeated.CacheKey() != reference.CacheKey() {
			t.Fatalf(
				"reference key is not deterministic: %q, %q, %v",
				reference.CacheKey(),
				repeated.CacheKey(),
				repeatedErr,
			)
		}
	})
}
