export const queryKeys = Object.freeze({
  account: Object.freeze({
    bookmarks: ["account", "bookmarks"] as const,
    preferences: ["account", "preferences"] as const,
    root: ["account"] as const,
    savedSearches: ["account", "saved-searches"] as const,
  }),
  auth: Object.freeze({
    session: ["auth", "session"] as const,
  }),
  issues: Object.freeze({
    detail(
      owner: string,
      repository: string,
      issueNumber: number,
      skills: readonly string[],
    ) {
      return [
        "issues",
        "detail",
        owner.toLowerCase(),
        repository.toLowerCase(),
        issueNumber,
        [...skills],
      ] as const;
    },
    root: ["issues"] as const,
    search(canonicalSearch: string) {
      return ["issues", "search", canonicalSearch] as const;
    },
  }),
  profile: Object.freeze({
    analysis(username: string) {
      return ["profile", username.toLowerCase(), "analysis"] as const;
    },
    root: ["profile"] as const,
    user(username: string) {
      return ["profile", username.toLowerCase(), "user"] as const;
    },
  }),
  repositories: Object.freeze({
    root: ["repositories"] as const,
    search(canonicalSearch: string) {
      return ["repositories", "search", canonicalSearch] as const;
    },
  }),
});
