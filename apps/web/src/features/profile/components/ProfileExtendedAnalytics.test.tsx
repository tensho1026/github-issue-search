import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { profileAnalysisFixture } from "../../../test/profile-fixtures";
import { ProfileExtendedAnalytics } from "./ProfileExtendedAnalytics";

describe("ProfileExtendedAnalytics", () => {
  it("renders contribution, recency, proficiency, and repository evidence", () => {
    render(<ProfileExtendedAnalytics analysis={profileAnalysisFixture.data} />);

    expect(
      screen.getByRole("heading", { name: "Public contribution activity" }),
    ).toBeInTheDocument();
    expect(screen.getByText("18")).toBeInTheDocument();
    expect(screen.getAllByText("Sampled").length).toBeGreaterThan(0);
    expect(
      screen.getByRole("heading", {
        name: "Recently observed technologies",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", {
        name: "TypeScript diagnostic level 2 of 5",
      }),
    ).toHaveAttribute("aria-valuenow", "2");
    expect(
      screen.getByRole("heading", { name: "Repository source evidence" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/starred totals can be privacy-ambiguous/i),
    ).toBeVisible();
  });

  it("distinguishes unavailable evidence from zero activity", () => {
    render(
      <ProfileExtendedAnalytics
        analysis={{
          ...profileAnalysisFixture.data,
          contributions: {
            ...profileAnalysisFixture.data.contributions,
            commits: { status: "unavailable", value: 0 },
          },
          proficiency: [],
          recentTechnologies: [],
          repositoryEvidence: {
            ...profileAnalysisFixture.data.repositoryEvidence,
            starred: {
              ...profileAnalysisFixture.data.repositoryEvidence.starred,
              observed: 0,
              status: "unavailable",
            },
          },
        }}
      />,
    );

    expect(screen.getAllByText("Unavailable").length).toBeGreaterThan(1);
    expect(
      screen.getByText(/No five-level diagnostic could be supported/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/No recently observed technology evidence/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/GitHub did not provide this public segment/i),
    ).toBeInTheDocument();
  });
});
