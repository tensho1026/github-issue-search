import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../app/AppProviders";
import { AccountControl } from "../../auth/components/AccountControl";
import { BookmarkAction } from "./BookmarkAction";
import { SaveSearchAction } from "./SaveSearchAction";

const meta = {
  requestId: "req_action",
  timestamp: "2026-08-01T00:00:00Z",
};

function jsonResponse(payload: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(payload), {
      headers: { "Content-Type": "application/json" },
      status: 200,
    }),
  );
}

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("optional account actions", () => {
  it("explains an anonymous bookmark without mutating account storage", async () => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);
    const user = userEvent.setup();

    render(
      <AppProviders>
        <MemoryRouter initialEntries={["/issues/owner/repo/42"]}>
          <BookmarkAction
            request={{
              issueNumber: 42,
              repositoryName: "repo",
              repositoryOwner: "owner",
              targetType: "issue",
            }}
          />
        </MemoryRouter>
      </AppProviders>,
    );

    await user.click(screen.getByRole("button", { name: "Bookmark" }));
    expect(
      screen.getByRole("heading", {
        name: "Save this reference to your workspace",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(/return to this exact page/)).toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });

  it("saves bookmarks and named filters with the in-memory CSRF token", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input) => {
      const path = requestUrl(input);
      if (path === "/api/auth/session") {
        return jsonResponse({
          data: {
            authenticated: true,
            configured: true,
            csrfToken: "csrf-action",
            user: {
              accountId: "00000000-0000-4000-8000-000000000001",
              avatarUrl: "https://avatars.githubusercontent.com/u/1",
              login: "octocat",
              profileUrl: "https://github.com/octocat",
            },
          },
          meta,
        });
      }
      if (path.startsWith("/api/account/bookmarks")) {
        return jsonResponse({
          data: {
            createdAt: meta.timestamp,
            id: "bookmark",
            issueNumber: 42,
            repositoryName: "repo",
            repositoryOwner: "owner",
            targetType: "issue",
            updatedAt: meta.timestamp,
            upstreamState: "unverified",
            version: 1,
          },
          meta,
        });
      }
      return jsonResponse({
        data: {
          createdAt: meta.timestamp,
          filters: { username: "octocat" },
          id: "saved",
          name: "My search",
          searchType: "issue",
          updatedAt: meta.timestamp,
          version: 1,
        },
        meta,
      });
    });
    vi.stubGlobal("fetch", request);
    const user = userEvent.setup();

    render(
      <AppProviders>
        <MemoryRouter>
          <AccountControl />
          <BookmarkAction
            request={{
              issueNumber: 42,
              repositoryName: "repo",
              repositoryOwner: "owner",
              targetType: "issue",
            }}
          />
          <SaveSearchAction
            filters={{ username: "octocat" }}
            searchType="issue"
          />
        </MemoryRouter>
      </AppProviders>,
    );

    await screen.findByRole("button", {
      name: "Open account menu for octocat",
    });
    await user.click(screen.getByRole("button", { name: "Bookmark" }));
    expect(
      await screen.findByRole("button", { name: "Bookmarked" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Save this search" }));
    await user.type(screen.getByLabelText("Saved-search name"), "My search");
    await user.click(screen.getByRole("button", { name: "Save search" }));
    expect(
      await screen.findByRole("button", { name: "Search saved" }),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(
        request.mock.calls.filter(([path]) =>
          requestUrl(path).startsWith("/api/account/"),
        ),
      ).toHaveLength(2);
    });
    for (const [path, options] of request.mock.calls.filter(([path]) =>
      requestUrl(path).startsWith("/api/account/"),
    )) {
      expect(path).toEqual(expect.stringMatching(/^\/api\/account\//));
      expect(new Headers(options?.headers).get("X-CSRF-Token")).toBe(
        "csrf-action",
      );
    }
  });
});
