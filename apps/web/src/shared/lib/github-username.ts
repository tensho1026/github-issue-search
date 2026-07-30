export const gitHubUsernameLimits = Object.freeze({
  maximumLength: 39,
  minimumLength: 1,
});

export const gitHubUsernamePattern =
  /^(?:[A-Za-z0-9](?:[A-Za-z0-9]|-(?!-)){0,37}[A-Za-z0-9]|[A-Za-z0-9])$/;

export type GitHubUsernameValidationCode = "empty" | "invalid" | "too_long";

export type GitHubUsernameValidationResult =
  | {
      valid: true;
      username: string;
    }
  | {
      code: GitHubUsernameValidationCode;
      message: string;
      valid: false;
    };

export function validateGitHubUsername(
  rawUsername: string,
): GitHubUsernameValidationResult {
  const username = rawUsername.trim();
  if (username.length < gitHubUsernameLimits.minimumLength) {
    return {
      code: "empty",
      message: "Enter a GitHub username to continue.",
      valid: false,
    };
  }
  if (username.length > gitHubUsernameLimits.maximumLength) {
    return {
      code: "too_long",
      message: `GitHub usernames contain at most ${gitHubUsernameLimits.maximumLength} characters.`,
      valid: false,
    };
  }
  if (!gitHubUsernamePattern.test(username)) {
    return {
      code: "invalid",
      message:
        "Use letters, numbers, or single hyphens. A username cannot begin or end with a hyphen.",
      valid: false,
    };
  }
  return { username, valid: true };
}
