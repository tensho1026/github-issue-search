import { useMemo } from "react";
import { useLocation, useParams } from "react-router";

import { useIssueDetail } from "../features/issue-detail/api/useIssueDetail";
import { IssueDetailDashboard } from "../features/issue-detail/components/IssueDetailDashboard";
import {
  IssueDetailErrorState,
  IssueDetailInvalidState,
  IssueDetailLoadingState,
} from "../features/issue-detail/components/IssueDetailState";
import {
  decodeIssueDetailContext,
  validateIssueReference,
} from "../features/issue-detail/model/issue-reference";
import { appRoutes } from "../shared/config/app-config";

export function IssueDetailPage() {
  const route = useParams();
  const location = useLocation();
  const reference = useMemo(
    () =>
      validateIssueReference(route.owner, route.repository, route.issueNumber),
    [route.issueNumber, route.owner, route.repository],
  );
  const context = useMemo(
    () => decodeIssueDetailContext(new URLSearchParams(location.search)),
    [location.search],
  );
  const query = useIssueDetail(reference, context);
  const returnTo = context.valid ? context.returnTo : appRoutes.search;

  let content;
  if (!reference.valid) {
    content = (
      <IssueDetailInvalidState
        action={{ label: "Start a new issue search", to: appRoutes.search }}
        message={reference.message}
      />
    );
  } else if (!context.valid) {
    content = (
      <IssueDetailInvalidState
        action={{ label: "Start a new issue search", to: appRoutes.search }}
        message={context.message}
      />
    );
  } else if (query.isPending) {
    content = <IssueDetailLoadingState />;
  } else if (query.error) {
    content = (
      <IssueDetailErrorState
        error={query.error}
        isFetching={query.isFetching}
        onRetry={() => {
          void query.refetch();
        }}
        returnTo={returnTo}
      />
    );
  } else if (query.data) {
    content = (
      <IssueDetailDashboard envelope={query.data} returnTo={returnTo} />
    );
  } else {
    content = <IssueDetailLoadingState />;
  }

  return (
    <div className="mx-auto min-h-[68vh] w-full max-w-7xl px-5 py-8 sm:px-8 sm:py-12 lg:px-10">
      {content}
    </div>
  );
}
