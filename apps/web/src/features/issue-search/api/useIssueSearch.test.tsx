import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../app/AppProviders";
import {
  createDefaultSearchFilters,
  type DecodedSearchLocation,
} from "../model/search-filters";
import { useIssueSearch } from "./useIssueSearch";

afterEach(() => {
  vi.unstubAllGlobals();
});

function location(
  overrides: Partial<DecodedSearchLocation> = {},
): DecodedSearchLocation {
  return {
    errors: {},
    filters: createDefaultSearchFilters("octocat"),
    shouldSearch: true,
    valid: true,
    ...overrides,
  };
}

describe("useIssueSearch", () => {
  it("posts the typed request and forwards query cancellation", async () => {
    const request = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {
            items: [],
            pagination: {
              hasNext: false,
              page: 1,
              perPage: 20,
              total: 0,
              totalPages: 0,
            },
            searchSummary: {
              candidatesChecked: 0,
              enrichmentAttempted: 0,
              enrichmentFailed: 0,
              excludedByReason: [],
              upstreamTotal: 0,
            },
            warnings: [],
          },
          meta: {
            requestId: "req_search_test",
            timestamp: "2026-07-30T00:00:00Z",
          },
        }),
        {
          headers: { "Content-Type": "application/json" },
          status: 200,
        },
      ),
    );
    vi.stubGlobal("fetch", request);

    const { result } = renderHook(() => useIssueSearch(location()), {
      wrapper: AppProviders,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    const [url, options] = request.mock.calls[0] ?? [];
    expect(url).toBe("/api/issues/search?page=1&perPage=20");
    expect(options?.signal).toBeInstanceOf(AbortSignal);
    expect(options?.body).toEqual(expect.any(String));
    expect(JSON.parse(options?.body as string)).toMatchObject({
      maximumDifficulty: 3,
      minimumStars: 10,
      username: "octocat",
    });
  });

  it("does not request data before search or for invalid URL state", () => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    renderHook(
      () =>
        useIssueSearch(
          location({
            shouldSearch: false,
          }),
        ),
      { wrapper: AppProviders },
    );
    renderHook(
      () =>
        useIssueSearch(
          location({
            errors: { form: "Invalid shared URL" },
            valid: false,
          }),
        ),
      { wrapper: AppProviders },
    );

    expect(request).not.toHaveBeenCalled();
  });
});
