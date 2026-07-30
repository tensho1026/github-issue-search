import { ArrowLeft, RefreshCw, SearchX } from "lucide-react";
import { Link } from "react-router";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { Button } from "../../../components/ui/button";
import { Card, CardContent } from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import { appRoutes } from "../../../shared/config/app-config";
import { profileErrorPresentation } from "../model/profile-error";

type ProfileErrorStateProps = {
  error: Error;
  isFetching: boolean;
  onRetry: () => void;
  username: string;
};

export function ProfileErrorState({
  error,
  isFetching,
  onRetry,
  username,
}: ProfileErrorStateProps) {
  const presentation = profileErrorPresentation(error, username);
  return (
    <section className="mx-auto grid min-h-[68vh] w-full max-w-3xl content-center px-5 py-16 sm:px-8">
      <Card className="overflow-hidden">
        <CardContent className="grid gap-6 p-7 sm:p-10">
          <span className="grid size-14 place-items-center rounded-2xl bg-warning-soft text-warning">
            <Icon className="size-6" icon={SearchX} />
          </span>
          <div>
            <p className="font-mono text-xs tracking-[0.16em] text-muted-foreground uppercase">
              Profile analysis
            </p>
            <h1 className="mt-3 text-3xl font-semibold tracking-[-0.045em]">
              {presentation.title}
            </h1>
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
          <div className="flex flex-wrap gap-3">
            {presentation.retryable ? (
              <Button disabled={isFetching} onClick={onRetry}>
                <Icon
                  className={isFetching ? "animate-spin" : undefined}
                  icon={RefreshCw}
                />
                {isFetching ? "Retrying…" : "Retry analysis"}
              </Button>
            ) : null}
            <Button asChild variant="outline">
              <Link to={appRoutes.home}>
                <Icon icon={ArrowLeft} />
                Try another profile
              </Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
