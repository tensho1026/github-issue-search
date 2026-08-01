import type {
  IssueSearchRequest,
  RepositoryDiscoveryRequest,
  SavedSearch,
} from "../../../shared/api/generated";
import { appRoutes } from "../../../shared/config/app-config";
import {
  createDefaultSearchFilters,
  encodeSearchParams,
} from "../../issue-search/model/search-filters";
import {
  createDefaultRepositoryFilters,
  encodeRepositorySearchParams,
} from "../../repository-discovery/model/repository-filters";

export function savedSearchRoute(search: SavedSearch): string {
  if (search.searchType === "issue") {
    const filters = search.filters as IssueSearchRequest;
    const parameters = encodeSearchParams({
      ...createDefaultSearchFilters(filters.username),
      excludeArchived: filters.excludeArchived ?? true,
      frameworks: filters.frameworks ?? [],
      includeDocumentation: filters.includeDocumentation ?? false,
      includeEnglish: filters.includeEnglish ?? true,
      labels: filters.labels ?? ["good first issue", "help wanted"],
      languages: filters.languages ?? [],
      maximumDifficulty: filters.maximumDifficulty ?? 3,
      maximumEffort: filters.maximumEffort ?? "",
      minimumStars: filters.minimumStars ?? 10,
      updatedWithinDays: filters.updatedWithinDays ?? 180,
    });
    return `${appRoutes.search}?${parameters.toString()}`;
  }

  const filters = search.filters as RepositoryDiscoveryRequest;
  const parameters = encodeRepositorySearchParams({
    ...createDefaultRepositoryFilters(),
    categories: filters.categories ?? [],
    excludeArchived: filters.excludeArchived ?? true,
    forkPolicy: filters.forkPolicy ?? "exclude",
    hasJapaneseReadme:
      filters.hasJapaneseReadme === undefined
        ? "any"
        : filters.hasJapaneseReadme
          ? "yes"
          : "no",
    languages: filters.languages ?? [],
    licenses: filters.licenses ?? [],
    maximumDifficulty: filters.maximumDifficulty ?? 3,
    maximumOpenIssues: filters.maximumOpenIssues ?? 500,
    minimumForks: filters.minimumForks ?? 0,
    minimumOpenIssues: filters.minimumOpenIssues ?? 1,
    minimumReadiness: filters.minimumReadiness ?? 40,
    minimumStars: filters.minimumStars ?? 10,
    technologies: filters.technologies ?? [],
    updatedWithinDays: filters.updatedWithinDays ?? 365,
  });
  return `${appRoutes.repositories}?${parameters.toString()}`;
}
