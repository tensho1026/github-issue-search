package issue

import (
	"slices"
	"strings"
)

type technologyRule struct {
	name         string
	kind         TechnologyKind
	textTerms    []string
	dependencies []string
}

var technologyRules = []technologyRule{
	{name: "Angular", kind: TechnologyFramework, textTerms: []string{"angular"}, dependencies: []string{"@angular/core"}},
	{name: "Docker", kind: TechnologyPlatform, textTerms: []string{"docker", "dockerfile", "container"}, dependencies: []string{}},
	{name: "Gin", kind: TechnologyFramework, textTerms: []string{"gin", "gin-gonic"}, dependencies: []string{"github.com/gin-gonic/gin"}},
	{name: "Go", kind: TechnologyLanguage, textTerms: []string{"golang", "go module"}, dependencies: []string{}},
	{name: "GraphQL", kind: TechnologyCapability, textTerms: []string{"graphql"}, dependencies: []string{"graphql"}},
	{name: "gRPC", kind: TechnologyCapability, textTerms: []string{"grpc", "protocol buffers", "protobuf"}, dependencies: []string{"google.golang.org/grpc", "@grpc/grpc-js"}},
	{name: "Java", kind: TechnologyLanguage, textTerms: []string{"java", "jvm"}, dependencies: []string{}},
	{name: "JavaScript", kind: TechnologyLanguage, textTerms: []string{"javascript", "ecmascript"}, dependencies: []string{}},
	{name: "Kubernetes", kind: TechnologyPlatform, textTerms: []string{"kubernetes", "k8s", "helm"}, dependencies: []string{}},
	{name: "MongoDB", kind: TechnologyDatabase, textTerms: []string{"mongodb", "mongo"}, dependencies: []string{"mongodb", "go.mongodb.org/mongo-driver"}},
	{name: "MySQL", kind: TechnologyDatabase, textTerms: []string{"mysql"}, dependencies: []string{"mysql"}},
	{name: "Next.js", kind: TechnologyFramework, textTerms: []string{"next.js", "nextjs"}, dependencies: []string{"next"}},
	{name: "Node.js", kind: TechnologyPlatform, textTerms: []string{"node.js", "nodejs", "node runtime"}, dependencies: []string{}},
	{name: "PHP", kind: TechnologyLanguage, textTerms: []string{"php"}, dependencies: []string{}},
	{name: "PostgreSQL", kind: TechnologyDatabase, textTerms: []string{"postgresql", "postgres"}, dependencies: []string{"pg", "github.com/jackc/pgx"}},
	{name: "Python", kind: TechnologyLanguage, textTerms: []string{"python"}, dependencies: []string{}},
	{name: "React", kind: TechnologyFramework, textTerms: []string{"react", "reactjs"}, dependencies: []string{"react", "react-dom"}},
	{name: "Redis", kind: TechnologyDatabase, textTerms: []string{"redis"}, dependencies: []string{"redis", "github.com/redis/go-redis"}},
	{name: "REST API", kind: TechnologyCapability, textTerms: []string{"rest api", "restful", "api endpoint", "http endpoint"}, dependencies: []string{}},
	{name: "Rust", kind: TechnologyLanguage, textTerms: []string{"rust", "cargo"}, dependencies: []string{}},
	{name: "SQL", kind: TechnologyCapability, textTerms: []string{"sql", "database query"}, dependencies: []string{}},
	{name: "Svelte", kind: TechnologyFramework, textTerms: []string{"svelte", "sveltekit"}, dependencies: []string{"svelte", "@sveltejs/kit"}},
	{name: "Terraform", kind: TechnologyPlatform, textTerms: []string{"terraform", "infrastructure as code"}, dependencies: []string{}},
	{name: "Testing", kind: TechnologyPractice, textTerms: []string{"test", "tests", "testing", "regression", "unit test", "integration test"}, dependencies: []string{"vitest", "jest", "pytest", "testify"}},
	{name: "TypeScript", kind: TechnologyLanguage, textTerms: []string{"typescript"}, dependencies: []string{"typescript"}},
	{name: "Vue", kind: TechnologyFramework, textTerms: []string{"vue", "vue.js", "vuejs"}, dependencies: []string{"vue"}},
}

func inferRequiredTechnologies(
	input normalizedAnalysisInput,
) []RequiredTechnology {
	inferred := make(map[string]RequiredTechnology)
	if input.repositoryLanguage != "" {
		addTechnologyEvidence(
			inferred,
			canonicalTechnology(input.repositoryLanguage),
			kindForLanguage(input.repositoryLanguage),
			ConfidenceMedium,
			Evidence{
				RuleID:      "technology.repository-language",
				Source:      EvidenceRepositoryLanguage,
				Description: "The repository primary language may be required.",
			},
		)
	}

	for _, rule := range technologyRules {
		textSource := EvidenceBody
		textMatched := containsAnyTerm(input.body, rule.textTerms...)
		if containsAnyTerm(input.title, rule.textTerms...) {
			textMatched = true
			textSource = EvidenceTitle
		}
		if textMatched {
			addTechnologyEvidence(
				inferred,
				rule.name,
				rule.kind,
				ConfidenceMedium,
				Evidence{
					RuleID:      "technology.explicit-text",
					Source:      textSource,
					Description: "The issue title or body names this technology.",
				},
			)
		}
		for _, dependency := range input.dependencies {
			if matchesDependency(dependency, rule.dependencies) {
				addTechnologyEvidence(
					inferred,
					rule.name,
					rule.kind,
					ConfidenceHigh,
					Evidence{
						RuleID:      "technology.manifest-dependency",
						Source:      EvidenceDependency,
						Description: "A repository manifest dependency identifies this technology.",
					},
				)
				break
			}
		}
	}

	technologies := make([]RequiredTechnology, 0, len(inferred))
	for _, technology := range inferred {
		slices.SortFunc(technology.Evidence, compareEvidence)
		technologies = append(technologies, technology)
	}
	slices.SortFunc(technologies, func(
		left RequiredTechnology,
		right RequiredTechnology,
	) int {
		if confidenceRank(left.Confidence) != confidenceRank(right.Confidence) {
			return confidenceRank(right.Confidence) -
				confidenceRank(left.Confidence)
		}
		if left.Kind != right.Kind {
			return strings.Compare(string(left.Kind), string(right.Kind))
		}
		return strings.Compare(left.Name, right.Name)
	})
	return technologies
}

func addTechnologyEvidence(
	inferred map[string]RequiredTechnology,
	name string,
	kind TechnologyKind,
	confidence Confidence,
	evidence Evidence,
) {
	key := strings.ToLower(name)
	technology, exists := inferred[key]
	if !exists {
		technology = RequiredTechnology{
			Name:       name,
			Kind:       kind,
			Confidence: confidence,
			Evidence:   []Evidence{},
		}
	}
	if confidenceRank(confidence) > confidenceRank(technology.Confidence) {
		technology.Confidence = confidence
	}
	for _, existing := range technology.Evidence {
		if existing.RuleID == evidence.RuleID &&
			existing.Source == evidence.Source {
			inferred[key] = technology
			return
		}
	}
	technology.Evidence = append(technology.Evidence, evidence)
	inferred[key] = technology
}

func canonicalTechnology(value string) string {
	for _, rule := range technologyRules {
		if strings.EqualFold(rule.name, value) {
			return rule.name
		}
	}
	switch strings.ToLower(value) {
	case "golang":
		return "Go"
	case "csharp", "c#":
		return "C#"
	default:
		return strings.TrimSpace(value)
	}
}

func kindForLanguage(language string) TechnologyKind {
	switch strings.ToLower(language) {
	case "go", "golang", "typescript", "javascript", "python", "java",
		"rust", "php", "c#", "csharp", "ruby", "kotlin", "swift":
		return TechnologyLanguage
	default:
		return TechnologyCapability
	}
}

func matchesDependency(dependency string, patterns []string) bool {
	for _, pattern := range patterns {
		if dependency == pattern ||
			strings.HasPrefix(dependency, pattern+"/") ||
			strings.HasSuffix(dependency, "/"+pattern) ||
			strings.Contains(dependency, "/"+pattern+"/") {
			return true
		}
	}
	return false
}

func confidenceRank(confidence Confidence) int {
	switch confidence {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	default:
		return 1
	}
}

func compareEvidence(left Evidence, right Evidence) int {
	if left.RuleID != right.RuleID {
		return strings.Compare(left.RuleID, right.RuleID)
	}
	if left.Source != right.Source {
		return strings.Compare(string(left.Source), string(right.Source))
	}
	return strings.Compare(left.Description, right.Description)
}
