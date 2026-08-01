import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  CircleUserRound,
  ExternalLink,
  LayoutDashboard,
  LogOut,
  UserRoundCheck,
} from "lucide-react";
import { useEffect } from "react";
import { Link } from "react-router";

import { Button } from "../../../components/ui/button";
import { Icon } from "../../../components/ui/icon";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "../../../components/ui/popover";
import { ApiError } from "../../../shared/api/client";
import type { AuthSessionEnvelope } from "../../../shared/api/generated";
import { appRoutes } from "../../../shared/config/app-config";
import { queryKeys } from "../../../shared/query/query-keys";
import { logoutAuthSession } from "../api/auth";
import { useAuth } from "../auth-context";

export function AccountControl() {
  const { markSessionExpired, query, session, signIn } = useAuth();
  const queryClient = useQueryClient();

  useEffect(() => {
    if (query.fetchStatus === "idle" && !query.data && !query.error) {
      void query.refetch();
    }
  }, [query]);

  const logout = useMutation({
    mutationFn: async () => {
      if (!session?.authenticated || !session.csrfToken) {
        throw new ApiError({
          code: "AUTHENTICATION_REQUIRED",
          message: "Your session has expired.",
          status: 401,
        });
      }
      return logoutAuthSession(session.csrfToken);
    },
    async onSuccess() {
      await markSessionExpired();
      queryClient.setQueryData<AuthSessionEnvelope>(
        queryKeys.auth.session,
        (current) =>
          current
            ? {
                ...current,
                data: { authenticated: false, configured: true },
              }
            : current,
      );
    },
    async onError(error) {
      if (error instanceof ApiError && error.status === 401) {
        await markSessionExpired();
      }
    },
  });

  if (query.isFetching && !query.data) {
    return (
      <span
        aria-label="Checking account session"
        className="grid size-10 animate-pulse place-items-center rounded-full bg-muted text-muted-foreground motion-reduce:animate-none"
        role="status"
      >
        <Icon icon={CircleUserRound} />
      </span>
    );
  }

  if (query.error) {
    return (
      <span
        className="inline-flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-medium text-muted-foreground"
        title="Public features remain available."
      >
        <Icon icon={AlertCircle} />
        <span className="hidden lg:inline">Sign-in unavailable</span>
      </span>
    );
  }

  if (!session?.authenticated || !session.user) {
    return session?.configured === false ? null : (
      <Button
        onClick={() => {
          signIn();
        }}
        size="small"
        variant="outline"
      >
        <Icon icon={UserRoundCheck} />
        Sign in
      </Button>
    );
  }

  const { user } = session;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          aria-label={`Open account menu for ${user.login}`}
          className="gap-2 rounded-full pr-3 pl-1.5"
          size="small"
          variant="outline"
        >
          <img
            alt=""
            className="size-7 rounded-full bg-muted object-cover"
            height={28}
            referrerPolicy="no-referrer"
            src={user.avatarUrl}
            width={28}
          />
          <span className="hidden max-w-28 truncate lg:inline">
            {user.login}
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="grid w-72 gap-4">
        <div className="min-w-0">
          <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            Signed in with GitHub
          </p>
          <p className="mt-1 truncate font-semibold">{user.login}</p>
        </div>
        <div className="grid gap-1 border-y border-border py-2">
          <Button asChild className="justify-start" variant="ghost">
            <Link to={appRoutes.workspace}>
              <Icon icon={LayoutDashboard} />
              Account workspace
            </Link>
          </Button>
          <Button asChild className="justify-start" variant="ghost">
            <a href={user.profileUrl} rel="noreferrer" target="_blank">
              <Icon icon={ExternalLink} />
              View GitHub profile
            </a>
          </Button>
        </div>
        {logout.error ? (
          <p className="text-xs leading-5 text-danger" role="alert">
            {logout.error instanceof ApiError && logout.error.status === 401
              ? "Your session expired. Sign in again when you are ready."
              : "Logout could not be confirmed. Your public work is unchanged."}
          </p>
        ) : null}
        <Button
          className="justify-start"
          disabled={logout.isPending}
          onClick={() => {
            logout.mutate();
          }}
          variant="ghost"
        >
          <Icon icon={LogOut} />
          {logout.isPending ? "Signing out…" : "Sign out"}
        </Button>
      </PopoverContent>
    </Popover>
  );
}
