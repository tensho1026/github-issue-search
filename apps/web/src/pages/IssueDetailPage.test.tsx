import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { AppProviders } from "../app/AppProviders";
import {
  createDefaultSearchFilters,
  encodeSearchParams,
} from "../features/issue-search/model/search-filters";
import { appRoutes, externalLinks } from "../shared/config/app-config";
import { issueDetailFixture } from "../test/issue-fixtures";
import { IssueDetailPage } from "./IssueDetailPage";

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

function renderDetailPage(path: string) {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route element={<IssueDetailPage />} path={appRoutes.issuePattern} />
          <Route element={<p>Restored search</p>} path={appRoutes.search} />
        </Routes>
      </MemoryRouter>
    </AppProviders>,
  );
}

function detailPath() {
  const search = encodeSearchParams({
    ...createDefaultSearchFilters("octocat"),
    frameworks: ["React"],
    languages: ["TypeScript"],
    page: 2,
  });
  const from = `/search?${search.toString()}`;
  return {
    from,
    path: `${appRoutes.issue("octocat", "typed-service", 42)}?${new URLSearchParams(
      { from },
    ).toString()}`,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("IssueDetailPage", () => {
  it("renders the complete recommendation and restores search context", async () => {
    const request = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse(issueDetailFixture));
    vi.stubGlobal("fetch", request);
    const route = detailPath();

    renderDetailPage(route.path);

    const title = await screen.findByRole("heading", {
      level: 1,
      name: "Improve keyboard navigation in the command palette",
    });
    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0]?.[0]).toBe(
      "/api/issues/octocat/typed-service/42?skills=TypeScript&skills=React",
    );
    expect(screen.getByRole("meter")).toHaveAccessibleName(
      "83 out of 100, Strong fit",
    );
    expect(
      screen.getByRole("heading", { name: "Issue description" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "What this work involves" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        name: "Why IssueScout recommends it",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Contributor readiness" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Maintainer activity" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Back to search results" }),
    ).toHaveAttribute("href", route.from);
    expect(
      screen.getByRole("link", { name: /open original github issue/i }),
    ).toHaveAttribute(
      "href",
      externalLinks.gitHubIssue("octocat", "typed-service", 42),
    );
    await waitFor(() => {
      expect(title).toHaveFocus();
    });
  });

  it("keeps untrusted issue markup inert in the integrated page", async () => {
    const malicious = structuredClone(issueDetailFixture);
    malicious.data.issue.body =
      "<script>globalThis.compromised=true</script>\n[unsafe](javascript:alert(1))";
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(malicious)),
    );
    const { container } = renderDetailPage(detailPath().path);

    expect(
      await screen.findByText(/<script>globalThis\.compromised=true<\/script>/),
    ).toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
    expect(screen.queryByRole("link", { name: "unsafe" })).toBeNull();
  });

  it.each([
    "/issues/bad--owner/repository/42",
    "/issues/octocat/../42",
    "/issues/octocat/repository/0",
    "/issues/octocat/repository/42?from=https%3A%2F%2Fevil.example%2Fsearch",
  ])("blocks the invalid detail URL before the API: %s", (path) => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    renderDetailPage(path);

    expect(
      screen.getByRole("heading", {
        name: "This recommendation link needs attention.",
      }),
    ).toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });

  it("renders a stable non-retryable not-found state", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        jsonResponse(
          {
            error: {
              code: "NOT_FOUND",
              message: "upstream detail omitted",
            },
            meta: {
              requestId: "req_missing_issue",
              timestamp: "2026-07-30T00:00:00Z",
            },
          },
          404,
        ),
      ),
    );

    renderDetailPage(detailPath().path);

    expect(
      await screen.findByRole("heading", { name: "Issue not found" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/req_missing_issue/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /retry recommendation/i }),
    ).toBeNull();
  });

  it("labels incomplete and unavailable activity without inventing zero", async () => {
    const partial = structuredClone(issueDetailFixture);
    partial.data.inspection.incomplete = true;
    partial.data.activity.issueResponse = {
      ...partial.data.activity.issueResponse,
      medianSeconds: null,
      percentile90Seconds: null,
      status: "unavailable",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(partial)),
    );

    renderDetailPage(detailPath().path);

    expect(
      await screen.findByText("Partial GitHub inspection"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Unavailable").length).toBeGreaterThan(0);
  });
});
