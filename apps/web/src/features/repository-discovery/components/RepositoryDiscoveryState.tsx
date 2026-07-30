import { RefreshCw, Search, SearchX } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import { Skeleton } from "../../../components/ui/skeleton";
import { repositoryErrorPresentation } from "../model/repository-presentation";

export function RepositoryDiscoveryBeforeState() {
  return (
    <Card>
      <CardContent className="grid justify-items-center gap-4 p-8 text-center sm:p-12">
        <span className="grid size-14 place-items-center rounded-2xl bg-accent-soft text-accent-soft-foreground">
          <Icon className="size-6" icon={Search} />
        </span>
        <div>
          <h2 className="text-xl font-semibold">Shape an OSS shortlist</h2>
          <p className="mx-auto mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
            Choose filters, then run one bounded public GitHub search. The URL
            restores the exact conditions and page without storing result data.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

export function RepositoryDiscoveryInvalidState() {
  return (
    <Card>
      <CardContent className="grid justify-items-center gap-4 p-8 text-center sm:p-12">
        <span className="grid size-14 place-items-center rounded-2xl bg-danger-soft text-danger">
          <Icon className="size-6" icon={SearchX} />
        </span>
        <div>
          <h2 className="text-xl font-semibold">
            Fix the shared repository URL
          </h2>
          <p className="mx-auto mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
            No API request was sent. Correct the highlighted values and submit
            to replace this invalid URL with a validated one.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

export function RepositoryDiscoveryLoadingState() {
  return (
    <div
      aria-label="Discovering repositories"
      className="grid gap-5"
      role="status"
    >
      <Card>
        <CardHeader>
          <Skeleton className="h-4 w-44" />
          <Skeleton className="h-9 w-64 max-w-full" />
        </CardHeader>
      </Card>
      {[0, 1].map((index) => (
        <Card key={index}>
          <CardContent className="grid gap-4 p-6">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-8 w-4/5" />
            <Skeleton className="h-24 w-full" />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

type RepositoryDiscoveryErrorStateProps = {
  error: Error;
  isFetching: boolean;
  onRetry: () => void;
};

export function RepositoryDiscoveryErrorState({
  error,
  isFetching,
  onRetry,
}: RepositoryDiscoveryErrorStateProps) {
  const presentation = repositoryErrorPresentation(error);
  return (
    <Card className="overflow-hidden">
      <CardContent className="grid gap-6 p-7 sm:p-10">
        <span className="grid size-14 place-items-center rounded-2xl bg-warning-soft text-warning">
          <Icon className="size-6" icon={SearchX} />
        </span>
        <div>
          <p className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">
            Repository discovery
          </p>
          <CardTitle className="mt-3 text-3xl">{presentation.title}</CardTitle>
          <p className="mt-4 max-w-xl leading-7 text-muted-foreground">
            {presentation.description}
          </p>
        </div>
        {presentation.requestId ? (
          <Alert variant={presentation.tone}>
            <AlertTitle>Reference for support</AlertTitle>
            <AlertDescription>
              Request ID:{" "}
              <code className="font-mono">{presentation.requestId}</code>
            </AlertDescription>
          </Alert>
        ) : null}
        {presentation.retryable ? (
          <div>
            <Button disabled={isFetching} onClick={onRetry}>
              <Icon
                className={isFetching ? "animate-spin" : undefined}
                icon={RefreshCw}
              />
              {isFetching ? "Retrying…" : "Retry discovery"}
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
