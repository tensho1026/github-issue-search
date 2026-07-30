import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { AppProviders } from "../app/AppProviders";
import {
  createDefaultSearchFilters,
  encodeSearchParams,
} from "../features/issue-search/model/search-filters";
import { issueSearchFixture } from "../test/issue-fixtures";
import { IssueSearchPage } from "./IssueSearchPage";

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

function renderSearchPage(path: string) {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route element={<IssueSearchPage />} path="/search" />
        </Routes>
      </MemoryRouter>
    </AppProviders>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("IssueSearchPage", () => {
  it("renders the before-search state without an API request", () => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    renderSearchPage("/search?username=octocat&language=Go");

    expect(screen.getByText("Shape a realistic search")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Languages" })).toHaveTextContent(
      "Go",
    );
    expect(request).not.toHaveBeenCalled();
  });

  it("restores a valid shared search and renders API results", async () => {
    const request = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse(issueSearchFixture));
    vi.stubGlobal("fetch", request);
    const parameters = encodeSearchParams(
      createDefaultSearchFilters("octocat"),
    );

    renderSearchPage(`/search?${parameters.toString()}`);

    expect(
      await screen.findByRole("heading", {
        name: "Improve keyboard navigation in the command palette",
      }),
    ).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0]?.[0]).toBe(
      "/api/issues/search?page=1&perPage=20",
    );
  });

  it("blocks invalid shared URLs before the API boundary", async () => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    renderSearchPage("/search?username=invalid--user&page=1&page=2&search=1");

    expect(screen.getByText("Fix the shared search URL")).toBeInTheDocument();
    expect(
      screen.getByText(/shared search URL is invalid.*provided once/i),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(request).not.toHaveBeenCalled();
    });
  });
});
