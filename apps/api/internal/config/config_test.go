package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

var configurationKeys = []string{
	"PORT",
	"ALLOWED_ORIGINS",
	"GITHUB_API_BASE_URL",
	"GITHUB_TOKEN",
	"GITHUB_REQUEST_TIMEOUT",
	"GITHUB_API_MAX_CONCURRENCY",
	"PROFILE_ANALYSIS_REPOSITORY_LIMIT",
	"PROFILE_ANALYSIS_CACHE_TTL",
	"PROFILE_ANALYSIS_CACHE_CAPACITY",
	"ISSUE_SEARCH_RESULT_LIMIT",
	"ISSUE_SEARCH_CACHE_TTL",
	"ISSUE_SEARCH_CACHE_CAPACITY",
	"ISSUE_DETAIL_ANALYSIS_LIMIT",
	"MANIFEST_FILE_LIMIT",
	"USE_GITHUB_API_MOCK",
}

func TestLoadUsesSafeDefaults(t *testing.T) {
	clearConfiguration(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != defaultPort {
		t.Fatalf("Load().Port = %q, want %q", cfg.Port, defaultPort)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{defaultAllowedOrigins}) {
		t.Fatalf("Load().AllowedOrigins = %v", cfg.AllowedOrigins)
	}
	if cfg.GitHubAPIBaseURL.String() != defaultGitHubAPIBaseURL {
		t.Fatalf("Load().GitHubAPIBaseURL = %q", cfg.GitHubAPIBaseURL)
	}
	if cfg.GitHubRequestTimeout != defaultGitHubRequestTimeout {
		t.Fatalf("Load().GitHubRequestTimeout = %s", cfg.GitHubRequestTimeout)
	}
	if cfg.GitHubMaxConcurrency != defaultGitHubMaxConcurrency {
		t.Fatalf("Load().GitHubMaxConcurrency = %d", cfg.GitHubMaxConcurrency)
	}
	if cfg.ProfileAnalysisCacheTTL != defaultProfileAnalysisCacheTTL ||
		cfg.ProfileAnalysisCacheCapacity != defaultProfileAnalysisCacheCapacity {
		t.Fatalf("Load() profile cache defaults = %+v", cfg)
	}
	if cfg.IssueSearchCacheTTL != defaultIssueSearchCacheTTL ||
		cfg.IssueSearchCacheCapacity != defaultIssueSearchCacheCapacity {
		t.Fatalf("Load() issue search cache defaults = %+v", cfg)
	}
}

func TestLoadReadsConfiguredValues(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("PORT", "9090")
	t.Setenv(
		"ALLOWED_ORIGINS",
		"https://issuescout.example, http://localhost:5173,https://issuescout.example/",
	)
	t.Setenv("GITHUB_API_BASE_URL", "http://127.0.0.1:9000/")
	t.Setenv("GITHUB_TOKEN", "server-only-token")
	t.Setenv("GITHUB_REQUEST_TIMEOUT", "7s")
	t.Setenv("GITHUB_API_MAX_CONCURRENCY", "3")
	t.Setenv("PROFILE_ANALYSIS_REPOSITORY_LIMIT", "12")
	t.Setenv("PROFILE_ANALYSIS_CACHE_TTL", "45m")
	t.Setenv("PROFILE_ANALYSIS_CACHE_CAPACITY", "750")
	t.Setenv("ISSUE_SEARCH_RESULT_LIMIT", "30")
	t.Setenv("ISSUE_SEARCH_CACHE_TTL", "4m")
	t.Setenv("ISSUE_SEARCH_CACHE_CAPACITY", "900")
	t.Setenv("ISSUE_DETAIL_ANALYSIS_LIMIT", "10")
	t.Setenv("MANIFEST_FILE_LIMIT", "2")
	t.Setenv("USE_GITHUB_API_MOCK", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "9090" || cfg.GitHubToken != "server-only-token" {
		t.Fatalf("Load() did not preserve configured scalar values")
	}
	if cfg.GitHubRequestTimeout != 7*time.Second ||
		cfg.GitHubMaxConcurrency != 3 ||
		cfg.ProfileRepositoryLimit != 12 ||
		cfg.ProfileAnalysisCacheTTL != 45*time.Minute ||
		cfg.ProfileAnalysisCacheCapacity != 750 ||
		cfg.IssueSearchResultLimit != 30 ||
		cfg.IssueSearchCacheTTL != 4*time.Minute ||
		cfg.IssueSearchCacheCapacity != 900 ||
		cfg.IssueDetailAnalysisLimit != 10 ||
		cfg.ManifestFileLimit != 2 ||
		!cfg.UseGitHubAPIMock {
		t.Fatalf("Load() did not parse configured limits: %+v", cfg)
	}
	if got := cfg.GitHubAPIBaseURL.String(); got != "http://127.0.0.1:9000" {
		t.Fatalf("Load().GitHubAPIBaseURL = %q", got)
	}
	wantOrigins := []string{"https://issuescout.example", "http://localhost:5173"}
	if !reflect.DeepEqual(cfg.AllowedOrigins, wantOrigins) {
		t.Fatalf("Load().AllowedOrigins = %v, want %v", cfg.AllowedOrigins, wantOrigins)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		message  string
		preKey   string
		preValue string
	}{
		{name: "port text", key: "PORT", value: "api", message: "PORT"},
		{name: "port range", key: "PORT", value: "70000", message: "PORT"},
		{
			name:    "insecure public origin",
			key:     "ALLOWED_ORIGINS",
			value:   "http://issuescout.example",
			message: "ALLOWED_ORIGINS",
		},
		{
			name:    "origin path",
			key:     "ALLOWED_ORIGINS",
			value:   "https://issuescout.example/path",
			message: "ALLOWED_ORIGINS",
		},
		{
			name:    "insecure GitHub URL",
			key:     "GITHUB_API_BASE_URL",
			value:   "http://api.github.example",
			message: "GITHUB_API_BASE_URL",
		},
		{
			name:    "timeout",
			key:     "GITHUB_REQUEST_TIMEOUT",
			value:   "0s",
			message: "GITHUB_REQUEST_TIMEOUT",
		},
		{
			name:    "concurrency",
			key:     "GITHUB_API_MAX_CONCURRENCY",
			value:   "0",
			message: "GITHUB_API_MAX_CONCURRENCY",
		},
		{
			name:    "repository limit",
			key:     "PROFILE_ANALYSIS_REPOSITORY_LIMIT",
			value:   "21",
			message: "PROFILE_ANALYSIS_REPOSITORY_LIMIT",
		},
		{
			name:    "profile cache TTL",
			key:     "PROFILE_ANALYSIS_CACHE_TTL",
			value:   "25h",
			message: "PROFILE_ANALYSIS_CACHE_TTL",
		},
		{
			name:    "profile cache capacity",
			key:     "PROFILE_ANALYSIS_CACHE_CAPACITY",
			value:   "0",
			message: "PROFILE_ANALYSIS_CACHE_CAPACITY",
		},
		{
			name:    "search limit",
			key:     "ISSUE_SEARCH_RESULT_LIMIT",
			value:   "51",
			message: "ISSUE_SEARCH_RESULT_LIMIT",
		},
		{
			name:    "search cache TTL",
			key:     "ISSUE_SEARCH_CACHE_TTL",
			value:   "25h",
			message: "ISSUE_SEARCH_CACHE_TTL",
		},
		{
			name:    "search cache capacity",
			key:     "ISSUE_SEARCH_CACHE_CAPACITY",
			value:   "0",
			message: "ISSUE_SEARCH_CACHE_CAPACITY",
		},
		{
			name:     "detail exceeds search",
			key:      "ISSUE_DETAIL_ANALYSIS_LIMIT",
			value:    "21",
			message:  "ISSUE_DETAIL_ANALYSIS_LIMIT",
			preKey:   "ISSUE_SEARCH_RESULT_LIMIT",
			preValue: "20",
		},
		{
			name:    "manifest limit",
			key:     "MANIFEST_FILE_LIMIT",
			value:   "11",
			message: "MANIFEST_FILE_LIMIT",
		},
		{
			name:    "mock boolean",
			key:     "USE_GITHUB_API_MOCK",
			value:   "sometimes",
			message: "USE_GITHUB_API_MOCK",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConfiguration(t)
			if test.preKey != "" {
				t.Setenv(test.preKey, test.preValue)
			}
			t.Setenv(test.key, test.value)

			_, err := Load()
			if !errors.Is(err, errInvalidConfig) {
				t.Fatalf("Load() error = %v, want invalid configuration", err)
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Load() error = %q, want key %q", err, test.message)
			}
		})
	}
}

func clearConfiguration(t *testing.T) {
	t.Helper()
	for _, key := range configurationKeys {
		t.Setenv(key, "")
	}
}
