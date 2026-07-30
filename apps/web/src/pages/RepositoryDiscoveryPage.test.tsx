import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { AppProviders } from "../app/AppProviders";
import {
  createDefaultRepositoryFilters,
  encodeRepositorySearchParams,
} from "../features/repository-discovery/model/repository-filters";
import { repositoryDiscoveryFixture } from "../test/repository-fixtures";
import { RepositoryDiscoveryPage } from "./RepositoryDiscoveryPage";

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

function renderPage(search = "") {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={[`/repositories${search}`]}>
        <Routes>
          <Route element={<RepositoryDiscoveryPage />} path="/repositories" />
        </Routes>
      </MemoryRouter>
    </AppProviders>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("RepositoryDiscoveryPage", () => {
  it("renders explainable server-ordered repository evidence", async () => {
    const request = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse(repositoryDiscoveryFixture));
    vi.stubGlobal("fetch", request);
    const search = encodeRepositorySearchParams(
      createDefaultRepositoryFilters(),
    );

    renderPage(`?${search.toString()}`);

    expect(
      await screen.findByRole("heading", {
        name: "1 eligible repositories",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /example\/typed-service/i }),
    ).toHaveAttribute("href", "https://github.com/example/typed-service");
    expect(screen.getByText(/Contribution ready · 88\/100/i)).toBeVisible();
    expect(screen.getByText("Japanese script detected")).toBeVisible();
    expect(
      screen.getAllByText(/repository evidence is partial/i).length,
    ).toBeGreaterThan(0);

    expect(request).toHaveBeenCalledTimes(1);
    const [url, options] = request.mock.calls[0] ?? [];
    expect(url).toContain("/api/repositories/search?page=1&perPage=20");
    expect(options?.method).toBe("POST");
    expect(options?.signal).toBeInstanceOf(AbortSignal);
    expect(options?.body).toEqual(expect.any(String));
    expect(JSON.parse(options?.body as string)).toMatchObject({
      excludeArchived: true,
      forkPolicy: "exclude",
      minimumReadiness: 40,
      minimumStars: 10,
    });
  });

  it("does not contact the API for an invalid shared URL", async () => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    renderPage("?search=1&minimumStars=10&minimumStars=20");

    expect(
      await screen.findByRole("heading", {
        name: "Fix the shared repository URL",
      }),
    ).toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });

  it("renders an explicit recoverable empty state", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        jsonResponse({
          ...repositoryDiscoveryFixture,
          data: {
            ...repositoryDiscoveryFixture.data,
            items: [],
            pagination: {
              ...repositoryDiscoveryFixture.data.pagination,
              total: 0,
              totalPages: 0,
            },
          },
        }),
      ),
    );
    const search = encodeRepositorySearchParams(
      createDefaultRepositoryFilters(),
    );

    renderPage(`?${search.toString()}`);
    expect(
      await screen.findByRole("heading", {
        name: "No eligible repositories found",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Broaden the filters" }),
    ).toHaveAttribute("href", "#repository-filters");
  });

  it("aborts discovery when the route becomes obsolete", async () => {
    const signals: AbortSignal[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation((_input, options) => {
        if (options?.signal instanceof AbortSignal) {
          signals.push(options.signal);
        }
        return new Promise<Response>((_resolve, reject) => {
          options?.signal?.addEventListener("abort", () => {
            reject(new DOMException("Aborted", "AbortError"));
          });
        });
      }),
    );
    const search = encodeRepositorySearchParams(
      createDefaultRepositoryFilters(),
    );
    const view = renderPage(`?${search.toString()}`);
    await waitFor(() => {
      expect(signals).toHaveLength(1);
    });
    view.unmount();
    await waitFor(() => {
      expect(signals[0]?.aborted).toBe(true);
    });
  });
});
