import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createSavedSearch,
  deleteAccount,
  deleteBookmark,
  deleteSavedSearch,
  exportAccount,
  getPreferences,
  listBookmarks,
  listSavedSearches,
  updatePreferences,
  updateSavedSearch,
  upsertBookmark,
} from "./account";

function response() {
  return Promise.resolve(
    new Response(
      JSON.stringify({
        data: { deleted: true, items: [], loggedOut: true },
        meta: { requestId: "req", timestamp: "2026-08-01T00:00:00Z" },
      }),
      { headers: { "Content-Type": "application/json" }, status: 200 },
    ),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("account API", () => {
  it("uses cookie credentials, owned paths, and CSRF on every mutation", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation(response);
    vi.stubGlobal("fetch", request);
    const signal = new AbortController().signal;

    await listBookmarks(signal);
    await upsertBookmark(
      {
        issueNumber: 42,
        repositoryName: "repo",
        repositoryOwner: "owner",
        targetType: "issue",
      },
      "csrf",
    );
    await deleteBookmark("bookmark", 2, "csrf");
    await listSavedSearches(signal);
    await createSavedSearch(
      {
        filters: { username: "octocat" },
        name: "Starter issues",
        searchType: "issue",
      },
      "csrf",
    );
    await updateSavedSearch(
      "saved",
      {
        filters: {},
        name: "Repositories",
        searchType: "repository",
        version: 3,
      },
      "csrf",
    );
    await deleteSavedSearch("saved", 3, "csrf");
    await getPreferences(signal);
    await updatePreferences(
      {
        reducedMotion: "reduce",
        resultsPerPage: 20,
        theme: "dark",
        version: 1,
      },
      "csrf",
    );
    await exportAccount(signal);
    await deleteAccount("csrf");

    expect(request).toHaveBeenCalledTimes(11);
    expect(request.mock.calls.map(([path]) => path)).toEqual([
      "/api/account/bookmarks?page=1&perPage=50",
      "/api/account/bookmarks?page=1&perPage=50",
      "/api/account/bookmarks/bookmark?version=2",
      "/api/account/saved-searches?page=1&perPage=50",
      "/api/account/saved-searches?page=1&perPage=50",
      "/api/account/saved-searches/saved",
      "/api/account/saved-searches/saved?version=3",
      "/api/account/preferences",
      "/api/account/preferences",
      "/api/account/export",
      "/api/account",
    ]);
    for (const [, options] of request.mock.calls) {
      expect(options?.credentials).toBe("include");
    }
    for (const index of [1, 2, 4, 5, 6, 8, 10]) {
      expect(
        new Headers(request.mock.calls[index]?.[1]?.headers).get(
          "X-CSRF-Token",
        ),
      ).toBe("csrf");
    }
    expect(request.mock.calls[10]?.[1]?.body).toBe(
      JSON.stringify({ confirmation: "DELETE" }),
    );
  });
});
