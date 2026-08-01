import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState, type PropsWithChildren } from "react";

import { appConfig } from "../shared/config/app-config";
import { shouldRetryQuery } from "../shared/query/retry-policy";
import { TooltipProvider } from "../components/ui/tooltip";
import { AuthProvider } from "../features/auth/AuthProvider";

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: {
        gcTime: appConfig.query.garbageCollectionTimeMs,
        refetchOnWindowFocus: false,
        retry: shouldRetryQuery,
        staleTime: appConfig.query.staleTimeMs,
      },
    },
  });

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(createQueryClient);

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <TooltipProvider delayDuration={300}>{children}</TooltipProvider>
      </AuthProvider>
    </QueryClientProvider>
  );
}
