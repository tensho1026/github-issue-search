import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { searchGitHubRepositories } from "../../../shared/api/repositories";
import { queryKeys } from "../../../shared/query/query-keys";
import {
  encodeRepositorySearchParams,
  toRepositoryDiscoveryRequest,
  type DecodedRepositoryLocation,
} from "../model/repository-filters";

export function useRepositoryDiscovery(location: DecodedRepositoryLocation) {
  const enabled = location.shouldSearch && location.valid;
  const canonicalSearch = enabled
    ? encodeRepositorySearchParams(location.filters).toString()
    : "disabled";

  return useQuery({
    enabled,
    placeholderData: keepPreviousData,
    queryFn: ({ signal }) =>
      searchGitHubRepositories(
        toRepositoryDiscoveryRequest(location.filters),
        location.filters.page,
        location.filters.perPage,
        signal,
      ),
    queryKey: queryKeys.repositories.search(canonicalSearch),
  });
}
