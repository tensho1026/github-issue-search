import { issueEndpoints } from "../config/app-config";
import { apiClient, type ApiClient } from "./client";
import type {
  IssueDetailEnvelope,
  IssueSearchEnvelope,
  IssueSearchRequest,
} from "./generated";

export function getIssueDetail(
  owner: string,
  repository: string,
  issueNumber: number,
  skills: readonly string[],
  signal?: AbortSignal,
  client: ApiClient = apiClient,
): Promise<IssueDetailEnvelope> {
  return client.get<IssueDetailEnvelope>(
    issueEndpoints.detail(owner, repository, issueNumber, skills),
    { signal },
  );
}

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
