package usecase

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestAnalyzeGitHubProfileAggregatesAndCachesData(t *testing.T) {
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	reader := &profileAnalysisReaderStub{
		repositories: []repository.Summary{
			{
				Owner:        "octocat",
				Name:         "api",
				FullName:     "octocat/api",
				MainLanguage: "Go",
				UpdatedAt:    now.Add(2 * time.Hour),
			},
			{
				Owner:        "octocat",
				Name:         "web",
				FullName:     "octocat/web",
				MainLanguage: "TypeScript",
				UpdatedAt:    now.Add(time.Hour),
			},
			{
				Owner:        "octocat",
				Name:         "archived",
				FullName:     "octocat/archived",
				MainLanguage: "Rust",
				IsArchived:   true,
				UpdatedAt:    now.Add(3 * time.Hour),
			},
		},
		languages: map[string]map[string]int64{
			"api": {"Go": 100},
			"web": {"TypeScript": 100},
		},
		files: map[string][]byte{
			"api:go.mod": []byte("require github.com/gin-gonic/gin v1.12.0"),
			"web:package.json": []byte(
				`{"dependencies":{"react":"latest"}}`,
			),
		},
		languageDelay: 20 * time.Millisecond,
	}
	usecase := newProfileAnalysisUsecase(t, reader, 20, 3, 2)

	output, err := usecase.Execute(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Analysis.Username != "octocat" ||
		output.Analysis.RepositoriesAnalyzed != 2 ||
		len(output.Analysis.Languages) != 2 ||
		output.Analysis.Languages[0].Name != "Go" ||
		output.Analysis.Languages[0].Percentage != 50 ||
		output.Analysis.Languages[1].Name != "TypeScript" ||
		output.Analysis.Languages[1].Percentage != 50 {
		t.Fatalf("Execute() analysis = %+v", output.Analysis)
	}
	if len(output.Analysis.Frameworks) != 2 ||
		output.Analysis.Frameworks[0] != "Gin" ||
		output.Analysis.Frameworks[1] != "React" {
		t.Fatalf("frameworks = %v", output.Analysis.Frameworks)
	}
	if output.RateLimit.Remaining != 70 {
		t.Fatalf("rate limit = %+v", output.RateLimit)
	}
	if reader.maximumConcurrency() > 2 || reader.maximumConcurrency() < 2 {
		t.Fatalf("maximum concurrency = %d, want 2", reader.maximumConcurrency())
	}

	if _, err := usecase.Execute(context.Background(), "OCTOCAT"); err != nil {
		t.Fatalf("cached Execute() error = %v", err)
	}
	if calls := reader.callCounts(); calls != (profileAnalysisCallCounts{
		getUser:   1,
		list:      1,
		languages: 2,
		files:     2,
	}) {
		t.Fatalf("calls after cache hit = %+v", calls)
	}
}

func TestAnalyzeGitHubProfileReturnsDeterministicPartialWarnings(t *testing.T) {
	now := time.Now()
	reader := &profileAnalysisReaderStub{
		repositories: []repository.Summary{
			{
				Owner:        "octocat",
				Name:         "alpha",
				FullName:     "octocat/alpha",
				MainLanguage: "Go",
				UpdatedAt:    now.Add(time.Hour),
			},
			{
				Owner:        "octocat",
				Name:         "beta",
				FullName:     "octocat/beta",
				MainLanguage: "TypeScript",
				UpdatedAt:    now,
			},
		},
		languages: map[string]map[string]int64{
			"beta": {"TypeScript": 10},
		},
		languageErrors: map[string]error{
			"alpha": errors.New("language endpoint unavailable"),
		},
		fileErrors: map[string]error{
			"beta:package.json": errors.New("contents endpoint unavailable"),
		},
	}

	output, err := newProfileAnalysisUsecase(t, reader, 20, 3, 2).
		Execute(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Analysis.RepositoriesAnalyzed != 2 ||
		len(output.Analysis.Languages) != 1 ||
		output.Analysis.Languages[0].Name != "TypeScript" {
		t.Fatalf("analysis = %+v", output.Analysis)
	}
	if len(output.Analysis.Warnings) != 2 {
		t.Fatalf("warnings = %+v", output.Analysis.Warnings)
	}
	if output.Analysis.Warnings[0].Code != warningLanguageUnavailable ||
		output.Analysis.Warnings[0].Repository != "octocat/alpha" ||
		output.Analysis.Warnings[1].Code != warningManifestUnavailable ||
		output.Analysis.Warnings[1].Repository != "octocat/beta" {
		t.Fatalf("warning order = %+v", output.Analysis.Warnings)
	}
}

func TestAnalyzeGitHubProfileDeduplicatesConcurrentRequests(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	reader := &profileAnalysisReaderStub{
		started: started,
		release: release,
	}
	usecase := newProfileAnalysisUsecase(t, reader, 20, 3, 5)

	const requestCount = 12
	var waitGroup sync.WaitGroup
	errorsByRequest := make(chan error, requestCount)
	for index := 0; index < requestCount; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := usecase.Execute(context.Background(), "octocat")
			errorsByRequest <- err
		}()
	}
	<-started
	close(release)
	waitGroup.Wait()
	close(errorsByRequest)

	for err := range errorsByRequest {
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}
	if calls := reader.callCounts(); calls.getUser != 1 || calls.list != 1 {
		t.Fatalf("single-flight calls = %+v", calls)
	}
}

func TestAnalyzeGitHubProfileMapsFatalGitHubErrors(t *testing.T) {
	tests := []struct {
		name       string
		readerErr  error
		wantCode   apperror.Code
		wantStatus int
	}{
		{
			name:       "not found",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorNotFound},
			wantCode:   apperror.CodeGitHubUserNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rate limited",
			readerErr:  &port.GitHubError{Kind: port.GitHubErrorRateLimited},
			wantCode:   apperror.CodeRateLimit,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "cancelled",
			readerErr:  context.Canceled,
			wantCode:   apperror.CodeRequestTimeout,
			wantStatus: http.StatusGatewayTimeout,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &profileAnalysisReaderStub{userError: test.readerErr}
			_, err := newProfileAnalysisUsecase(t, reader, 20, 3, 5).
				Execute(context.Background(), "octocat")

			var applicationError *apperror.Error
			if !errors.As(err, &applicationError) {
				t.Fatalf("Execute() error = %v", err)
			}
			if applicationError.Code != test.wantCode ||
				applicationError.HTTPStatus != test.wantStatus {
				t.Fatalf("application error = %+v", applicationError)
			}
			if !errors.Is(applicationError, test.readerErr) {
				t.Fatal("Execute() did not preserve the fatal cause")
			}
		})
	}
}

func TestAnalyzeGitHubProfilePropagatesCancellationDuringRepositoryWork(
	t *testing.T,
) {
	reader := &profileAnalysisReaderStub{
		repositories: []repository.Summary{{
			Owner:        "octocat",
			Name:         "api",
			FullName:     "octocat/api",
			MainLanguage: "Go",
		}},
		blockLanguages: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newProfileAnalysisUsecase(t, reader, 20, 3, 5).
		Execute(ctx, "octocat")
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) ||
		applicationError.Code != apperror.CodeRequestTimeout {
		t.Fatalf("Execute() error = %v", err)
	}
}

func newProfileAnalysisUsecase(
	t *testing.T,
	reader port.GitHubProfileAnalysisReader,
	repositoryLimit int,
	manifestLimit int,
	maxConcurrency int,
) AnalyzeGitHubProfile {
	t.Helper()
	cache, err := memory.NewProfileAnalysis(100, time.Hour)
	if err != nil {
		t.Fatalf("NewProfileAnalysis() error = %v", err)
	}
	return NewAnalyzeGitHubProfile(
		reader,
		cache,
		repositoryLimit,
		manifestLimit,
		maxConcurrency,
	)
}

type profileAnalysisReaderStub struct {
	mu             sync.Mutex
	repositories   []repository.Summary
	languages      map[string]map[string]int64
	files          map[string][]byte
	languageErrors map[string]error
	fileErrors     map[string]error
	userError      error
	languageDelay  time.Duration
	blockLanguages bool
	started        chan struct{}
	startedOnce    sync.Once
	release        chan struct{}
	getUserCalls   int
	listCalls      int
	languageCalls  int
	fileCalls      int
	active         int
	maxActive      int
}

type profileAnalysisCallCounts struct {
	getUser   int
	list      int
	languages int
	files     int
}

func (stub *profileAnalysisReaderStub) GetUser(
	ctx context.Context,
	username user.Username,
) (port.GitHubUserResult, error) {
	stub.mu.Lock()
	stub.getUserCalls++
	stub.mu.Unlock()

	if stub.started != nil {
		stub.startedOnce.Do(func() { close(stub.started) })
	}
	if stub.release != nil {
		select {
		case <-stub.release:
		case <-ctx.Done():
			return port.GitHubUserResult{}, ctx.Err()
		}
	}
	if stub.userError != nil {
		return port.GitHubUserResult{}, stub.userError
	}
	return port.GitHubUserResult{
		Profile: user.Profile{Login: username},
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 100,
		},
	}, nil
}

func (stub *profileAnalysisReaderStub) ListRepositories(
	context.Context,
	user.Username,
	int,
) ([]repository.Summary, port.RateLimit, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.listCalls++
	return append([]repository.Summary(nil), stub.repositories...),
		port.RateLimit{Known: true, Remaining: 90},
		nil
}

func (stub *profileAnalysisReaderStub) GetRepositoryLanguages(
	ctx context.Context,
	_ string,
	name string,
) (port.GitHubLanguagesResult, error) {
	stub.mu.Lock()
	stub.languageCalls++
	stub.active++
	if stub.active > stub.maxActive {
		stub.maxActive = stub.active
	}
	stub.mu.Unlock()
	defer func() {
		stub.mu.Lock()
		stub.active--
		stub.mu.Unlock()
	}()

	if stub.blockLanguages {
		<-ctx.Done()
		return port.GitHubLanguagesResult{}, ctx.Err()
	}
	if stub.languageDelay > 0 {
		select {
		case <-time.After(stub.languageDelay):
		case <-ctx.Done():
			return port.GitHubLanguagesResult{}, ctx.Err()
		}
	}
	if err := stub.languageErrors[name]; err != nil {
		return port.GitHubLanguagesResult{}, err
	}
	return port.GitHubLanguagesResult{
		Languages: cloneLanguageMap(stub.languages[name]),
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 80,
		},
	}, nil
}

func (stub *profileAnalysisReaderStub) GetRepositoryFile(
	_ context.Context,
	_ string,
	name string,
	filePath string,
) (port.GitHubRepositoryFileResult, error) {
	stub.mu.Lock()
	stub.fileCalls++
	stub.mu.Unlock()

	key := name + ":" + filePath
	if err := stub.fileErrors[key]; err != nil {
		return port.GitHubRepositoryFileResult{}, err
	}
	content, exists := stub.files[key]
	return port.GitHubRepositoryFileResult{
		Content: append([]byte(nil), content...),
		Exists:  exists,
		RateLimit: port.RateLimit{
			Known:     true,
			Remaining: 70,
		},
	}, nil
}

func (stub *profileAnalysisReaderStub) maximumConcurrency() int {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.maxActive
}

func (stub *profileAnalysisReaderStub) callCounts() profileAnalysisCallCounts {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return profileAnalysisCallCounts{
		getUser:   stub.getUserCalls,
		list:      stub.listCalls,
		languages: stub.languageCalls,
		files:     stub.fileCalls,
	}
}

func cloneLanguageMap(source map[string]int64) map[string]int64 {
	cloned := make(map[string]int64, len(source))
	for language, count := range source {
		cloned[language] = count
	}
	return cloned
}
