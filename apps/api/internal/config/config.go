package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort                     = "8080"
	defaultAllowedOrigins           = "http://127.0.0.1:5173"
	defaultGitHubAPIBaseURL         = "https://api.github.com"
	defaultGitHubRequestTimeout     = 10 * time.Second
	defaultGitHubMaxConcurrency     = 5
	defaultProfileRepositoryLimit   = 20
	defaultIssueSearchResultLimit   = 50
	defaultIssueDetailAnalysisLimit = 20
	defaultManifestFileLimit        = 3
)

var errInvalidConfig = errors.New("invalid configuration")

// Config is the immutable process-level configuration assembled at startup.
// Secrets remain server-side and callers must never serialize this type.
type Config struct {
	Port                      string
	AllowedOrigins            []string
	GitHubToken               string
	GitHubAPIBaseURL          *url.URL
	GitHubRequestTimeout      time.Duration
	GitHubMaxConcurrency      int
	ProfileRepositoryLimit    int
	IssueSearchResultLimit    int
	IssueDetailAnalysisLimit  int
	ManifestFileLimit         int
	UseGitHubAPIMock          bool
	ReadHeaderTimeout         time.Duration
	ReadTimeout               time.Duration
	WriteTimeout              time.Duration
	IdleTimeout               time.Duration
	ShutdownTimeout           time.Duration
	NormalRequestTimeout      time.Duration
	ProfileRequestTimeout     time.Duration
	IssueSearchRequestTimeout time.Duration
}

// Load reads and validates all process configuration once. Optional values
// receive production-safe defaults; malformed values fail startup.
func Load() (Config, error) {
	port, err := parsePort(valueOrDefault("PORT", defaultPort))
	if err != nil {
		return Config{}, err
	}

	allowedOrigins, err := parseOrigins(
		valueOrDefault("ALLOWED_ORIGINS", defaultAllowedOrigins),
	)
	if err != nil {
		return Config{}, err
	}

	gitHubAPIBaseURL, err := parseBaseURL(
		valueOrDefault("GITHUB_API_BASE_URL", defaultGitHubAPIBaseURL),
	)
	if err != nil {
		return Config{}, err
	}

	gitHubRequestTimeout, err := parseDuration(
		"GITHUB_REQUEST_TIMEOUT",
		defaultGitHubRequestTimeout,
	)
	if err != nil {
		return Config{}, err
	}

	gitHubMaxConcurrency, err := parseInt(
		"GITHUB_API_MAX_CONCURRENCY",
		defaultGitHubMaxConcurrency,
		1,
		20,
	)
	if err != nil {
		return Config{}, err
	}

	profileRepositoryLimit, err := parseInt(
		"PROFILE_ANALYSIS_REPOSITORY_LIMIT",
		defaultProfileRepositoryLimit,
		1,
		20,
	)
	if err != nil {
		return Config{}, err
	}

	issueSearchResultLimit, err := parseInt(
		"ISSUE_SEARCH_RESULT_LIMIT",
		defaultIssueSearchResultLimit,
		1,
		50,
	)
	if err != nil {
		return Config{}, err
	}

	issueDetailAnalysisLimit, err := parseInt(
		"ISSUE_DETAIL_ANALYSIS_LIMIT",
		defaultIssueDetailAnalysisLimit,
		1,
		issueSearchResultLimit,
	)
	if err != nil {
		return Config{}, err
	}

	manifestFileLimit, err := parseInt(
		"MANIFEST_FILE_LIMIT",
		defaultManifestFileLimit,
		1,
		10,
	)
	if err != nil {
		return Config{}, err
	}

	useGitHubAPIMock, err := parseBool("USE_GITHUB_API_MOCK", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:                      port,
		AllowedOrigins:            allowedOrigins,
		GitHubToken:               os.Getenv("GITHUB_TOKEN"),
		GitHubAPIBaseURL:          gitHubAPIBaseURL,
		GitHubRequestTimeout:      gitHubRequestTimeout,
		GitHubMaxConcurrency:      gitHubMaxConcurrency,
		ProfileRepositoryLimit:    profileRepositoryLimit,
		IssueSearchResultLimit:    issueSearchResultLimit,
		IssueDetailAnalysisLimit:  issueDetailAnalysisLimit,
		ManifestFileLimit:         manifestFileLimit,
		UseGitHubAPIMock:          useGitHubAPIMock,
		ReadHeaderTimeout:         5 * time.Second,
		ReadTimeout:               20 * time.Second,
		WriteTimeout:              20 * time.Second,
		IdleTimeout:               60 * time.Second,
		ShutdownTimeout:           10 * time.Second,
		NormalRequestTimeout:      5 * time.Second,
		ProfileRequestTimeout:     15 * time.Second,
		IssueSearchRequestTimeout: 15 * time.Second,
	}, nil
}

func parsePort(raw string) (string, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return "", configError("PORT", "must be an integer between 1 and 65535")
	}

	return strconv.Itoa(port), nil
}

func parseOrigins(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		origin := strings.TrimSpace(part)
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Path != "" && parsed.Path != "/") {
			return nil, configError(
				"ALLOWED_ORIGINS",
				"must contain comma-separated HTTP(S) origins without paths",
			)
		}
		if parsed.Scheme != "https" && !isLoopbackHTTP(parsed) {
			return nil, configError(
				"ALLOWED_ORIGINS",
				"must use HTTPS unless the host is loopback",
			)
		}
		normalized := strings.TrimSuffix(parsed.String(), "/")
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		origins = append(origins, normalized)
	}

	if len(origins) == 0 {
		return nil, configError("ALLOWED_ORIGINS", "must contain at least one origin")
	}

	return origins, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSuffix(strings.TrimSpace(raw), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, configError("GITHUB_API_BASE_URL", "must be a valid base URL")
	}
	if parsed.Scheme != "https" && !isLoopbackHTTP(parsed) {
		return nil, configError(
			"GITHUB_API_BASE_URL",
			"must use HTTPS unless the host is loopback",
		)
	}

	return parsed, nil
}

func isLoopbackHTTP(parsed *url.URL) bool {
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 || value > time.Minute {
		return 0, configError(key, "must be a positive duration no greater than 1m")
	}

	return value, nil
}

func parseInt(key string, fallback, minimum, maximum int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, configError(
			key,
			fmt.Sprintf("must be an integer between %d and %d", minimum, maximum),
		)
	}

	return value, nil
}

func parseBool(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, configError(key, "must be true or false")
	}

	return value, nil
}

func configError(key, requirement string) error {
	return fmt.Errorf("%w: %s %s", errInvalidConfig, key, requirement)
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
