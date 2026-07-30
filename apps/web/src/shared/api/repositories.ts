import { repositoryEndpoints } from "../config/app-config";
import { apiClient, type ApiClient } from "./client";
import type {
  RepositoryDiscoveryEnvelope,
  RepositoryDiscoveryRequest,
} from "./generated";

export function searchGitHubRepositories(
  request: RepositoryDiscoveryRequest,
  page: number,
  perPage: number,
  signal?: AbortSignal,
  client: ApiClient = apiClient,
): Promise<RepositoryDiscoveryEnvelope> {
  return client.post<RepositoryDiscoveryEnvelope, RepositoryDiscoveryRequest>(
    repositoryEndpoints.search(page, perPage),
    request,
    { signal },
  );
}
