import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { issueSearchFixture } from "../../../test/issue-fixtures";
import { IssueSearchResults } from "./IssueSearchResults";

describe("IssueSearchResults", () => {
  it("preserves API order and renders every required recommendation field", () => {
    render(
      <MemoryRouter>
        <IssueSearchResults
          envelope={issueSearchFixture}
          isFetching={false}
          onPageChange={vi.fn()}
        />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("heading", {
        name: "Improve keyboard navigation in the command palette",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("meter")).toHaveAccessibleName(
      "83 out of 100, Strong fit",
    );
    expect(screen.getByText("Difficulty 3: Intermediate")).toBeInTheDocument();
    expect(screen.getByText("Half a day")).toBeInTheDocument();
    expect(screen.getByText("good first issue")).toBeInTheDocument();
    expect(screen.getByText("TypeScript: matched")).toBeInTheDocument();
    expect(
      screen.getByText("The primary language matches your profile."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Maintainer response evidence is partial."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "View recommendation details" }),
    ).toHaveAttribute("href", "/issues/octocat/typed-service/42");
    const externalLink = screen.getByRole("link", {
      name: /Open GitHub issue/,
    });
    expect(externalLink).toHaveAttribute(
      "href",
      "https://github.com/octocat/typed-service/issues/42",
    );
    expect(externalLink).toHaveAttribute("rel", "noreferrer");
    expect(externalLink).toHaveAttribute("target", "_blank");
  });

  it("uses server pagination metadata without reordering items", async () => {
    const user = userEvent.setup();
    const onPageChange = vi.fn();
    render(
      <MemoryRouter>
        <IssueSearchResults
          envelope={issueSearchFixture}
          isFetching={false}
          onPageChange={onPageChange}
        />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: "Go to page 2" }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it("renders an actionable no-results state", () => {
    render(
      <MemoryRouter>
        <IssueSearchResults
          envelope={{
            ...issueSearchFixture,
            data: {
              ...issueSearchFixture.data,
              items: [],
              pagination: {
                hasNext: false,
                page: 1,
                perPage: 20,
                total: 0,
                totalPages: 0,
              },
            },
          }}
          isFetching={false}
          onPageChange={vi.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText("No eligible issues found")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Broaden the filters" }),
    ).toHaveAttribute("href", "#search-filters");
  });

  it("recovers when a shared page exceeds the changed server result set", async () => {
    const user = userEvent.setup();
    const onPageChange = vi.fn();
    render(
      <MemoryRouter>
        <IssueSearchResults
          envelope={{
            ...issueSearchFixture,
            data: {
              ...issueSearchFixture.data,
              items: [],
              pagination: {
                hasNext: false,
                page: 3,
                perPage: 20,
                total: 21,
                totalPages: 2,
              },
            },
          }}
          isFetching={false}
          onPageChange={onPageChange}
        />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: "Return to page 1" }));

    expect(onPageChange).toHaveBeenCalledWith(1);
    expect(
      screen.getByText("This result page is no longer available"),
    ).toBeInTheDocument();
  });
});
