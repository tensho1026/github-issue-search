import { useQuery } from "@tanstack/react-query";

import { getIssueDetail } from "../../../shared/api/issues";
import { queryKeys } from "../../../shared/query/query-keys";
import type {
  IssueDetailContext,
  IssueReference,
} from "../model/issue-reference";

export function useIssueDetail(
  reference: IssueReference,
  context: IssueDetailContext,
) {
  const enabled = reference.valid && context.valid;
  const owner = reference.valid ? reference.owner : "";
  const repository = reference.valid ? reference.repository : "";
  const issueNumber = reference.valid ? reference.issueNumber : 0;
  const skills = context.valid ? context.skills : [];

  return useQuery({
    enabled,
    queryFn: ({ signal }) =>
      getIssueDetail(owner, repository, issueNumber, skills, signal),
    queryKey: queryKeys.issues.detail(owner, repository, issueNumber, skills),
  });
}
