package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var configurationKeys = []string{
	"APP_ENV",
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
	"ISSUE_DETAIL_CACHE_TTL",
	"ISSUE_DETAIL_CACHE_CAPACITY",
	"REPOSITORY_DISCOVERY_RESULT_LIMIT",
	"REPOSITORY_DISCOVERY_ENRICHMENT_LIMIT",
	"REPOSITORY_DISCOVERY_CACHE_TTL",
	"REPOSITORY_DISCOVERY_CACHE_CAPACITY",
	"MANIFEST_FILE_LIMIT",
	"DATABASE_URL",
	"DATABASE_MAX_CONNECTIONS",
	"DATABASE_MIN_CONNECTIONS",
	"DATABASE_CONNECT_TIMEOUT",
	"DATABASE_QUERY_TIMEOUT",
	"DATABASE_MAX_CONNECTION_LIFETIME",
	"DATABASE_MAX_CONNECTION_IDLE_TIME",
	"DATABASE_HEALTH_CHECK_PERIOD",
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
	if cfg.AppEnvironment != defaultAppEnvironment {
		t.Fatalf(
			"Load().AppEnvironment = %q, want %q",
			cfg.AppEnvironment,
			defaultAppEnvironment,
		)
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
	if cfg.IssueDetailCacheTTL != defaultIssueDetailCacheTTL ||
		cfg.IssueDetailCacheCapacity != defaultIssueDetailCacheCapacity {
		t.Fatalf("Load() issue detail cache defaults = %+v", cfg)
	}
	if cfg.RepositoryDiscoveryResultLimit !=
		defaultRepositoryDiscoveryResultLimit ||
		cfg.RepositoryDiscoveryEnrichmentLimit !=
			defaultRepositoryDiscoveryEnrichmentLimit ||
		cfg.RepositoryDiscoveryCacheTTL !=
			defaultRepositoryDiscoveryCacheTTL ||
		cfg.RepositoryDiscoveryCacheCapacity !=
			defaultRepositoryDiscoveryCacheCapacity {
		t.Fatalf("Load() repository discovery defaults = %+v", cfg)
	}
	if cfg.DatabaseURL.IsSet() ||
		cfg.DatabaseMaxConnections != defaultDatabaseMaxConnections ||
		cfg.DatabaseMinConnections != defaultDatabaseMinConnections ||
		cfg.DatabaseConnectTimeout != defaultDatabaseConnectTimeout ||
		cfg.DatabaseQueryTimeout != defaultDatabaseQueryTimeout ||
		cfg.DatabaseMaxConnectionLifetime !=
			defaultDatabaseMaxConnectionLifetime ||
		cfg.DatabaseMaxConnectionIdleTime !=
			defaultDatabaseMaxConnectionIdleTime ||
		cfg.DatabaseHealthCheckPeriod != defaultDatabaseHealthCheckPeriod {
		t.Fatal("Load() did not apply database-safe defaults")
	}
}

func TestLoadReadsConfiguredValues(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("APP_ENV", "test")
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
	t.Setenv("ISSUE_DETAIL_CACHE_TTL", "3m")
	t.Setenv("ISSUE_DETAIL_CACHE_CAPACITY", "450")
	t.Setenv("REPOSITORY_DISCOVERY_RESULT_LIMIT", "40")
	t.Setenv("REPOSITORY_DISCOVERY_ENRICHMENT_LIMIT", "12")
	t.Setenv("REPOSITORY_DISCOVERY_CACHE_TTL", "2m")
	t.Setenv("REPOSITORY_DISCOVERY_CACHE_CAPACITY", "700")
	t.Setenv("MANIFEST_FILE_LIMIT", "2")
	databaseURL := fmt.Sprintf(
		"postgresql://owner:%s@db.example/issuescout?sslmode=require",
		"configuration-test-value",
	)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("DATABASE_MAX_CONNECTIONS", "12")
	t.Setenv("DATABASE_MIN_CONNECTIONS", "2")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "4s")
	t.Setenv("DATABASE_QUERY_TIMEOUT", "3s")
	t.Setenv("DATABASE_MAX_CONNECTION_LIFETIME", "20m")
	t.Setenv("DATABASE_MAX_CONNECTION_IDLE_TIME", "4m")
	t.Setenv("DATABASE_HEALTH_CHECK_PERIOD", "20s")
	t.Setenv("USE_GITHUB_API_MOCK", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnvironment != "test" ||
		cfg.Port != "9090" ||
		cfg.GitHubToken.Value() != "server-only-token" ||
		cfg.DatabaseURL.Value() != databaseURL {
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
		cfg.IssueDetailCacheTTL != 3*time.Minute ||
		cfg.IssueDetailCacheCapacity != 450 ||
		cfg.RepositoryDiscoveryResultLimit != 40 ||
		cfg.RepositoryDiscoveryEnrichmentLimit != 12 ||
		cfg.RepositoryDiscoveryCacheTTL != 2*time.Minute ||
		cfg.RepositoryDiscoveryCacheCapacity != 700 ||
		cfg.ManifestFileLimit != 2 ||
		cfg.DatabaseMaxConnections != 12 ||
		cfg.DatabaseMinConnections != 2 ||
		cfg.DatabaseConnectTimeout != 4*time.Second ||
		cfg.DatabaseQueryTimeout != 3*time.Second ||
		cfg.DatabaseMaxConnectionLifetime != 20*time.Minute ||
		cfg.DatabaseMaxConnectionIdleTime != 4*time.Minute ||
		cfg.DatabaseHealthCheckPeriod != 20*time.Second ||
		!cfg.UseGitHubAPIMock {
		t.Fatal("Load() did not parse configured limits")
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
		{
			name:    "application environment",
			key:     "APP_ENV",
			value:   "preview",
			message: "APP_ENV",
		},
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
			name:    "detail cache TTL",
			key:     "ISSUE_DETAIL_CACHE_TTL",
			value:   "25h",
			message: "ISSUE_DETAIL_CACHE_TTL",
		},
		{
			name:    "detail cache capacity",
			key:     "ISSUE_DETAIL_CACHE_CAPACITY",
			value:   "0",
			message: "ISSUE_DETAIL_CACHE_CAPACITY",
		},
		{
			name:    "repository discovery result limit",
			key:     "REPOSITORY_DISCOVERY_RESULT_LIMIT",
			value:   "51",
			message: "REPOSITORY_DISCOVERY_RESULT_LIMIT",
		},
		{
			name:     "repository enrichment exceeds result limit",
			key:      "REPOSITORY_DISCOVERY_ENRICHMENT_LIMIT",
			value:    "11",
			message:  "REPOSITORY_DISCOVERY_ENRICHMENT_LIMIT",
			preKey:   "REPOSITORY_DISCOVERY_RESULT_LIMIT",
			preValue: "10",
		},
		{
			name:    "repository discovery cache TTL",
			key:     "REPOSITORY_DISCOVERY_CACHE_TTL",
			value:   "25h",
			message: "REPOSITORY_DISCOVERY_CACHE_TTL",
		},
		{
			name:    "repository discovery cache capacity",
			key:     "REPOSITORY_DISCOVERY_CACHE_CAPACITY",
			value:   "0",
			message: "REPOSITORY_DISCOVERY_CACHE_CAPACITY",
		},
		{
			name:    "manifest limit",
			key:     "MANIFEST_FILE_LIMIT",
			value:   "11",
			message: "MANIFEST_FILE_LIMIT",
		},
		{
			name:    "database URL scheme",
			key:     "DATABASE_URL",
			value:   "https://db.example/issuescout?sslmode=require",
			message: "DATABASE_URL",
		},
		{
			name: "database URL TLS",
			key:  "DATABASE_URL",
			value: fmt.Sprintf(
				"postgresql://owner:%s@db.example/issuescout?sslmode=disable",
				"configuration-test-value",
			),
			message: "DATABASE_URL",
		},
		{
			name:    "database maximum connections",
			key:     "DATABASE_MAX_CONNECTIONS",
			value:   "101",
			message: "DATABASE_MAX_CONNECTIONS",
		},
		{
			name:     "database minimum exceeds maximum",
			key:      "DATABASE_MIN_CONNECTIONS",
			value:    "4",
			message:  "DATABASE_MIN_CONNECTIONS",
			preKey:   "DATABASE_MAX_CONNECTIONS",
			preValue: "3",
		},
		{
			name:    "database query timeout",
			key:     "DATABASE_QUERY_TIMEOUT",
			value:   "0s",
			message: "DATABASE_QUERY_TIMEOUT",
		},
		{
			name:    "mock boolean",
			key:     "USE_GITHUB_API_MOCK",
			value:   "sometimes",
			message: "USE_GITHUB_API_MOCK",
		},
		{
			name:     "mock outside test environment",
			key:      "USE_GITHUB_API_MOCK",
			value:    "true",
			message:  "USE_GITHUB_API_MOCK",
			preKey:   "APP_ENV",
			preValue: "production",
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

func TestSecretsNeverFormatTheirValues(t *testing.T) {
	clearConfiguration(t)
	//nolint:gosec // This is a synthetic sentinel used only to prove redaction.
	const token = "github-configuration-sensitive-value"
	const databasePassword = "database-configuration-sensitive-value"
	t.Setenv("GITHUB_TOKEN", token)
	t.Setenv(
		"DATABASE_URL",
		fmt.Sprintf(
			"postgresql://owner:%s@db.example/issuescout?sslmode=verify-full",
			databasePassword,
		),
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	formatted := fmt.Sprintf("%+v %#v %s %s", cfg, cfg, cfg.GitHubToken, cfg.DatabaseURL)
	if strings.Contains(formatted, token) ||
		strings.Contains(formatted, databasePassword) {
		t.Fatal("formatted configuration exposed a secret")
	}
	if !strings.Contains(formatted, "<redacted>") {
		t.Fatal("formatted configuration did not identify redacted values")
	}
}

func clearConfiguration(t *testing.T) {
	t.Helper()
	for _, key := range configurationKeys {
		t.Setenv(key, "")
	}
}
