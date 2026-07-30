import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { searchGitHubIssues } from "../../../shared/api/issues";
import { queryKeys } from "../../../shared/query/query-keys";
import {
  encodeSearchParams,
  toIssueSearchRequest,
  type DecodedSearchLocation,
} from "../model/search-filters";

export function useIssueSearch(location: DecodedSearchLocation) {
  const enabled = location.shouldSearch && location.valid;
  const canonicalSearch = enabled
    ? encodeSearchParams(location.filters).toString()
    : "disabled";

  return useQuery({
    enabled,
    placeholderData: keepPreviousData,
    queryFn: ({ signal }) =>
      searchGitHubIssues(
        toIssueSearchRequest(location.filters),
        location.filters.page,
        location.filters.perPage,
        signal,
      ),
    queryKey: queryKeys.issues.search(canonicalSearch),
  });
}
