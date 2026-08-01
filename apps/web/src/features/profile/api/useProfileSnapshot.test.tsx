import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  gitHubUserFixture,
  profileAnalysisFixture,
} from "../../../test/profile-fixtures";
import { useProfileSnapshot } from "./useProfileSnapshot";

type PendingRequest = {
  resolve: () => void;
  signal: AbortSignal | undefined;
  url: string;
};

function requestURL(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

function queryWrapper(queryClient: QueryClient) {
  return function TestQueryProvider({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useProfileSnapshot", () => {
  it("starts both requests concurrently and reuses canonical cached data", async () => {
    const pending: PendingRequest[] = [];
    const request = vi.fn<typeof fetch>().mockImplementation(
      (input, options) =>
        new Promise<Response>((resolve) => {
          const url = requestURL(input);
          pending.push({
            resolve: () => {
              resolve(
                new Response(
                  JSON.stringify(
                    url.endsWith("/profile-analysis")
                      ? profileAnalysisFixture
                      : gitHubUserFixture,
                  ),
                  {
                    headers: { "Content-Type": "application/json" },
                    status: 200,
                  },
                ),
              );
            },
            signal: options?.signal ?? undefined,
            url,
          });
        }),
    );
    vi.stubGlobal("fetch", request);
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
          staleTime: Number.POSITIVE_INFINITY,
        },
      },
    });

    const { rerender, result } = renderHook(
      ({ username }) => useProfileSnapshot(username),
      {
        initialProps: { username: "octocat" },
        wrapper: queryWrapper(queryClient),
      },
    );

    // Neither dependency is resolved yet. Observing both pending requests
    // proves the hook does not serialize profile analysis behind the user call.
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });
    expect(pending.map(({ url }) => url).sort()).toEqual([
      "/api/github/users/octocat",
      "/api/github/users/octocat/profile-analysis",
    ]);
    expect(pending.every(({ signal }) => signal instanceof AbortSignal)).toBe(
      true,
    );

    act(() => {
      for (const request of pending) {
        request.resolve();
      }
    });
    await waitFor(() => {
      expect(result.current.snapshot?.user.login).toBe("octocat");
      expect(result.current.snapshot?.analysis.username).toBe("octocat");
    });

    rerender({ username: "OCTOCAT" });
    await waitFor(() => {
      expect(result.current.snapshot).toBeDefined();
    });
    expect(request).toHaveBeenCalledTimes(2);
  });
});
