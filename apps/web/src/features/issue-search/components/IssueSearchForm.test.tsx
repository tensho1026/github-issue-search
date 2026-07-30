import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { createDefaultSearchFilters } from "../model/search-filters";
import { IssueSearchForm } from "./IssueSearchForm";

describe("IssueSearchForm", () => {
  it("submits every MVP filter in normalized form and resets pagination", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <IssueSearchForm
        defaultValues={{
          ...createDefaultSearchFilters(" OctoCat "),
          page: 4,
        }}
        onSubmit={onSubmit}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Languages" }));
    await user.click(screen.getByRole("checkbox", { name: "Go" }));
    await user.keyboard("{Escape}");

    await user.click(screen.getByRole("button", { name: "Frameworks" }));
    await user.click(screen.getByRole("checkbox", { name: "React" }));
    await user.keyboard("{Escape}");

    await user.clear(
      screen.getByRole("spinbutton", { name: "Minimum repository stars" }),
    );
    await user.type(
      screen.getByRole("spinbutton", { name: "Minimum repository stars" }),
      "25",
    );
    await user.clear(
      screen.getByRole("spinbutton", { name: "Updated within days" }),
    );
    await user.type(
      screen.getByRole("spinbutton", { name: "Updated within days" }),
      "90",
    );
    fireEvent.change(
      screen.getByRole("slider", { name: "Maximum difficulty" }),
      { target: { value: "4" } },
    );
    await user.click(
      screen.getByRole("checkbox", { name: /Include documentation/ }),
    );
    await user.click(
      screen.getByRole("button", { name: "Find ranked issues" }),
    );

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        frameworks: ["React"],
        includeDocumentation: true,
        languages: ["Go"],
        maximumDifficulty: 4,
        minimumStars: 25,
        page: 1,
        updatedWithinDays: 90,
        username: "OctoCat",
      }),
    );
  });

  it("blocks invalid usernames and exposes validation text", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <IssueSearchForm
        defaultValues={createDefaultSearchFilters("invalid--user")}
        onSubmit={onSubmit}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Find ranked issues" }),
    );

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/single hyphens/i);
  });

  it("renders invalid shared URL feedback without discarding editable defaults", () => {
    render(
      <IssueSearchForm
        defaultValues={createDefaultSearchFilters("octocat")}
        locationErrors={{
          form: "The shared search URL is invalid.",
        }}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/shared search URL/i);
    expect(
      screen.getByRole("textbox", { name: "GitHub username" }),
    ).toHaveValue("octocat");
  });
});
