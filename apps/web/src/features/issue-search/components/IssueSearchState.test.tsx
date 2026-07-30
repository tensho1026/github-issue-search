import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../../../shared/api/client";
import {
  IssueSearchErrorState,
  IssueSearchLoadingState,
} from "./IssueSearchState";

describe("IssueSearchState", () => {
  it("renders a named loading region", () => {
    render(<IssueSearchLoadingState />);

    expect(
      screen.getByRole("status", { name: "Searching ranked issues" }),
    ).toBeInTheDocument();
  });

  it.each([
    [404, "Profile not found"],
    [429, "GitHub needs a breather"],
  ] as const)("renders non-retryable %d recovery guidance", (status, title) => {
    render(
      <IssueSearchErrorState
        error={
          new ApiError({
            code: "UPSTREAM_ERROR",
            message: "transport detail",
            requestId: "req_recovery",
            status,
          })
        }
        isFetching={false}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByText(title)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry search" })).toBeNull();
    expect(screen.getByText(/req_recovery/)).toBeInTheDocument();
  });

  it("offers one explicit retry for an upstream failure", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(
      <IssueSearchErrorState
        error={
          new ApiError({
            code: "GITHUB_UPSTREAM_ERROR",
            message: "transport detail",
            status: 502,
          })
        }
        isFetching={false}
        onRetry={onRetry}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Retry search" }));

    expect(onRetry).toHaveBeenCalledOnce();
    expect(
      screen.getByText("GitHub search is unavailable"),
    ).toBeInTheDocument();
  });
});
