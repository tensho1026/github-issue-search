export const queryKeys = Object.freeze({
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
