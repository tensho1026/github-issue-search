package usecase

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/cache/memory"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/profile"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
	"github.com/tensho1026/github-issue-search/apps/api/internal/platform/apperror"
	"github.com/tensho1026/github-issue-search/apps/api/internal/port"
)

func TestAnalyzeGitHubProfileBuildsExtendedSnapshotAndCachesIt(
	t *testing.T,
) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	reader := &profileAnalysisSnapshotReaderStub{
		result: port.GitHubProfileAnalysisResult{
			Snapshot: profile.ProfileSnapshot{
				Username:   "octocat",
				WindowFrom: now.AddDate(0, 0, -profile.AnalysisWindowDays),
				WindowTo:   now,
				Owned: profile.RepositoryCollection{
					Available:  true,
					Total:      1,
					TotalKnown: true,
					Limit:      20,
					Repositories: []profile.RepositoryObservation{{
						Repository: repository.Summary{
							Owner:        "octocat",
							Name:         "api",
							FullName:     "octocat/api",
							MainLanguage: "Go",
							UpdatedAt:    now.Add(-time.Hour),
						},
						Languages: map[string]int64{
							"Go":         750,
							"TypeScript": 250,
						},
						LanguagesComplete: true,
						Manifests: []profile.Manifest{{
							Path: "go.mod",
							Content: []byte(
								"require github.com/gin-gonic/gin v1.12.0",
							),
						}},
					}},
				},
				Contributed: profile.RepositoryCollection{
					Available:  true,
					Total:      1,
					TotalKnown: true,
					Limit:      20,
					Repositories: []profile.RepositoryObservation{{
						Repository: repository.Summary{
							FullName:     "community/project",
							MainLanguage: "Go",
							UpdatedAt:    now.Add(-2 * time.Hour),
						},
					}},
				},
				Starred: profile.RepositoryCollection{
					Available: true,
					Limit:     20,
				},
				Forked: profile.RepositoryCollection{
					Available:  true,
					TotalKnown: true,
					Limit:      20,
				},
				Contributions: profile.ContributionSnapshot{
					Available: true,
					Commits: profile.CountEvidence{
						Available: true,
						Value:     12,
					},
					IssuesOpened: profile.CountEvidence{
						Available: true,
						Value:     2,
						Complete:  true,
					},
					PullRequestsOpened: profile.CountEvidence{
						Available: true,
						Value:     5,
						Complete:  true,
					},
					PullRequestReviews: profile.CountEvidence{
						Available: true,
						Value:     3,
					},
					RepositoriesTouched: profile.CountEvidence{
						Available: true,
						Value:     1,
					},
				},
			},
			RateLimit: port.RateLimit{
				Known:     true,
				Limit:     5000,
				Remaining: 4975,
				Reset:     now.Add(time.Hour),
			},
		},
	}
	analyze := newProfileAnalysisUsecase(t, reader, 20, 3)

	output, err := analyze.Execute(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Analysis.Username != "octocat" ||
		output.Analysis.RepositoriesAnalyzed != 1 ||
		output.Analysis.Languages[0] != (profile.LanguageShare{
			Name: "Go", Percentage: 75,
		}) ||
		output.Analysis.LanguageStatus != profile.EvidenceExact ||
		len(output.Analysis.Frameworks) != 1 ||
		output.Analysis.Frameworks[0] != "Gin" ||
		output.Analysis.Contributions.PullRequestsOpened.Value != 5 ||
		output.Analysis.OSSExperience.Level != "contributing" ||
		len(output.Analysis.Proficiency) == 0 {
		t.Fatalf("analysis = %+v", output.Analysis)
	}
	if output.RateLimit.Remaining != 4975 {
		t.Fatalf("rate limit = %+v", output.RateLimit)
	}

	if _, err := analyze.Execute(context.Background(), "OCTOCAT"); err != nil {
		t.Fatalf("cached Execute() error = %v", err)
	}
	calls, repositoryLimit, manifestLimit := reader.observations()
	if calls != 1 || repositoryLimit != 20 || manifestLimit != 3 {
		t.Fatalf(
			"reader observations = calls:%d repositories:%d manifests:%d",
			calls,
			repositoryLimit,
			manifestLimit,
		)
	}
}

func TestAnalyzeGitHubProfilePreservesPartialWarnings(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	reader := &profileAnalysisSnapshotReaderStub{
		result: port.GitHubProfileAnalysisResult{
			Snapshot: profile.ProfileSnapshot{
				Username:   "octocat",
				WindowFrom: now.AddDate(0, 0, -profile.AnalysisWindowDays),
				WindowTo:   now,
				Warnings: []profile.Warning{{
					Code:    "contribution_activity_unavailable",
					Message: "Public contribution evidence is unavailable",
				}},
			},
		},
	}

	output, err := newProfileAnalysisUsecase(t, reader, 20, 3).
		Execute(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(output.Analysis.Warnings) != 1 ||
		output.Analysis.Warnings[0].Code !=
			"contribution_activity_unavailable" ||
		output.Analysis.Contributions.Commits.Status !=
			profile.EvidenceUnavailable ||
		output.Analysis.OSSExperience.Level != "unavailable" {
		t.Fatalf("analysis = %+v", output.Analysis)
	}
}

func TestAnalyzeGitHubProfileDeduplicatesConcurrentRequests(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	reader := &profileAnalysisSnapshotReaderStub{
		result: port.GitHubProfileAnalysisResult{
			Snapshot: profile.ProfileSnapshot{Username: "octocat"},
		},
		started: started,
		release: release,
	}
	analyze := newProfileAnalysisUsecase(t, reader, 20, 3)

	const requestCount = 12
	var waitGroup sync.WaitGroup
	errorsByRequest := make(chan error, requestCount)
	for range requestCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := analyze.Execute(context.Background(), "octocat")
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
	calls, _, _ := reader.observations()
	if calls != 1 {
		t.Fatalf("snapshot calls = %d, want 1", calls)
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
			reader := &profileAnalysisSnapshotReaderStub{err: test.readerErr}
			_, err := newProfileAnalysisUsecase(t, reader, 20, 3).
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

func TestAnalyzeGitHubProfilePropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	reader := &profileAnalysisSnapshotReaderStub{
		started:           started,
		blockUntilContext: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := newProfileAnalysisUsecase(t, reader, 20, 3).Execute(
			ctx,
			"octocat",
		)
		result <- err
	}()
	<-started
	cancel()

	var applicationError *apperror.Error
	if err := <-result; !errors.As(err, &applicationError) ||
		applicationError.Code != apperror.CodeRequestTimeout {
		t.Fatalf("Execute() error = %v", err)
	}
}

func newProfileAnalysisUsecase(
	t *testing.T,
	reader port.GitHubProfileAnalysisReader,
	repositoryLimit int,
	manifestLimit int,
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
	)
}

type profileAnalysisSnapshotReaderStub struct {
	mu                sync.Mutex
	result            port.GitHubProfileAnalysisResult
	err               error
	started           chan struct{}
	startedOnce       sync.Once
	release           chan struct{}
	blockUntilContext bool
	calls             int
	repositoryLimit   int
	manifestLimit     int
}

func (stub *profileAnalysisSnapshotReaderStub) GetProfileAnalysis(
	ctx context.Context,
	_ user.Username,
	repositoryLimit int,
	manifestLimit int,
) (port.GitHubProfileAnalysisResult, error) {
	stub.mu.Lock()
	stub.calls++
	stub.repositoryLimit = repositoryLimit
	stub.manifestLimit = manifestLimit
	stub.mu.Unlock()
	if stub.started != nil {
		stub.startedOnce.Do(func() { close(stub.started) })
	}
	if stub.blockUntilContext {
		<-ctx.Done()
		return port.GitHubProfileAnalysisResult{}, ctx.Err()
	}
	if stub.release != nil {
		select {
		case <-stub.release:
		case <-ctx.Done():
			return port.GitHubProfileAnalysisResult{}, ctx.Err()
		}
	}
	if stub.err != nil {
		return port.GitHubProfileAnalysisResult{}, stub.err
	}
	return stub.result, nil
}

func (stub *profileAnalysisSnapshotReaderStub) observations() (int, int, int) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return stub.calls, stub.repositoryLimit, stub.manifestLimit
}
