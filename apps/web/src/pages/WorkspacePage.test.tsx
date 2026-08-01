import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { AppProviders } from "../app/AppProviders";
import { WorkspacePage } from "./WorkspacePage";

const meta = {
  requestId: "req_workspace",
  timestamp: "2026-08-01T00:00:00Z",
};

const authenticatedSession = {
  data: {
    authenticated: true,
    configured: true,
    csrfToken: "csrf-token",
    expiresAt: "2026-08-01T02:00:00Z",
    user: {
      accountId: "00000000-0000-4000-8000-000000000001",
      avatarUrl: "https://avatars.githubusercontent.com/u/1",
      login: "octocat",
      profileUrl: "https://github.com/octocat",
    },
  },
  meta,
};

function jsonResponse(payload: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(payload), {
      headers: { "Content-Type": "application/json" },
      status,
    }),
  );
}

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

function renderWorkspace(path = "/workspace") {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route element={<WorkspacePage />} path="/workspace" />
          <Route element={<p>Public home</p>} path="/" />
        </Routes>
      </MemoryRouter>
    </AppProviders>,
  );
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("WorkspacePage", () => {
  it("offers sign-in without redirecting an anonymous visitor", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation(() =>
      jsonResponse({
        data: { authenticated: false, configured: true },
        meta,
      }),
    );
    vi.stubGlobal("fetch", request);

    renderWorkspace();

    expect(
      await screen.findByRole("heading", {
        name: "Sign in only for saved features",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Use anonymous search" }),
    ).toHaveAttribute("href", "/search");
    expect(request).toHaveBeenCalledTimes(1);
  });

  it("keeps public recovery available when session storage fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation(() =>
        jsonResponse(
          {
            error: { code: "AUTH_UNAVAILABLE", message: "Unavailable" },
            meta,
          },
          503,
        ),
      ),
    );

    renderWorkspace();

    expect(
      await screen.findByText(
        "Account services are temporarily unavailable",
        {},
        { timeout: 3000 },
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Continue with public issue search",
      }),
    ).toHaveAttribute("href", "/search");
  });

  it("lists and deletes only the current account bookmark", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input, init) => {
      const path = requestUrl(input);
      if (path === "/api/auth/session") {
        return jsonResponse(authenticatedSession);
      }
      if (init?.method === "DELETE") {
        return jsonResponse({ data: { deleted: true }, meta });
      }
      return jsonResponse({
        data: {
          items: [
            {
              createdAt: "2026-08-01T00:00:00Z",
              id: "00000000-0000-4000-8000-000000000010",
              issueNumber: 42,
              repositoryName: "repo",
              repositoryOwner: "owner",
              targetType: "issue",
              updatedAt: "2026-08-01T00:00:00Z",
              upstreamState: "unverified",
              version: 2,
            },
          ],
          pagination: { page: 1, perPage: 50, total: 1, totalPages: 1 },
        },
        meta,
      });
    });
    vi.stubGlobal("fetch", request);
    const user = userEvent.setup();

    renderWorkspace();
    await user.click(
      await screen.findByRole("button", {
        name: "Delete bookmark owner/repo#42",
      }),
    );

    await waitFor(() => {
      expect(request).toHaveBeenCalledWith(
        "/api/account/bookmarks/00000000-0000-4000-8000-000000000010?version=2",
        expect.objectContaining({
          credentials: "include",
          method: "DELETE",
        }),
      );
    });
    const deleteOptions = request.mock.calls.find(
      ([, options]) => options?.method === "DELETE",
    )?.[1];
    expect(new Headers(deleteOptions?.headers).get("X-CSRF-Token")).toBe(
      "csrf-token",
    );
  });

  it("runs, renames, and deletes saved searches", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input, init) => {
      const path = requestUrl(input);
      if (path === "/api/auth/session") {
        return jsonResponse(authenticatedSession);
      }
      if (init?.method === "PUT") {
        return jsonResponse({
          data: {
            createdAt: "2026-08-01T00:00:00Z",
            filters: { username: "octocat" },
            id: "00000000-0000-4000-8000-000000000020",
            name: "Renamed",
            searchType: "issue",
            updatedAt: "2026-08-01T00:01:00Z",
            version: 2,
          },
          meta,
        });
      }
      if (init?.method === "DELETE") {
        return jsonResponse({ data: { deleted: true }, meta });
      }
      return jsonResponse({
        data: {
          items: [
            {
              createdAt: "2026-08-01T00:00:00Z",
              filters: { username: "octocat" },
              id: "00000000-0000-4000-8000-000000000020",
              name: "Starter issues",
              searchType: "issue",
              updatedAt: "2026-08-01T00:00:00Z",
              version: 1,
            },
          ],
          pagination: { page: 1, perPage: 50, total: 1, totalPages: 1 },
        },
        meta,
      });
    });
    vi.stubGlobal("fetch", request);
    const user = userEvent.setup();

    renderWorkspace("/workspace?tab=saved");

    expect(await screen.findByRole("link", { name: "Run" })).toHaveAttribute(
      "href",
      expect.stringMatching(/^\/search\?/),
    );
    const name = screen.getByRole("textbox", { name: /Search name/ });
    await user.clear(name);
    await user.type(name, "Renamed");
    await user.click(screen.getByRole("button", { name: "Rename" }));
    await waitFor(() => {
      expect(
        request.mock.calls.some(([, options]) => options?.method === "PUT"),
      ).toBe(true);
    });
    await user.click(
      screen.getByRole("button", {
        name: "Delete saved search Starter issues",
      }),
    );
    await waitFor(() => {
      expect(
        request.mock.calls.some(([, options]) => options?.method === "DELETE"),
      ).toBe(true);
    });
  });

  it("loads and updates versioned display preferences", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input, init) => {
      const path = requestUrl(input);
      if (path === "/api/auth/session") {
        return jsonResponse(authenticatedSession);
      }
      return jsonResponse({
        data: {
          reducedMotion: "reduce",
          resultsPerPage: 50,
          theme: "dark",
          version: init?.method === "PUT" ? 5 : 4,
        },
        meta,
      });
    });
    vi.stubGlobal("fetch", request);
    const user = userEvent.setup();

    renderWorkspace("/workspace?tab=preferences");
    await user.click(
      await screen.findByRole("button", { name: "Save preferences" }),
    );

    await waitFor(() => {
      const update = request.mock.calls.find(
        ([, options]) => options?.method === "PUT",
      );
      expect(typeof update?.[1]?.body).toBe("string");
      expect(JSON.parse(update?.[1]?.body as string)).toEqual({
        reducedMotion: "reduce",
        resultsPerPage: 50,
        theme: "dark",
        version: 4,
      });
    });
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.dataset.reducedMotion).toBe("reduce");
  });

  it("exports bounded data and requires exact destructive confirmation", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input, init) => {
      const path = requestUrl(input);
      if (path === "/api/auth/session") {
        return jsonResponse(authenticatedSession);
      }
      if (path === "/api/account/export") {
        return jsonResponse({
          data: {
            bookmarks: [],
            generatedAt: "2026-08-01T00:00:00Z",
            preferences: null,
            savedSearches: [],
            schemaVersion: 1,
          },
          meta,
        });
      }
      if (init?.method === "DELETE") {
        return jsonResponse({
          data: {
            deleted: true,
            removed: {
              bookmarks: 0,
              identities: 1,
              preferences: 0,
              savedSearches: 0,
              sessions: 1,
            },
          },
          meta,
        });
      }
      return jsonResponse({ data: {}, meta });
    });
    vi.stubGlobal("fetch", request);
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL: vi.fn(() => "blob:export"),
      revokeObjectURL: vi.fn(),
    });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(
      () => undefined,
    );
    const user = userEvent.setup();

    renderWorkspace("/workspace?tab=privacy");
    await user.click(
      await screen.findByRole("button", { name: "Download export" }),
    );
    await waitFor(() => {
      expect(request).toHaveBeenCalledWith(
        "/api/account/export",
        expect.any(Object),
      );
    });
    await user.click(screen.getByRole("button", { name: "Delete account" }));
    const confirmation = screen.getByLabelText("Type DELETE to confirm");
    const destructive = screen.getByRole("button", {
      name: "Delete permanently",
    });
    expect(destructive).toBeDisabled();
    await user.type(confirmation, "DELETE");
    expect(destructive).toBeEnabled();
    await user.click(destructive);

    expect(await screen.findByText("Public home")).toBeInTheDocument();
    const deletion = request.mock.calls.find(
      ([path, options]) =>
        path === "/api/account" && options?.method === "DELETE",
    );
    expect(deletion?.[1]?.body).toBe(
      JSON.stringify({ confirmation: "DELETE" }),
    );
  });
});
