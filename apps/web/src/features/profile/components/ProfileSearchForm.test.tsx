import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";

import { ProfileSearchForm } from "./ProfileSearchForm";

function renderForm() {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <Routes>
        <Route element={<ProfileSearchForm />} path="/" />
        <Route
          element={<h1>Profile destination</h1>}
          path="/profiles/:username"
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ProfileSearchForm", () => {
  it("rejects an invalid GitHub username before navigation", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(
      screen.getByRole("textbox", { name: "GitHub username" }),
      "invalid--user",
    );
    await user.click(screen.getByRole("button", { name: /analyze profile/i }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      /letters, numbers, or single hyphens/i,
    );
    expect(
      screen.queryByRole("heading", { name: "Profile destination" }),
    ).not.toBeInTheDocument();
  });

  it("normalizes a valid username into route state", async () => {
    const user = userEvent.setup();
    renderForm();

    await user.type(
      screen.getByRole("textbox", { name: "GitHub username" }),
      "  octocat  ",
    );
    await user.click(screen.getByRole("button", { name: /analyze profile/i }));

    expect(
      screen.getByRole("heading", { name: "Profile destination" }),
    ).toBeInTheDocument();
  });
});
