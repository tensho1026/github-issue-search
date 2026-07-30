import { describe, expect, it } from "vitest";

import { queryKeys } from "./query-keys";

describe("issue query keys", () => {
  it("normalizes case-insensitive repository references", () => {
    expect(queryKeys.issues.detail("OpenAI", "SDK", 42, ["Go"])).toEqual(
      queryKeys.issues.detail("openai", "sdk", 42, ["Go"]),
    );
  });

  it("preserves skill order because it is observable in the API response", () => {
    expect(
      queryKeys.issues.detail("openai", "sdk", 42, ["Go", "React"]),
    ).not.toEqual(
      queryKeys.issues.detail("openai", "sdk", 42, ["React", "Go"]),
    );
  });
});
