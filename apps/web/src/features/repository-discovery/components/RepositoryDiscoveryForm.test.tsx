import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  createDefaultRepositoryFilters,
  type RepositoryFilters,
} from "../model/repository-filters";
import { RepositoryDiscoveryForm } from "./RepositoryDiscoveryForm";

describe("RepositoryDiscoveryForm", () => {
  it("submits normalized values and returns pagination to page one", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<(filters: RepositoryFilters) => void>();
    render(
      <RepositoryDiscoveryForm
        defaultValues={{
          ...createDefaultRepositoryFilters(),
          languages: ["TypeScript"],
          page: 4,
        }}
        onSubmit={onSubmit}
      />,
    );

    const stars = screen.getByRole("spinbutton", { name: "Minimum stars" });
    await user.clear(stars);
    await user.type(stars, "75");
    await user.click(
      screen.getByRole("button", { name: "Discover repositories" }),
    );

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        languages: ["TypeScript"],
        minimumStars: 75,
        page: 1,
      }),
    );
  });

  it("renders shared URL errors and blocks invalid ranges", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<(filters: RepositoryFilters) => void>();
    render(
      <RepositoryDiscoveryForm
        defaultValues={{
          ...createDefaultRepositoryFilters(),
          maximumOpenIssues: 1,
          minimumOpenIssues: 2,
        }}
        locationErrors={{
          form: "The shared repository URL is invalid.",
          maximumOpenIssues:
            "Maximum open issues must be at least the minimum open issues.",
        }}
        onSubmit={onSubmit}
      />,
    );

    expect(
      screen.getByText("The shared repository URL is invalid."),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Discover repositories" }),
    );
    expect(onSubmit).not.toHaveBeenCalled();
    expect(
      screen.getByText(/maximum open issues must be at least/i),
    ).toBeInTheDocument();
  });

  it("restores safe defaults", async () => {
    const user = userEvent.setup();
    render(
      <RepositoryDiscoveryForm
        defaultValues={{
          ...createDefaultRepositoryFilters(),
          minimumStars: 500,
        }}
        onSubmit={vi.fn()}
      />,
    );
    const stars = screen.getByRole("spinbutton", { name: "Minimum stars" });
    expect(stars).toHaveValue(500);
    await user.click(screen.getByRole("button", { name: "Reset filters" }));
    expect(stars).toHaveValue(10);
  });
});
