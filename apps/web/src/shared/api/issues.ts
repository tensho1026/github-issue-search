import { issueEndpoints } from "../config/app-config";
import { apiClient, type ApiClient } from "./client";
import type { IssueSearchEnvelope, IssueSearchRequest } from "./generated";

export function searchGitHubIssues(
  request: IssueSearchRequest,
  page: number,
  perPage: number,
  signal?: AbortSignal,
  client: ApiClient = apiClient,
): Promise<IssueSearchEnvelope> {
  return client.post<IssueSearchEnvelope, IssueSearchRequest>(
    issueEndpoints.search(page, perPage),
    request,
    { signal },
  );
}
