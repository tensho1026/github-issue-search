package github

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestGetUserNormalizesPayloadAndRateLimit(t *testing.T) {
	reset := time.Date(2026, time.July, 30, 2, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != "/users/tensho1026" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := request.Header.Get("X-GitHub-Api-Version"); got != apiVersion {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-RateLimit-Limit", "5000")
		writer.Header().Set("X-RateLimit-Remaining", "4999")
		writer.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		_, _ = io.WriteString(writer, `{
			"login":"tensho1026",
			"name":"Tensho Shirakawa",
			"avatar_url":"https://avatars.example/user.png",
			"bio":"Web Developer",
			"public_repos":30,
			"followers":10,
			"following":20
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	result, err := client.GetUser(context.Background(), "tensho1026")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if result.Profile.Login != "tensho1026" ||
		result.Profile.Name != "Tensho Shirakawa" ||
		result.Profile.PublicRepos != 30 ||
		result.Profile.Followers != 10 ||
		result.Profile.Following != 20 {
		t.Fatalf("GetUser() profile = %+v", result.Profile)
	}
	if !result.RateLimit.Known ||
		result.RateLimit.Limit != 5000 ||
		result.RateLimit.Remaining != 4999 ||
		!result.RateLimit.Reset.Equal(reset) {
		t.Fatalf("GetUser() rate limit = %+v", result.RateLimit)
	}
}

func TestGetUserSupportsNullableGitHubFields(t *testing.T) {
	server := jsonServer(`{
		"login":"octocat",
		"name":null,
		"avatar_url":"https://avatars.example/octocat.png",
		"bio":null,
		"public_repos":8,
		"followers":1,
		"following":2
	}`)
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").GetUser(
		context.Background(),
		"octocat",
	)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if result.Profile.Name != "" || result.Profile.Bio != "" {
		t.Fatalf("nullable fields = %+v", result.Profile)
	}
}

func TestGetUserMapsNonRetryableStatuses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		remaining string
		wantKind  port.GitHubErrorKind
	}{
		{
			name:     "not found",
			status:   http.StatusNotFound,
			wantKind: port.GitHubErrorNotFound,
		},
		{
			name:      "rate limited",
			status:    http.StatusForbidden,
			remaining: "0",
			wantKind:  port.GitHubErrorRateLimited,
		},
		{
			name:      "unauthorized",
			status:    http.StatusForbidden,
			remaining: "10",
			wantKind:  port.GitHubErrorUnauthorized,
		},
		{
			name:     "forbidden without rate headers",
			status:   http.StatusForbidden,
			wantKind: port.GitHubErrorUnauthorized,
		},
		{
			name:     "bad request",
			status:   http.StatusBadRequest,
			wantKind: port.GitHubErrorUpstream,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				_ *http.Request,
			) {
				requests.Add(1)
				if test.remaining != "" {
					writer.Header().Set("X-RateLimit-Remaining", test.remaining)
				}
				writer.WriteHeader(test.status)
			}))
			defer server.Close()

			_, err := newTestClient(t, server.URL, "").GetUser(
				context.Background(),
				"octocat",
			)
			if !port.IsGitHubError(err, test.wantKind) {
				t.Fatalf("GetUser() error = %v, want kind %s", err, test.wantKind)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1", got)
			}
		})
	}
}

func TestGetUserRetriesTransientStatuses(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		attempt := requests.Add(1)
		if attempt < maxAttempts {
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(writer, "temporary failure")
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"login":"octocat"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "")
	var sleeps atomic.Int32
	client.backoff = func(int) time.Duration { return 0 }
	client.sleep = func(context.Context, time.Duration) error {
		sleeps.Add(1)
		return nil
	}

	_, err := client.GetUser(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got := requests.Load(); got != maxAttempts {
		t.Fatalf("requests = %d, want %d", got, maxAttempts)
	}
	if got := sleeps.Load(); got != maxAttempts-1 {
		t.Fatalf("sleeps = %d, want %d", got, maxAttempts-1)
	}
}

func TestGetUserRejectsMalformedOrInvalidPayload(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"login":`},
		{name: "invalid login", body: `{"login":"invalid--login"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := jsonServer(test.body)
			defer server.Close()

			_, err := newTestClient(t, server.URL, "").GetUser(
				context.Background(),
				"octocat",
			)
			if !port.IsGitHubError(err, port.GitHubErrorUpstream) {
				t.Fatalf("GetUser() error = %v", err)
			}
		})
	}
}

func TestGetUserPropagatesCancellationWithoutRetry(t *testing.T) {
	var requests atomic.Int32
	client := newTestClient(t, "https://api.github.example", "")
	client.httpClient = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests.Add(1)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetUser(ctx, user.Username("octocat"))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetUser() error = %v, want context canceled", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestGetUserBoundsUpstreamTimeoutRetries(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		select {
		case <-request.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
			_, _ = io.WriteString(writer, `{"login":"octocat"}`)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	client := NewClient(
		baseURL,
		"",
		5*time.Millisecond,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
	)
	client.backoff = func(int) time.Duration { return 0 }
	client.sleep = func(context.Context, time.Duration) error { return nil }

	_, err = client.GetUser(context.Background(), "octocat")

	if !port.IsGitHubError(err, port.GitHubErrorUpstream) {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got := requests.Load(); got != maxAttempts {
		t.Fatalf("requests = %d, want %d", got, maxAttempts)
	}
}

func TestListRepositoriesPaginatesAndHonorsLimit(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		attempt := requests.Add(1)
		if request.URL.Path != "/users/octocat/repos" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("type") != "owner" ||
			request.URL.Query().Get("sort") != "updated" ||
			request.URL.Query().Get("direction") != "desc" {
			t.Errorf("query = %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-RateLimit-Remaining", strconv.Itoa(50-int(attempt)))
		switch attempt {
		case 1:
			if request.URL.Query().Get("page") != "1" ||
				request.URL.Query().Get("per_page") != "3" {
				t.Errorf("first query = %s", request.URL.RawQuery)
			}
			writer.Header().Set(
				"Link",
				`<http://example.test?page=2>; rel="next", <http://example.test?page=2>; rel="last"`,
			)
			_, _ = io.WriteString(writer, `[
				{
					"id":1,
					"owner":{"login":"octocat"},
					"name":"alpha",
					"full_name":"octocat/alpha",
					"description":"Alpha repository",
					"html_url":"https://github.com/octocat/alpha",
					"language":"Go",
					"stargazers_count":10,
					"forks_count":2,
					"open_issues_count":3,
					"fork":false,
					"archived":false,
					"default_branch":"main",
					"updated_at":"2026-07-30T01:00:00Z",
					"pushed_at":"2026-07-30T00:30:00Z"
				},
				{"id":2,"owner":{"login":"octocat"},"name":"beta","full_name":"octocat/beta"}
			]`)
		case 2:
			if request.URL.Query().Get("page") != "2" ||
				request.URL.Query().Get("per_page") != "3" {
				t.Errorf("second query = %s", request.URL.RawQuery)
			}
			_, _ = io.WriteString(writer, `[
				{"id":3,"owner":{"login":"octocat"},"name":"gamma","full_name":"octocat/gamma"},
				{"id":4,"owner":{"login":"octocat"},"name":"must-not-escape-limit"}
			]`)
		default:
			t.Errorf("unexpected request %d", attempt)
		}
	}))
	defer server.Close()

	repositories, rateLimit, err := newTestClient(t, server.URL, "").
		ListRepositories(context.Background(), "octocat", 3)
	if err != nil {
		t.Fatalf("ListRepositories() error = %v", err)
	}
	if len(repositories) != 3 {
		t.Fatalf("repository count = %d, want 3", len(repositories))
	}
	if repositories[0].FullName != "octocat/alpha" ||
		repositories[0].MainLanguage != "Go" ||
		repositories[0].Stars != 10 ||
		repositories[2].Name != "gamma" {
		t.Fatalf("repositories = %+v", repositories)
	}
	if !rateLimit.Known || rateLimit.Remaining != 48 {
		t.Fatalf("rate limit = %+v", rateLimit)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestListRepositoriesStopsWithoutNextLink(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		requests.Add(1)
		_, _ = io.WriteString(writer, `[{"id":1,"name":"only"}]`)
	}))
	defer server.Close()

	repositories, _, err := newTestClient(t, server.URL, "").
		ListRepositories(context.Background(), "octocat", 100)
	if err != nil {
		t.Fatalf("ListRepositories() error = %v", err)
	}
	if len(repositories) != 1 || requests.Load() != 1 {
		t.Fatalf("repositories = %d, requests = %d", len(repositories), requests.Load())
	}
}

func TestGetRepositoryLanguagesNormalizesPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != "/repos/octocat/hello-world/languages" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		writer.Header().Set("X-RateLimit-Remaining", "41")
		_, _ = io.WriteString(writer, `{"Go":1200,"TypeScript":800}`)
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").GetRepositoryLanguages(
		context.Background(),
		"octocat",
		"hello-world",
	)
	if err != nil {
		t.Fatalf("GetRepositoryLanguages() error = %v", err)
	}
	if result.Languages["Go"] != 1200 ||
		result.Languages["TypeScript"] != 800 ||
		!result.RateLimit.Known ||
		result.RateLimit.Remaining != 41 {
		t.Fatalf("GetRepositoryLanguages() result = %+v", result)
	}
}

func TestGetRepositoryLanguagesRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"Go":`},
		{name: "negative count", body: `{"Go":-1}`},
		{name: "blank language", body: `{" ":1}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := jsonServer(test.body)
			defer server.Close()

			_, err := newTestClient(t, server.URL, "").GetRepositoryLanguages(
				context.Background(),
				"octocat",
				"hello-world",
			)
			if !port.IsGitHubError(err, port.GitHubErrorUpstream) {
				t.Fatalf("GetRepositoryLanguages() error = %v", err)
			}
		})
	}
}

func TestGetRepositoryFileDecodesContent(t *testing.T) {
	content := []byte(`{"dependencies":{"react":"latest"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != "/repos/octocat/hello-world/contents/package.json" {
			t.Errorf("path = %q", request.URL.Path)
		}
		writer.Header().Set("X-RateLimit-Remaining", "40")
		_, _ = io.WriteString(
			writer,
			`{"encoding":"base64","content":"`+
				base64.StdEncoding.EncodeToString(content)+`"}`,
		)
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").GetRepositoryFile(
		context.Background(),
		"octocat",
		"hello-world",
		"package.json",
	)
	if err != nil {
		t.Fatalf("GetRepositoryFile() error = %v", err)
	}
	if !result.Exists ||
		string(result.Content) != string(content) ||
		result.RateLimit.Remaining != 40 {
		t.Fatalf("GetRepositoryFile() result = %+v", result)
	}
}

func TestGetRepositoryFileTreatsNotFoundAsOptional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Set("X-RateLimit-Remaining", "39")
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL, "").GetRepositoryFile(
		context.Background(),
		"octocat",
		"hello-world",
		"go.mod",
	)
	if err != nil {
		t.Fatalf("GetRepositoryFile() error = %v", err)
	}
	if result.Exists || result.Content != nil || result.RateLimit.Remaining != 39 {
		t.Fatalf("GetRepositoryFile() result = %+v", result)
	}
}

func TestGetRepositoryFileRejectsUnsafeUpstreamContent(t *testing.T) {
	oversized := make([]byte, maxManifestBytes+1)
	tests := []struct {
		name string
		body string
	}{
		{
			name: "unsupported encoding",
			body: `{"encoding":"utf-8","content":"hello"}`,
		},
		{
			name: "malformed base64",
			body: `{"encoding":"base64","content":"***"}`,
		},
		{
			name: "oversized decoded content",
			body: `{"encoding":"base64","content":"` +
				base64.StdEncoding.EncodeToString(oversized) + `"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := jsonServer(test.body)
			defer server.Close()

			_, err := newTestClient(t, server.URL, "").GetRepositoryFile(
				context.Background(),
				"octocat",
				"hello-world",
				"package.json",
			)
			if !port.IsGitHubError(err, port.GitHubErrorUpstream) {
				t.Fatalf("GetRepositoryFile() error = %v", err)
			}
		})
	}
}

func newTestClient(t *testing.T, rawURL, token string) *Client {
	t.Helper()
	baseURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return NewClient(
		baseURL,
		token,
		time.Second,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
	)
}

func jsonServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, body)
	}))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}
