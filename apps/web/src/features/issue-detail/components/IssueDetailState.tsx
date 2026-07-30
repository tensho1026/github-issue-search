import { RefreshCw } from "lucide-react";
import { Link } from "react-router";

import { Button } from "../../../components/ui/button";
import { Card, CardContent } from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import { Skeleton } from "../../../components/ui/skeleton";
import { detailErrorPresentation } from "../model/detail-presentation";

type DetailAction = {
  label: string;
  to: string;
};

export function IssueDetailInvalidState({
  action,
  message,
}: {
  action: DetailAction;
  message: string;
}) {
  return (
    <Card>
      <CardContent className="grid justify-items-start gap-5 p-8 sm:p-10">
        <div>
          <p className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">
            Invalid detail URL
          </p>
          <h1 className="mt-3 text-3xl font-semibold tracking-[-0.045em]">
            Check this recommendation link.
          </h1>
          <p className="mt-3 max-w-xl leading-7 text-muted-foreground">
            {message} No request was sent.
          </p>
        </div>
        <Button asChild>
          <Link to={action.to}>{action.label}</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

export function IssueDetailLoadingState() {
  return (
    <div
      aria-label="Loading issue recommendation"
      className="grid gap-5 py-8"
      role="status"
    >
      <Skeleton className="h-4 w-44" />
      <Skeleton className="h-12 w-full max-w-3xl" />
      <Skeleton className="h-24" />
    </div>
  );
}

export function IssueDetailErrorState({
  error,
  isFetching,
  onRetry,
  returnTo,
}: {
  error: Error;
  isFetching: boolean;
  onRetry: () => void;
  returnTo: string;
}) {
  const presentation = detailErrorPresentation(error);
  return (
    <Card className="overflow-hidden">
      <CardContent className="grid gap-6 p-8 sm:p-10">
        <div>
          <p className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">
            Issue recommendation
          </p>
          <h1 className="mt-3 text-3xl font-semibold tracking-[-0.045em]">
            {presentation.title}
          </h1>
          <p className="mt-4 max-w-xl leading-7 text-muted-foreground">
            {presentation.description}
          </p>
        </div>
        {presentation.requestId ? (
          <p className="text-sm text-muted-foreground">
            Request ID:{" "}
            <code className="font-mono">{presentation.requestId}</code>
          </p>
        ) : null}
        <div className="flex flex-wrap gap-3">
          {presentation.retryable ? (
            <Button disabled={isFetching} onClick={onRetry}>
              <Icon
                className={isFetching ? "animate-spin" : undefined}
                icon={RefreshCw}
              />
              {isFetching ? "Retrying…" : "Retry recommendation"}
            </Button>
          ) : null}
          <Button asChild variant="outline">
            <Link to={returnTo}>Back to issue search</Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
