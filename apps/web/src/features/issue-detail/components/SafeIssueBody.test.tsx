import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { normalizeIssueBody } from "../model/safe-issue-body";
import { SafeIssueBody } from "./SafeIssueBody";

describe("SafeIssueBody", () => {
  it("renders untrusted markup and unsafe links only as text", () => {
    const body =
      '<script>globalThis.pwned = true</script>\n[click](javascript:alert("x"))';
    const { container } = render(<SafeIssueBody body={body} />);

    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("a")).toBeNull();
    expect(
      screen.getByText(/<script>globalThis\.pwned = true<\/script>/),
    ).toBeInTheDocument();
    expect(screen.getByText(/javascript:alert/)).toBeInTheDocument();
  });

  it("normalizes controls and reports bounded content", () => {
    expect(normalizeIssueBody("line one\r\nline\u0000two")).toEqual({
      text: "line one\nline�two",
      truncated: false,
    });
    const content = normalizeIssueBody("a".repeat(65_537));
    expect(content.truncated).toBe(true);
    expect(content.text).toContain("[Content truncated by IssueScout]");
  });

  it("makes an empty upstream body explicit", () => {
    render(<SafeIssueBody body="" />);

    expect(
      screen.getByText("No public issue description was provided."),
    ).toBeInTheDocument();
  });
});
