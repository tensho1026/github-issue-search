package repository

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func FuzzParseDiscoveryFilterValue(fuzzer *testing.F) {
	for _, seed := range []string{
		"TypeScript",
		" machine learning ",
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
			utf8.RuneCountInString(normalized) >
				MaximumDiscoveryFilterValueRunes ||
			len(normalized) > MaximumDiscoveryFilterValueBytes {
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
