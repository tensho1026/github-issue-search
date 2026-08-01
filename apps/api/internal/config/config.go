package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/issue"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

const (
	defaultAppEnvironment                     = "development"
	defaultPort                               = "8080"
	defaultAllowedOrigins                     = "http://127.0.0.1:5173"
	defaultGitHubAPIBaseURL                   = "https://api.github.com"
	defaultGitHubRequestTimeout               = 10 * time.Second
	defaultGitHubMaxConcurrency               = 5
	defaultProfileRepositoryLimit             = 20
	defaultProfileAnalysisCacheTTL            = 30 * time.Minute
	defaultProfileAnalysisCacheCapacity       = 500
	defaultIssueSearchResultLimit             = issue.MaximumCandidateResults
	defaultIssueSearchCacheTTL                = 5 * time.Minute
	defaultIssueSearchCacheCapacity           = 1000
	defaultIssueDetailAnalysisLimit           = 20
	defaultIssueDetailCacheTTL                = 5 * time.Minute
	defaultIssueDetailCacheCapacity           = 500
	defaultRepositoryDiscoveryResultLimit     = repository.MaximumDiscoveryCandidateResults
	defaultRepositoryDiscoveryEnrichmentLimit = 10
	defaultRepositoryDiscoveryCacheTTL        = 5 * time.Minute
	defaultRepositoryDiscoveryCacheCapacity   = 1000
	defaultManifestFileLimit                  = 3
	defaultDatabaseMaxConnections             = 10
	defaultDatabaseMinConnections             = 0
	defaultDatabaseConnectTimeout             = 5 * time.Second
	defaultDatabaseQueryTimeout               = 5 * time.Second
	defaultDatabaseMaxConnectionLifetime      = 30 * time.Minute
	defaultDatabaseMaxConnectionIdleTime      = 5 * time.Minute
	defaultDatabaseHealthCheckPeriod          = 30 * time.Second
	defaultGitHubOAuthAuthorizeURL            = "https://github.com/login/oauth/authorize"
	//nolint:gosec // This is GitHub's public token-exchange endpoint, not a credential.
	defaultGitHubOAuthTokenURL = "https://github.com/login/oauth/access_token"
	defaultAuthStateTTL        = 10 * time.Minute
	defaultAuthSessionTTL      = 12 * time.Hour
	defaultAuthMaxSessions     = 10
)

var errInvalidConfig = errors.New("invalid configuration")

// Secret contains a server-only configuration value without exposing it
// through formatting or JSON serialization.
type Secret struct {
	value string
}

// Value returns the secret for the narrow adapter boundary that consumes it.
// Callers must never log, serialize, or include the result in an error.
func (secret Secret) Value() string {
	return secret.value
}

// IsSet reports whether the environment supplied a non-empty value.
func (secret Secret) IsSet() bool {
	return secret.value != ""
}

// String implements fmt.Stringer with a deliberately non-sensitive value.
func (secret Secret) String() string {
	if !secret.IsSet() {
		return "<unset>"
	}

	return "<redacted>"
}

// GoString prevents %#v formatting from revealing the underlying value.
func (secret Secret) GoString() string {
	return secret.String()
}

// Config is the immutable process-level configuration assembled at startup.
// Secrets remain server-side and callers must never serialize this type.
type Config struct {
	AppEnvironment                     string
	Port                               string
	AllowedOrigins                     []string
	GitHubToken                        Secret
	GitHubAPIBaseURL                   *url.URL
	GitHubRequestTimeout               time.Duration
	GitHubMaxConcurrency               int
	ProfileRepositoryLimit             int
	ProfileAnalysisCacheTTL            time.Duration
	ProfileAnalysisCacheCapacity       int
	IssueSearchResultLimit             int
	IssueSearchCacheTTL                time.Duration
	IssueSearchCacheCapacity           int
	IssueDetailAnalysisLimit           int
	IssueDetailCacheTTL                time.Duration
	IssueDetailCacheCapacity           int
	RepositoryDiscoveryResultLimit     int
	RepositoryDiscoveryEnrichmentLimit int
	RepositoryDiscoveryCacheTTL        time.Duration
	RepositoryDiscoveryCacheCapacity   int
	ManifestFileLimit                  int
	DatabaseURL                        Secret
	DatabaseMaxConnections             int
	DatabaseMinConnections             int
	DatabaseConnectTimeout             time.Duration
	DatabaseQueryTimeout               time.Duration
	DatabaseMaxConnectionLifetime      time.Duration
	DatabaseMaxConnectionIdleTime      time.Duration
	DatabaseHealthCheckPeriod          time.Duration
	AuthEnabled                        bool
	GitHubOAuthClientID                string
	GitHubOAuthClientSecret            Secret
	GitHubOAuthAuthorizeURL            *url.URL
	GitHubOAuthTokenURL                *url.URL
	GitHubOAuthCallbackURL             *url.URL
	AuthFrontendURL                    *url.URL
	AuthFlowEncryptionKey              Secret
	AuthStateTTL                       time.Duration
	AuthSessionTTL                     time.Duration
	AuthMaxSessions                    int
	AuthCookieSecure                   bool
	TrustedProxyCIDRs                  []string
	UseGitHubAPIMock                   bool
	ReadHeaderTimeout                  time.Duration
	ReadTimeout                        time.Duration
	WriteTimeout                       time.Duration
	IdleTimeout                        time.Duration
	ShutdownTimeout                    time.Duration
	NormalRequestTimeout               time.Duration
	ProfileRequestTimeout              time.Duration
	IssueSearchRequestTimeout          time.Duration
	IssueDetailRequestTimeout          time.Duration
	RepositoryDiscoveryRequestTimeout  time.Duration
}

// Load reads and validates all process configuration once. Optional values
// receive production-safe defaults; malformed values fail startup.
func Load() (Config, error) {
	appEnvironment, err := parseAppEnvironment(
		valueOrDefault("APP_ENV", defaultAppEnvironment),
	)
	if err != nil {
		return Config{}, err
	}

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
		time.Minute,
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

	profileAnalysisCacheTTL, err := parseDuration(
		"PROFILE_ANALYSIS_CACHE_TTL",
		defaultProfileAnalysisCacheTTL,
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}

	profileCacheCapacity, err := parseInt(
		"PROFILE_ANALYSIS_CACHE_CAPACITY",
		defaultProfileAnalysisCacheCapacity,
		1,
		10_000,
	)
	if err != nil {
		return Config{}, err
	}

	issueSearchResultLimit, err := parseInt(
		"ISSUE_SEARCH_RESULT_LIMIT",
		defaultIssueSearchResultLimit,
		1,
		issue.MaximumCandidateResults,
	)
	if err != nil {
		return Config{}, err
	}

	issueSearchCacheTTL, err := parseDuration(
		"ISSUE_SEARCH_CACHE_TTL",
		defaultIssueSearchCacheTTL,
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}

	issueSearchCacheCapacity, err := parseInt(
		"ISSUE_SEARCH_CACHE_CAPACITY",
		defaultIssueSearchCacheCapacity,
		1,
		10_000,
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

	issueDetailCacheTTL, err := parseDuration(
		"ISSUE_DETAIL_CACHE_TTL",
		defaultIssueDetailCacheTTL,
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}

	issueDetailCacheCapacity, err := parseInt(
		"ISSUE_DETAIL_CACHE_CAPACITY",
		defaultIssueDetailCacheCapacity,
		1,
		10_000,
	)
	if err != nil {
		return Config{}, err
	}

	repositoryDiscoveryResultLimit, err := parseInt(
		"REPOSITORY_DISCOVERY_RESULT_LIMIT",
		defaultRepositoryDiscoveryResultLimit,
		1,
		repository.MaximumDiscoveryCandidateResults,
	)
	if err != nil {
		return Config{}, err
	}
	repositoryDiscoveryEnrichmentLimit, err := parseInt(
		"REPOSITORY_DISCOVERY_ENRICHMENT_LIMIT",
		defaultRepositoryDiscoveryEnrichmentLimit,
		1,
		min(
			repositoryDiscoveryResultLimit,
			repository.MaximumDiscoveryEnrichmentResults,
		),
	)
	if err != nil {
		return Config{}, err
	}
	repositoryDiscoveryCacheTTL, err := parseDuration(
		"REPOSITORY_DISCOVERY_CACHE_TTL",
		defaultRepositoryDiscoveryCacheTTL,
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}
	repositoryDiscoveryCacheCapacity, err := parseInt(
		"REPOSITORY_DISCOVERY_CACHE_CAPACITY",
		defaultRepositoryDiscoveryCacheCapacity,
		1,
		10_000,
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

	databaseURL, err := parseDatabaseURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		return Config{}, err
	}
	databaseMaxConnections, err := parseInt(
		"DATABASE_MAX_CONNECTIONS",
		defaultDatabaseMaxConnections,
		1,
		100,
	)
	if err != nil {
		return Config{}, err
	}
	databaseMinConnections, err := parseInt(
		"DATABASE_MIN_CONNECTIONS",
		defaultDatabaseMinConnections,
		0,
		databaseMaxConnections,
	)
	if err != nil {
		return Config{}, err
	}
	databaseConnectTimeout, err := parseDuration(
		"DATABASE_CONNECT_TIMEOUT",
		defaultDatabaseConnectTimeout,
		time.Minute,
	)
	if err != nil {
		return Config{}, err
	}
	databaseQueryTimeout, err := parseDuration(
		"DATABASE_QUERY_TIMEOUT",
		defaultDatabaseQueryTimeout,
		time.Minute,
	)
	if err != nil {
		return Config{}, err
	}
	databaseMaxConnectionLifetime, err := parseDuration(
		"DATABASE_MAX_CONNECTION_LIFETIME",
		defaultDatabaseMaxConnectionLifetime,
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}
	databaseMaxConnectionIdleTime, err := parseDuration(
		"DATABASE_MAX_CONNECTION_IDLE_TIME",
		defaultDatabaseMaxConnectionIdleTime,
		24*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}
	databaseHealthCheckPeriod, err := parseDuration(
		"DATABASE_HEALTH_CHECK_PERIOD",
		defaultDatabaseHealthCheckPeriod,
		time.Hour,
	)
	if err != nil {
		return Config{}, err
	}
	authConfiguration, err := parseAuthConfiguration(
		appEnvironment,
		allowedOrigins,
		databaseURL != "",
	)
	if err != nil {
		return Config{}, err
	}
	trustedProxyCIDRs, err := parseTrustedProxyCIDRs(
		os.Getenv("TRUSTED_PROXY_CIDRS"),
	)
	if err != nil {
		return Config{}, err
	}

	useGitHubAPIMock, err := parseBool("USE_GITHUB_API_MOCK", false)
	if err != nil {
		return Config{}, err
	}
	if useGitHubAPIMock && appEnvironment != "test" {
		return Config{}, configError(
			"USE_GITHUB_API_MOCK",
			"can be enabled only when APP_ENV=test",
		)
	}

	return Config{
		AppEnvironment:                     appEnvironment,
		Port:                               port,
		AllowedOrigins:                     allowedOrigins,
		GitHubToken:                        Secret{value: os.Getenv("GITHUB_TOKEN")},
		GitHubAPIBaseURL:                   gitHubAPIBaseURL,
		GitHubRequestTimeout:               gitHubRequestTimeout,
		GitHubMaxConcurrency:               gitHubMaxConcurrency,
		ProfileRepositoryLimit:             profileRepositoryLimit,
		ProfileAnalysisCacheTTL:            profileAnalysisCacheTTL,
		ProfileAnalysisCacheCapacity:       profileCacheCapacity,
		IssueSearchResultLimit:             issueSearchResultLimit,
		IssueSearchCacheTTL:                issueSearchCacheTTL,
		IssueSearchCacheCapacity:           issueSearchCacheCapacity,
		IssueDetailAnalysisLimit:           issueDetailAnalysisLimit,
		IssueDetailCacheTTL:                issueDetailCacheTTL,
		IssueDetailCacheCapacity:           issueDetailCacheCapacity,
		RepositoryDiscoveryResultLimit:     repositoryDiscoveryResultLimit,
		RepositoryDiscoveryEnrichmentLimit: repositoryDiscoveryEnrichmentLimit,
		RepositoryDiscoveryCacheTTL:        repositoryDiscoveryCacheTTL,
		RepositoryDiscoveryCacheCapacity:   repositoryDiscoveryCacheCapacity,
		ManifestFileLimit:                  manifestFileLimit,
		DatabaseURL:                        Secret{value: databaseURL},
		DatabaseMaxConnections:             databaseMaxConnections,
		DatabaseMinConnections:             databaseMinConnections,
		DatabaseConnectTimeout:             databaseConnectTimeout,
		DatabaseQueryTimeout:               databaseQueryTimeout,
		DatabaseMaxConnectionLifetime:      databaseMaxConnectionLifetime,
		DatabaseMaxConnectionIdleTime:      databaseMaxConnectionIdleTime,
		DatabaseHealthCheckPeriod:          databaseHealthCheckPeriod,
		AuthEnabled:                        authConfiguration.enabled,
		GitHubOAuthClientID:                authConfiguration.clientID,
		GitHubOAuthClientSecret:            authConfiguration.clientSecret,
		GitHubOAuthAuthorizeURL:            authConfiguration.authorizeURL,
		GitHubOAuthTokenURL:                authConfiguration.tokenURL,
		GitHubOAuthCallbackURL:             authConfiguration.callbackURL,
		AuthFrontendURL:                    authConfiguration.frontendURL,
		AuthFlowEncryptionKey:              authConfiguration.flowEncryptionKey,
		AuthStateTTL:                       authConfiguration.stateTTL,
		AuthSessionTTL:                     authConfiguration.sessionTTL,
		AuthMaxSessions:                    authConfiguration.maxSessions,
		AuthCookieSecure:                   authConfiguration.cookieSecure,
		TrustedProxyCIDRs:                  trustedProxyCIDRs,
		UseGitHubAPIMock:                   useGitHubAPIMock,
		ReadHeaderTimeout:                  5 * time.Second,
		ReadTimeout:                        20 * time.Second,
		WriteTimeout:                       20 * time.Second,
		IdleTimeout:                        60 * time.Second,
		ShutdownTimeout:                    10 * time.Second,
		NormalRequestTimeout:               5 * time.Second,
		ProfileRequestTimeout:              15 * time.Second,
		IssueSearchRequestTimeout:          15 * time.Second,
		IssueDetailRequestTimeout:          15 * time.Second,
		RepositoryDiscoveryRequestTimeout:  15 * time.Second,
	}, nil
}

type authConfiguration struct {
	enabled           bool
	clientID          string
	clientSecret      Secret
	authorizeURL      *url.URL
	tokenURL          *url.URL
	callbackURL       *url.URL
	frontendURL       *url.URL
	flowEncryptionKey Secret
	stateTTL          time.Duration
	sessionTTL        time.Duration
	maxSessions       int
	cookieSecure      bool
}

func parseAuthConfiguration(
	appEnvironment string,
	allowedOrigins []string,
	databaseConfigured bool,
) (authConfiguration, error) {
	stateTTL, err := parseDuration(
		"AUTH_STATE_TTL",
		defaultAuthStateTTL,
		15*time.Minute,
	)
	if err != nil {
		return authConfiguration{}, err
	}
	sessionTTL, err := parseDuration(
		"AUTH_SESSION_TTL",
		defaultAuthSessionTTL,
		7*24*time.Hour,
	)
	if err != nil {
		return authConfiguration{}, err
	}
	maxSessions, err := parseInt(
		"AUTH_MAX_SESSIONS",
		defaultAuthMaxSessions,
		1,
		50,
	)
	if err != nil {
		return authConfiguration{}, err
	}
	secureDefault := appEnvironment != "development" && appEnvironment != "test"
	cookieSecure, err := parseBool("AUTH_COOKIE_SECURE", secureDefault)
	if err != nil {
		return authConfiguration{}, err
	}
	if !cookieSecure &&
		appEnvironment != "development" &&
		appEnvironment != "test" {
		return authConfiguration{}, configError(
			"AUTH_COOKIE_SECURE",
			"must be true outside development and test",
		)
	}

	rawValues := map[string]string{
		"GITHUB_OAUTH_CLIENT_ID":     os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		"GITHUB_OAUTH_CLIENT_SECRET": os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		"GITHUB_OAUTH_CALLBACK_URL":  os.Getenv("GITHUB_OAUTH_CALLBACK_URL"),
		"AUTH_FRONTEND_URL":          os.Getenv("AUTH_FRONTEND_URL"),
		"AUTH_FLOW_ENCRYPTION_KEY":   os.Getenv("AUTH_FLOW_ENCRYPTION_KEY"),
	}
	configuredValues := 0
	for _, value := range rawValues {
		if value != "" {
			configuredValues++
		}
	}
	base := authConfiguration{
		stateTTL:     stateTTL,
		sessionTTL:   sessionTTL,
		maxSessions:  maxSessions,
		cookieSecure: cookieSecure,
	}
	if configuredValues == 0 {
		return base, nil
	}
	if configuredValues != len(rawValues) {
		return authConfiguration{}, configError(
			"GITHUB_OAUTH_CLIENT_ID",
			"and all OAuth/authentication secrets and URLs must be supplied together",
		)
	}
	if !databaseConfigured {
		return authConfiguration{}, configError(
			"DATABASE_URL",
			"is required when GitHub OAuth is enabled",
		)
	}
	if clientID := rawValues["GITHUB_OAUTH_CLIENT_ID"]; len(clientID) > 255 ||
		strings.TrimSpace(clientID) != clientID ||
		strings.ContainsAny(clientID, " \t\r\n") {
		return authConfiguration{}, configError(
			"GITHUB_OAUTH_CLIENT_ID",
			"must be a non-empty GitHub client identifier",
		)
	}
	if len(rawValues["GITHUB_OAUTH_CLIENT_SECRET"]) < 20 {
		return authConfiguration{}, configError(
			"GITHUB_OAUTH_CLIENT_SECRET",
			"must contain at least 20 characters",
		)
	}
	encryptionKey := rawValues["AUTH_FLOW_ENCRYPTION_KEY"]
	decodedKey, decodeErr := hex.DecodeString(encryptionKey)
	if decodeErr != nil || len(decodedKey) != 32 ||
		encryptionKey != strings.ToLower(encryptionKey) {
		return authConfiguration{}, configError(
			"AUTH_FLOW_ENCRYPTION_KEY",
			"must be exactly 64 lower-case hexadecimal characters",
		)
	}
	callbackURL, err := parseSecureURL(
		"GITHUB_OAUTH_CALLBACK_URL",
		rawValues["GITHUB_OAUTH_CALLBACK_URL"],
	)
	if err != nil {
		return authConfiguration{}, err
	}
	if callbackURL.Path != "/api/auth/github/callback" ||
		callbackURL.RawQuery != "" {
		return authConfiguration{}, configError(
			"GITHUB_OAUTH_CALLBACK_URL",
			"must use the exact /api/auth/github/callback path without a query",
		)
	}
	frontendOrigins, err := parseOrigins(rawValues["AUTH_FRONTEND_URL"])
	if err != nil || len(frontendOrigins) != 1 {
		return authConfiguration{}, configError(
			"AUTH_FRONTEND_URL",
			"must be exactly one allowed HTTP(S) origin",
		)
	}
	if !containsString(allowedOrigins, frontendOrigins[0]) {
		return authConfiguration{}, configError(
			"AUTH_FRONTEND_URL",
			"must also be present in ALLOWED_ORIGINS",
		)
	}
	frontendURL, parseErr := url.Parse(frontendOrigins[0])
	if parseErr != nil {
		return authConfiguration{}, configError(
			"AUTH_FRONTEND_URL",
			"must be a valid origin",
		)
	}
	if !cookieSecure &&
		(!isLoopbackHTTP(callbackURL) || !isLoopbackHTTP(frontendURL)) {
		return authConfiguration{}, configError(
			"AUTH_COOKIE_SECURE",
			"can be false only when OAuth URLs use loopback HTTP",
		)
	}
	authorizeURL, err := parseSecureURL(
		"GITHUB_OAUTH_AUTHORIZE_URL",
		valueOrDefault(
			"GITHUB_OAUTH_AUTHORIZE_URL",
			defaultGitHubOAuthAuthorizeURL,
		),
	)
	if err != nil {
		return authConfiguration{}, err
	}
	tokenURL, err := parseSecureURL(
		"GITHUB_OAUTH_TOKEN_URL",
		valueOrDefault("GITHUB_OAUTH_TOKEN_URL", defaultGitHubOAuthTokenURL),
	)
	if err != nil {
		return authConfiguration{}, err
	}

	base.enabled = true
	base.clientID = rawValues["GITHUB_OAUTH_CLIENT_ID"]
	base.clientSecret = Secret{
		value: rawValues["GITHUB_OAUTH_CLIENT_SECRET"],
	}
	base.authorizeURL = authorizeURL
	base.tokenURL = tokenURL
	base.callbackURL = callbackURL
	base.frontendURL = frontendURL
	base.flowEncryptionKey = Secret{value: encryptionKey}
	return base, nil
}

func parseAppEnvironment(raw string) (string, error) {
	switch raw {
	case "development", "test", "staging", "production":
		return raw, nil
	default:
		return "", configError(
			"APP_ENV",
			"must be development, test, staging, or production",
		)
	}
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

func parseSecureURL(key, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		parsed.User != nil || parsed.Fragment != "" {
		return nil, configError(key, "must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "https" && !isLoopbackHTTP(parsed) {
		return nil, configError(
			key,
			"must use HTTPS unless the host is loopback",
		)
	}
	return parsed, nil
}

func parseDatabaseURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil ||
		(parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") ||
		parsed.Hostname() == "" ||
		parsed.User == nil ||
		parsed.User.Username() == "" ||
		parsed.Path == "" ||
		parsed.Path == "/" ||
		parsed.Fragment != "" {
		return "", configError(
			"DATABASE_URL",
			"must be a PostgreSQL URL with credentials, host, and database name",
		)
	}
	if _, hasPassword := parsed.User.Password(); !hasPassword {
		return "", configError(
			"DATABASE_URL",
			"must include a password supplied through the environment",
		)
	}
	sslModes := parsed.Query()["sslmode"]
	if len(sslModes) != 1 ||
		(sslModes[0] != "require" && sslModes[0] != "verify-full") {
		return "", configError(
			"DATABASE_URL",
			"must set sslmode=require or sslmode=verify-full",
		)
	}

	return raw, nil
}

func parseTrustedProxyCIDRs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	cidrs := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		cidr := strings.TrimSpace(part)
		_, network, err := net.ParseCIDR(cidr)
		if err != nil || network.String() != cidr {
			return nil, configError(
				"TRUSTED_PROXY_CIDRS",
				"must contain canonical comma-separated CIDR ranges",
			)
		}
		if _, exists := seen[cidr]; exists {
			continue
		}
		seen[cidr] = struct{}{}
		cidrs = append(cidrs, cidr)
	}
	return cidrs, nil
}

func isLoopbackHTTP(parsed *url.URL) bool {
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func parseDuration(
	key string,
	fallback time.Duration,
	maximum time.Duration,
) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 || value > maximum {
		return 0, configError(
			key,
			fmt.Sprintf(
				"must be a positive duration no greater than %s",
				maximum,
			),
		)
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
