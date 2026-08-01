import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useMemo, type PropsWithChildren } from "react";

import type { AuthSessionEnvelope } from "../../shared/api/generated";
import { ApiError } from "../../shared/api/client";
import { authEndpoints } from "../../shared/config/app-config";
import { queryKeys } from "../../shared/query/query-keys";
import { readAuthSession } from "./api/auth";
import { AuthContext, type AuthContextValue } from "./auth-context";
import { currentSafeReturnTo, safeReturnTo } from "./model/safe-return-to";

export function AuthProvider({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const query = useQuery({
    enabled: false,
    queryFn: ({ signal }) => readAuthSession(signal),
    queryKey: queryKeys.auth.session,
    retry(failureCount, error) {
      return (
        !(error instanceof ApiError && error.status < 500) && failureCount < 1
      );
    },
    staleTime: 30_000,
  });

  const markSessionExpired = useCallback(async () => {
    queryClient.setQueryData<AuthSessionEnvelope>(
      queryKeys.auth.session,
      (current) =>
        current
          ? {
              ...current,
              data: {
                authenticated: false,
                configured: current.data.configured,
              },
            }
          : current,
    );
    await queryClient.cancelQueries({ queryKey: queryKeys.account.root });
    queryClient.removeQueries({ queryKey: queryKeys.account.root });
  }, [queryClient]);

  const signIn = useCallback((returnTo?: string) => {
    const target = returnTo ? safeReturnTo(returnTo) : currentSafeReturnTo();
    globalThis.location.assign(authEndpoints.start(target));
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      markSessionExpired,
      query,
      session: query.data?.data,
      signIn,
    }),
    [markSessionExpired, query, signIn],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
