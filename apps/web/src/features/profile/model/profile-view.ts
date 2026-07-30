import type {
  GitHubUser,
  LanguageShare,
  ProfileAnalysis,
  RepositorySummary,
} from "../../../shared/api/generated";

export type LanguageOrder = "alphabetical" | "usage";

export function sortLanguages(
  languages: ReadonlyArray<LanguageShare>,
  order: LanguageOrder,
): Array<LanguageShare> {
  return [...languages].sort((left, right) => {
    if (order === "usage" && left.percentage !== right.percentage) {
      return right.percentage - left.percentage;
    }
    return left.name.localeCompare(right.name);
  });
}

export function profileTechnologyTags(
  analysis: ProfileAnalysis,
): Array<string> {
  const tags = new Map<string, string>();
  for (const language of analysis.languages) {
    tags.set(language.name.toLowerCase(), language.name);
  }
  for (const framework of analysis.frameworks) {
    const normalized = framework.trim();
    const key = normalized.toLowerCase();
    if (normalized && !tags.has(key)) {
      tags.set(key, normalized);
    }
  }
  return [...tags.values()].sort((left, right) => left.localeCompare(right));
}

export function featuredRepositories(
  user: GitHubUser,
  limit = 6,
): Array<RepositorySummary> {
  return [...user.repositories]
    .sort((left, right) => {
      if (left.stars !== right.stars) {
        return right.stars - left.stars;
      }
      return left.fullName.localeCompare(right.fullName);
    })
    .slice(0, Math.max(0, limit));
}
