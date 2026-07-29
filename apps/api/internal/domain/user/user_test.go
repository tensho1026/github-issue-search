package user

import (
	"errors"
	"strings"
	"testing"
)

func TestParseUsername(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Username
		wantErr bool
	}{
		{name: "simple", raw: "tensho1026", want: "tensho1026"},
		{name: "hyphen", raw: "issue-scout", want: "issue-scout"},
		{name: "trim spaces", raw: "  octocat  ", want: "octocat"},
		{name: "empty", raw: "", wantErr: true},
		{name: "too long", raw: strings.Repeat("a", 40), wantErr: true},
		{name: "leading hyphen", raw: "-octocat", wantErr: true},
		{name: "trailing hyphen", raw: "octocat-", wantErr: true},
		{name: "consecutive hyphens", raw: "issue--scout", wantErr: true},
		{name: "underscore", raw: "issue_scout", wantErr: true},
		{name: "non ASCII", raw: "イシュー", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseUsername(test.raw)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidUsername) {
					t.Fatalf("ParseUsername() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUsername() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ParseUsername() = %q, want %q", got, test.want)
			}
		})
	}
}
