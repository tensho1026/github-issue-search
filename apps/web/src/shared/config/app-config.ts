const defaultQueryStaleTimeMs = 60_000;
const defaultQueryGarbageCollectionTimeMs = 10 * 60_000;

function normalizeApiBaseUrl(value: string | undefined): string {
  const candidate = value?.trim();
  if (!candidate) {
    return "";
  }

  const parsed = new URL(candidate);
  if (
    (parsed.protocol !== "http:" && parsed.protocol !== "https:") ||
    parsed.username ||
    parsed.password ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error("VITE_API_BASE_URL must be a credential-free HTTP(S) URL");
  }

  return parsed.href.replace(/\/+$/, "");
}

const configuredApiBaseUrl: unknown = import.meta.env["VITE_API_BASE_URL"];

export const appConfig = Object.freeze({
  apiBaseUrl: normalizeApiBaseUrl(
    typeof configuredApiBaseUrl === "string" ? configuredApiBaseUrl : undefined,
  ),
  productName: "IssueScout",
  query: Object.freeze({
    garbageCollectionTimeMs: defaultQueryGarbageCollectionTimeMs,
    staleTimeMs: defaultQueryStaleTimeMs,
  }),
});

export const appRoutes = Object.freeze({
  home: "/",
  issuePattern: "/issues/:owner/:repository/:issueNumber",
  issue(owner: string, repository: string, issueNumber: number): string {
    return `/issues/${encodeURIComponent(owner)}/${encodeURIComponent(
      repository,
    )}/${issueNumber}`;
  },
  profilePattern: "/profiles/:username",
  profile(username: string): string {
    return `/profiles/${encodeURIComponent(username)}`;
  },
  search: "/search",
});

export const externalLinks = Object.freeze({
  gitHubIssue(owner: string, repository: string, issueNumber: number): string {
    return `https://github.com/${encodeURIComponent(
      owner,
    )}/${encodeURIComponent(repository)}/issues/${issueNumber}`;
  },
  gitHubProfile(username: string): string {
    return `https://github.com/${encodeURIComponent(username)}`;
  },
});

export const profileEndpoints = Object.freeze({
  analysis(username: string): `/${string}` {
    return `/api/github/users/${encodeURIComponent(username)}/profile-analysis`;
  },
  user(username: string): `/${string}` {
    return `/api/github/users/${encodeURIComponent(username)}`;
  },
});

export const issueEndpoints = Object.freeze({
  search(page: number, perPage: number): `/${string}` {
    const query = new URLSearchParams({
      page: page.toString(),
      perPage: perPage.toString(),
    });
    return `/api/issues/search?${query.toString()}`;
  },
});
