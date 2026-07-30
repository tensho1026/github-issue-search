package profile

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/repository"
	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

type LanguageShare struct {
	Name       string
	Percentage int
}

type Warning struct {
	Code       string
	Message    string
	Repository string
}

type Analysis struct {
	Username             user.Username
	Languages            []LanguageShare
	LanguageStatus       EvidenceStatus
	Frameworks           []string
	RecentTechnologies   []RecentTechnology
	Contributions        ContributionAnalysis
	OSSExperience        OSSExperience
	RepositoryEvidence   RepositoryEvidence
	Proficiency          []TechnologyProficiency
	Window               AnalysisWindow
	RepositoriesAnalyzed int
	Warnings             []Warning
}

type Manifest struct {
	Path    string
	Content []byte
}

// SelectRepositories applies the documented deterministic analysis priority:
// recent first, then non-forks, then repositories with a detected language.
// Archived repositories are never analyzed.
func SelectRepositories(
	repositories []repository.Summary,
	limit int,
) []repository.Summary {
	if limit <= 0 {
		return []repository.Summary{}
	}

	selected := make([]repository.Summary, 0, min(limit, len(repositories)))
	for _, item := range repositories {
		if item.IsArchived {
			continue
		}
		selected = append(selected, item)
	}
	sort.SliceStable(selected, func(left, right int) bool {
		leftRepository := selected[left]
		rightRepository := selected[right]
		leftUpdated := maxTime(leftRepository.PushedAt, leftRepository.UpdatedAt)
		rightUpdated := maxTime(rightRepository.PushedAt, rightRepository.UpdatedAt)
		if !leftUpdated.Equal(rightUpdated) {
			return leftUpdated.After(rightUpdated)
		}
		if leftRepository.IsFork != rightRepository.IsFork {
			return !leftRepository.IsFork
		}
		leftHasLanguage := leftRepository.MainLanguage != ""
		rightHasLanguage := rightRepository.MainLanguage != ""
		if leftHasLanguage != rightHasLanguage {
			return leftHasLanguage
		}
		return leftRepository.FullName < rightRepository.FullName
	})

	if len(selected) > limit {
		selected = selected[:limit]
	}
	return slices.Clone(selected)
}

// AggregateLanguages uses the largest-remainder method so integer percentages
// remain deterministic and sum to exactly 100 when any positive bytes exist.
func AggregateLanguages(languageBytes []map[string]int64) []LanguageShare {
	totals := make(map[string]int64)
	var totalBytes int64
	for _, repositoryLanguages := range languageBytes {
		for language, count := range repositoryLanguages {
			if count <= 0 || strings.TrimSpace(language) == "" {
				continue
			}
			totals[language] += count
			totalBytes += count
		}
	}
	if totalBytes == 0 {
		return []LanguageShare{}
	}

	type allocation struct {
		name       string
		percentage int
		remainder  float64
	}
	allocations := make([]allocation, 0, len(totals))
	allocated := 0
	for name, count := range totals {
		exact := float64(count) * 100 / float64(totalBytes)
		floor := int(math.Floor(exact))
		allocations = append(allocations, allocation{
			name:       name,
			percentage: floor,
			remainder:  exact - float64(floor),
		})
		allocated += floor
	}
	sort.Slice(allocations, func(left, right int) bool {
		if allocations[left].remainder != allocations[right].remainder {
			return allocations[left].remainder > allocations[right].remainder
		}
		return allocations[left].name < allocations[right].name
	})
	for index := 0; allocated < 100; index = (index + 1) % len(allocations) {
		allocations[index].percentage++
		allocated++
	}
	sort.Slice(allocations, func(left, right int) bool {
		if allocations[left].percentage != allocations[right].percentage {
			return allocations[left].percentage > allocations[right].percentage
		}
		return allocations[left].name < allocations[right].name
	})

	result := make([]LanguageShare, 0, len(allocations))
	for _, item := range allocations {
		result = append(result, LanguageShare{
			Name:       item.name,
			Percentage: item.percentage,
		})
	}
	return result
}

func ManifestCandidates(mainLanguage string, limit int) []string {
	if limit <= 0 {
		return []string{}
	}
	candidatesByLanguage := map[string][]string{
		"javascript": {"package.json"},
		"typescript": {"package.json"},
		"go":         {"go.mod"},
		"java":       {"pom.xml", "build.gradle"},
		"kotlin":     {"build.gradle", "pom.xml"},
		"python":     {"pyproject.toml", "requirements.txt"},
		"rust":       {"Cargo.toml"},
		"php":        {"composer.json"},
	}
	candidates := candidatesByLanguage[strings.ToLower(mainLanguage)]
	if len(candidates) == 0 {
		return []string{}
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return slices.Clone(candidates)
}

func InferFrameworks(manifests []Manifest) []string {
	found := make(map[string]struct{})
	for _, manifest := range manifests {
		switch strings.ToLower(manifest.Path) {
		case "package.json":
			inferPackageJSON(manifest.Content, found)
		case "go.mod":
			inferBySubstrings(string(manifest.Content), goFrameworkRules, found)
		case "pom.xml", "build.gradle":
			inferBySubstrings(string(manifest.Content), javaFrameworkRules, found)
		case "requirements.txt", "pyproject.toml":
			inferBySubstrings(string(manifest.Content), pythonFrameworkRules, found)
		case "cargo.toml":
			inferBySubstrings(string(manifest.Content), rustFrameworkRules, found)
		case "composer.json":
			inferBySubstrings(string(manifest.Content), phpFrameworkRules, found)
		}
	}

	frameworks := make([]string, 0, len(found))
	for framework := range found {
		frameworks = append(frameworks, framework)
	}
	slices.Sort(frameworks)
	return frameworks
}

func inferPackageJSON(content []byte, found map[string]struct{}) {
	var manifest struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return
	}
	for _, dependencies := range []map[string]string{
		manifest.Dependencies,
		manifest.DevDependencies,
		manifest.PeerDependencies,
		manifest.OptionalDependencies,
	} {
		for dependency := range dependencies {
			if framework, exists := packageFrameworkRules[dependency]; exists {
				found[framework] = struct{}{}
			}
		}
	}
}

func inferBySubstrings(
	content string,
	rules map[string]string,
	found map[string]struct{},
) {
	normalized := strings.ToLower(content)
	for signature, framework := range rules {
		if strings.Contains(normalized, signature) {
			found[framework] = struct{}{}
		}
	}
}

func maxTime(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

var packageFrameworkRules = map[string]string{
	"@angular/core":     "Angular",
	"@nestjs/core":      "NestJS",
	"@prisma/client":    "Prisma",
	"@sveltejs/kit":     "SvelteKit",
	"@vue/runtime-core": "Vue",
	"next":              "Next.js",
	"nuxt":              "Nuxt",
	"prisma":            "Prisma",
	"react":             "React",
	"svelte":            "Svelte",
	"tailwindcss":       "Tailwind CSS",
	"typeorm":           "TypeORM",
	"vue":               "Vue",
}

var goFrameworkRules = map[string]string{
	"github.com/gofiber/fiber": "Fiber",
	"github.com/gin-gonic/gin": "Gin",
	"github.com/labstack/echo": "Echo",
	"gorm.io/gorm":             "GORM",
}

var javaFrameworkRules = map[string]string{
	"spring-boot": "Spring Boot",
}

var pythonFrameworkRules = map[string]string{
	"django":  "Django",
	"fastapi": "FastAPI",
	"flask":   "Flask",
	"pytest":  "Pytest",
}

var rustFrameworkRules = map[string]string{
	"actix-web": "Actix Web",
	"axum":      "Axum",
}

var phpFrameworkRules = map[string]string{
	"laravel/framework":        "Laravel",
	"symfony/framework-bundle": "Symfony",
}
