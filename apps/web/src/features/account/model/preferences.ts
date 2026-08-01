import type {
  Preferences,
  ReducedMotionPreference,
  ThemePreference,
} from "../../../shared/api/generated";

function systemMatches(query: string): boolean {
  return (
    typeof globalThis.matchMedia === "function" &&
    globalThis.matchMedia(query).matches
  );
}

export function applyPreferences(
  preferences: Pick<Preferences, "reducedMotion" | "theme">,
) {
  const theme =
    preferences.theme === "system"
      ? systemMatches("(prefers-color-scheme: dark)")
        ? "dark"
        : "light"
      : preferences.theme;
  const reducedMotion =
    preferences.reducedMotion === "system"
      ? systemMatches("(prefers-reduced-motion: reduce)")
        ? "reduce"
        : "no-preference"
      : preferences.reducedMotion;

  document.documentElement.dataset.theme = theme;
  document.documentElement.dataset.reducedMotion = reducedMotion;
}

export const preferenceOptions = Object.freeze({
  reducedMotion: [
    { label: "Use system setting", value: "system" },
    { label: "Reduce motion", value: "reduce" },
    { label: "Allow motion", value: "no-preference" },
  ] satisfies Array<{ label: string; value: ReducedMotionPreference }>,
  resultsPerPage: [10, 20, 50] as const,
  theme: [
    { label: "Use system setting", value: "system" },
    { label: "Light", value: "light" },
    { label: "Dark", value: "dark" },
  ] satisfies Array<{ label: string; value: ThemePreference }>,
});
