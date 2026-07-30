import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppProviders } from "../../../app/AppProviders";
import type { IssueDetailContext } from "../model/issue-reference";
import { useIssueDetail } from "./useIssueDetail";

afterEach(() => {
  vi.unstubAllGlobals();
});

const context: IssueDetailContext = {
  returnTo: "/search?username=octocat&search=1",
  skills: ["TypeScript", "React"],
  valid: true,
};

describe("useIssueDetail", () => {
  it("requests the typed detail and forwards cancellation", async () => {
    const request = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {},
          meta: {
            requestId: "req_detail_test",
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

    const { result } = renderHook(
      () =>
        useIssueDetail(
          {
            issueNumber: 42,
            owner: "OpenAI",
            repository: "openai-go",
            valid: true,
          },
          context,
        ),
      { wrapper: AppProviders },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    const [url, options] = request.mock.calls[0] ?? [];
    expect(url).toBe(
      "/api/issues/OpenAI/openai-go/42?skills=TypeScript&skills=React",
    );
    expect(options?.signal).toBeInstanceOf(AbortSignal);
  });

  it("does not request data for invalid route state", () => {
    const request = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", request);

    renderHook(
      () => useIssueDetail({ message: "Invalid issue", valid: false }, context),
      { wrapper: AppProviders },
    );

    expect(request).not.toHaveBeenCalled();
  });
});
