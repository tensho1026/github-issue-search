package profile

import (
	"reflect"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
)

func TestSelectRepositoriesUsesDeterministicPriority(t *testing.T) {
	recent := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	older := recent.Add(-time.Hour)
	repositories := []repository.Summary{
		{FullName: "owner/archived", IsArchived: true, UpdatedAt: recent},
		{FullName: "owner/older", UpdatedAt: older},
		{FullName: "owner/recent-fork", IsFork: true, UpdatedAt: recent},
		{FullName: "owner/recent-source", MainLanguage: "Go", UpdatedAt: recent},
		{FullName: "owner/recent-empty", UpdatedAt: recent},
	}

	selected := SelectRepositories(repositories, 3)

	got := []string{
		selected[0].FullName,
		selected[1].FullName,
		selected[2].FullName,
	}
	want := []string{
		"owner/recent-source",
		"owner/recent-empty",
		"owner/recent-fork",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectRepositories() = %v, want %v", got, want)
	}
}

func TestAggregateLanguagesProducesExactStablePercentages(t *testing.T) {
	result := AggregateLanguages([]map[string]int64{
		{"TypeScript": 1, "Go": 1},
		{"JavaScript": 1, "Ignored": 0},
	})

	want := []LanguageShare{
		{Name: "Go", Percentage: 34},
		{Name: "JavaScript", Percentage: 33},
		{Name: "TypeScript", Percentage: 33},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("AggregateLanguages() = %+v, want %+v", result, want)
	}

	total := 0
	for _, language := range result {
		total += language.Percentage
	}
	if total != 100 {
		t.Fatalf("percentage total = %d", total)
	}
}

func TestAggregateLanguagesHandlesNoPositiveBytes(t *testing.T) {
	if got := AggregateLanguages([]map[string]int64{{"Go": 0}}); len(got) != 0 {
		t.Fatalf("AggregateLanguages() = %+v, want empty", got)
	}
}

func TestManifestCandidates(t *testing.T) {
	tests := []struct {
		language string
		limit    int
		want     []string
	}{
		{language: "TypeScript", limit: 3, want: []string{"package.json"}},
		{
			language: "Java",
			limit:    1,
			want:     []string{"pom.xml"},
		},
		{
			language: "Python",
			limit:    3,
			want:     []string{"pyproject.toml", "requirements.txt"},
		},
		{language: "Unknown", limit: 3, want: []string{}},
	}

	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			if got := ManifestCandidates(test.language, test.limit); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ManifestCandidates() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestInferFrameworksAcrossSupportedManifests(t *testing.T) {
	manifests := []Manifest{
		{
			Path: "package.json",
			Content: []byte(`{
				"dependencies": {
					"react": "19.0.0",
					"next": "16.0.0",
					"@nestjs/core": "12.0.0"
				},
				"devDependencies": {"tailwindcss": "4.0.0"}
			}`),
		},
		{
			Path:    "go.mod",
			Content: []byte("require github.com/gin-gonic/gin v1.12.0"),
		},
		{
			Path:    "pom.xml",
			Content: []byte("<artifactId>spring-boot-starter-web</artifactId>"),
		},
		{
			Path:    "requirements.txt",
			Content: []byte("fastapi==1.0.0\npytest==9.0.0"),
		},
		{
			Path:    "Cargo.toml",
			Content: []byte(`axum = "1.0"`),
		},
		{
			Path:    "composer.json",
			Content: []byte(`{"require":{"laravel/framework":"^13"}}`),
		},
	}

	got := InferFrameworks(manifests)
	want := []string{
		"Axum",
		"FastAPI",
		"Gin",
		"Laravel",
		"NestJS",
		"Next.js",
		"Pytest",
		"React",
		"Spring Boot",
		"Tailwind CSS",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InferFrameworks() = %v, want %v", got, want)
	}
}

func TestInferFrameworksIgnoresMalformedPackageJSON(t *testing.T) {
	got := InferFrameworks([]Manifest{
		{Path: "package.json", Content: []byte(`{"dependencies":`)},
	})
	if len(got) != 0 {
		t.Fatalf("InferFrameworks() = %v, want empty", got)
	}
}
