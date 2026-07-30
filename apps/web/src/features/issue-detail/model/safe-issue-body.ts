const maximumBodyCharacters = 65_536;

export function normalizeIssueBody(body: string): {
  text: string;
  truncated: boolean;
} {
  const characters: string[] = [];
  for (const character of body
    .replaceAll("\r\n", "\n")
    .replaceAll("\r", "\n")) {
    const codePoint = character.codePointAt(0) ?? 0;
    const unsafeControl =
      (codePoint < 32 && codePoint !== 9 && codePoint !== 10) ||
      codePoint === 127;
    characters.push(unsafeControl ? "�" : character);
  }

  if (characters.length <= maximumBodyCharacters) {
    return {
      text: characters.join("") || "No public issue description was provided.",
      truncated: false,
    };
  }
  return {
    text: `${characters.slice(0, maximumBodyCharacters).join("")}\n\n[Content truncated by IssueScout]`,
    truncated: true,
  };
}
