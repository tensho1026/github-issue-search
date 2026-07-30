package handler

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/transport/response"
)

type fixtureSuccessEnvelope[T any] struct {
	Data T             `json:"data"`
	Meta response.Meta `json:"meta"`
}

type fixtureErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta response.Meta `json:"meta"`
}

func TestContractFixturesDecodeIntoBackendResponseTypes(t *testing.T) {
	tests := []struct {
		file   string
		target any
	}{
		{
			file:   "health.success.json",
			target: &fixtureSuccessEnvelope[map[string]string]{},
		},
		{
			file:   "github-user.success.json",
			target: &fixtureSuccessEnvelope[githubUserResponse]{},
		},
		{
			file:   "profile-analysis.success.json",
			target: &fixtureSuccessEnvelope[githubProfileAnalysisResponse]{},
		},
		{
			file:   "issue-search.empty.json",
			target: &fixtureSuccessEnvelope[issueSearchResponse]{},
		},
		{
			file:   "issue-detail.success.json",
			target: &fixtureSuccessEnvelope[issueDetailResponse]{},
		},
		{
			file:   "rate-limit.error.json",
			target: &fixtureErrorEnvelope{},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			fixture, err := os.Open(contractFixturePath(t, test.file))
			if err != nil {
				t.Fatalf("open contract fixture: %v", err)
			}
			defer fixture.Close()

			decoder := json.NewDecoder(fixture)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(test.target); err != nil {
				t.Fatalf("decode contract fixture: %v", err)
			}
			if err := decoder.Decode(&struct{}{}); err != io.EOF {
				t.Fatalf("contract fixture has trailing JSON: %v", err)
			}
		})
	}
}

func contractFixturePath(t *testing.T, name string) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract fixture test path")
	}
	return filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"..",
		"..",
		"packages",
		"contracts",
		"fixtures",
		name,
	)
}
