export const queryKeys = Object.freeze({
  issues: Object.freeze({
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
});
