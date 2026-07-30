import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("renders the IssueScout application shell", async () => {
    render(<App />);

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: /your next contribution, decoded/i,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "IssueScout home" }),
    ).toBeInTheDocument();
  });
});
