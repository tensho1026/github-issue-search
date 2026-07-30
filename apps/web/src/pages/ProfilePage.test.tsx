import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";

import { AppProviders } from "../app/AppProviders";
import {
  errorEnvelope,
  gitHubUserFixture,
  profileAnalysisFixture,
} from "../test/profile-fixtures";
import { ProfilePage } from "./ProfilePage";

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return input;
  }
  return input instanceof URL ? input.href : input.url;
}

function renderProfile(path = "/profiles/octocat") {
  return render(
    <AppProviders>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route element={<ProfilePage />} path="/profiles/:username" />
        </Routes>
      </MemoryRouter>
    </AppProviders>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ProfilePage", () => {
  it("renders normalized profile and analysis API responses", async () => {
    const request = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = requestUrl(input);
      return Promise.resolve(
        jsonResponse(
          url.endsWith("/profile-analysis")
            ? profileAnalysisFixture
            : gitHubUserFixture,
        ),
      );
    });
    vi.stubGlobal("fetch", request);

    renderProfile();

    expect(
      await screen.findByRole("heading", { name: "The Octocat" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "TypeScript 65%" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("React")).toHaveLength(2);
    expect(screen.getByText("typed-service")).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(2);
    for (const [, options] of request.mock.calls) {
      expect(options?.signal).toBeInstanceOf(AbortSignal);
    }
  });

  it.each([
    [404, "Profile not found"],
    [429, "GitHub needs a breather"],
  ] as const)("renders the %d recovery state", async (status, title) => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(jsonResponse(errorEnvelope(status), status)),
    );

    renderProfile();

    expect(
      await screen.findByRole("heading", { name: title }),
    ).toBeInTheDocument();
  });

  it("renders explicit empty repository and framework states", async () => {
    const emptyUser = {
      ...gitHubUserFixture,
      data: { ...gitHubUserFixture.data, repositories: [] },
    };
    const emptyAnalysis = {
      ...profileAnalysisFixture,
      data: {
        ...profileAnalysisFixture.data,
        frameworks: [],
        languages: [],
      },
    };
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockImplementation((input) =>
          Promise.resolve(
            jsonResponse(
              requestUrl(input).endsWith("/profile-analysis")
                ? emptyAnalysis
                : emptyUser,
            ),
          ),
        ),
    );

    renderProfile();

    expect(
      await screen.findByText("No eligible public repositories"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/no supported framework dependency/i),
    ).toBeInTheDocument();
  });

  it("aborts both requests when the profile route becomes obsolete", async () => {
    const signals: Array<AbortSignal> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation((_input, options) => {
        const signal = options?.signal;
        if (signal instanceof AbortSignal) {
          signals.push(signal);
        }
        return new Promise<Response>((_resolve, reject) => {
          signal?.addEventListener("abort", () => {
            reject(new DOMException("Aborted", "AbortError"));
          });
        });
      }),
    );

    const view = renderProfile();
    await waitFor(() => {
      expect(signals).toHaveLength(2);
    });
    view.unmount();

    await waitFor(() => {
      expect(signals.every((signal) => signal.aborted)).toBe(true);
    });
  });
});
