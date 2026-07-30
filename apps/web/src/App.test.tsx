import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("renders the IssueScout application shell", () => {
    render(<App />);

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: /find the issue you can finish/i,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "IssueScout home" }),
    ).toBeInTheDocument();
  });
});
