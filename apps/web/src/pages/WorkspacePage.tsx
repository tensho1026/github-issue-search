import { useEffect } from "react";
import { Link, useNavigate } from "react-router";

import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import { WorkspaceDashboard } from "../features/account/components/WorkspaceDashboard";
import { useAuth } from "../features/auth/auth-context";
import { appRoutes } from "../shared/config/app-config";

export function WorkspacePage() {
  const navigate = useNavigate();
  const { markSessionExpired, query, session, signIn } = useAuth();

  useEffect(() => {
    if (query.fetchStatus === "idle" && !query.data && !query.error) {
      void query.refetch();
    }
  }, [query]);

  if (query.isFetching && !query.data) {
    return (
      <div
        aria-label="Checking secure account session"
        className="mx-auto grid min-h-[68vh] w-full max-w-7xl place-content-center gap-3 px-5 text-center"
        role="status"
      >
        <p className="font-semibold">Checking your secure session…</p>
      </div>
    );
  }

  if (query.error) {
    return (
      <div className="mx-auto grid min-h-[68vh] w-full max-w-3xl content-center gap-5 px-5 py-12">
        <Alert variant="warning">
          <AlertTitle>Account services are temporarily unavailable</AlertTitle>
          <AlertDescription>
            Session or database status could not be confirmed. Anonymous profile
            analysis, issue search, and repository discovery remain available.
          </AlertDescription>
        </Alert>
        <Button asChild variant="outline">
          <Link to={appRoutes.search}>Continue with public issue search</Link>
        </Button>
      </div>
    );
  }

  if (!session?.authenticated || !session.user || !session.csrfToken) {
    return (
      <div className="mx-auto grid min-h-[68vh] w-full max-w-3xl content-center px-5 py-12">
        <Card>
          <CardHeader>
            <CardTitle className="text-2xl">
              Sign in only for saved features
            </CardTitle>
            <CardDescription>
              Bookmarks, saved searches, preferences, exports, and account
              deletion require a secure GitHub session. Public journeys never
              do.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-3">
            {session?.configured === false ? (
              <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
                Sign-in is not configured in this environment.
              </span>
            ) : (
              <Button
                onClick={() => signIn(`${appRoutes.workspace}?tab=bookmarks`)}
              >
                Sign in with GitHub
              </Button>
            )}
            <Button asChild variant="outline">
              <Link to={appRoutes.search}>Use anonymous search</Link>
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <WorkspaceDashboard
      csrfToken={session.csrfToken}
      onAccountDeleted={async () => {
        await markSessionExpired();
        void navigate(appRoutes.home, { replace: true });
      }}
      onSessionExpired={markSessionExpired}
      user={session.user}
    />
  );
}
