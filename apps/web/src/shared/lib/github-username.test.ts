import { describe, expect, it } from "vitest";

import {
  gitHubUsernameLimits,
  validateGitHubUsername,
} from "./github-username";

describe("validateGitHubUsername", () => {
  it.each(["octocat", "user-42", "A"])(
    "accepts the backend-compatible username %s",
    (username) => {
      expect(validateGitHubUsername(` ${username} `)).toEqual({
        username,
        valid: true,
      });
    },
  );

  it.each([
    ["", "empty"],
    ["-octocat", "invalid"],
    ["octocat-", "invalid"],
    ["octo--cat", "invalid"],
    ["octo_cat", "invalid"],
    ["a".repeat(gitHubUsernameLimits.maximumLength + 1), "too_long"],
  ])("rejects %j as %s", (username, code) => {
    expect(validateGitHubUsername(username)).toMatchObject({
      code,
      valid: false,
    });
  });
});
