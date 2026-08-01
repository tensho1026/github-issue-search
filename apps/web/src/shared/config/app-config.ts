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
  repositories: "/repositories",
  search: "/search",
  workspace: "/workspace",
});

export const externalLinks = Object.freeze({
  gitHubIssue(owner: string, repository: string, issueNumber: number): string {
    return `https://github.com/${encodeURIComponent(
      owner,
    )}/${encodeURIComponent(repository)}/issues/${issueNumber}`;
  },
  gitHubRepository(owner: string, repository: string): string {
    return `https://github.com/${encodeURIComponent(
      owner,
    )}/${encodeURIComponent(repository)}`;
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
  detail(
    owner: string,
    repository: string,
    issueNumber: number,
    skills: readonly string[],
  ): `/${string}` {
    const query = new URLSearchParams();
    for (const skill of skills) {
      query.append("skills", skill);
    }
    const suffix = query.size > 0 ? `?${query.toString()}` : "";
    return `/api/issues/${encodeURIComponent(owner)}/${encodeURIComponent(
      repository,
    )}/${issueNumber}${suffix}`;
  },
  search(page: number, perPage: number): `/${string}` {
    const query = new URLSearchParams({
      page: page.toString(),
      perPage: perPage.toString(),
    });
    return `/api/issues/search?${query.toString()}`;
  },
});

export const repositoryEndpoints = Object.freeze({
  search(page: number, perPage: number): `/${string}` {
    const query = new URLSearchParams({
      page: page.toString(),
      perPage: perPage.toString(),
    });
    return `/api/repositories/search?${query.toString()}`;
  },
});

export const authEndpoints = Object.freeze({
  logout: "/api/auth/logout" as const,
  refresh: "/api/auth/session/refresh" as const,
  session: "/api/auth/session" as const,
  start(returnTo: string): string {
    const query = new URLSearchParams({ returnTo });
    return `${appConfig.apiBaseUrl}/api/auth/github/start?${query.toString()}`;
  },
});

export const accountEndpoints = Object.freeze({
  account: "/api/account" as const,
  bookmark(id: string, version: number): `/${string}` {
    return `/api/account/bookmarks/${encodeURIComponent(
      id,
    )}?version=${version}`;
  },
  bookmarks(page = 1, perPage = 50): `/${string}` {
    return `/api/account/bookmarks?page=${page}&perPage=${perPage}`;
  },
  export: "/api/account/export" as const,
  preferences: "/api/account/preferences" as const,
  savedSearch(id: string): `/${string}` {
    return `/api/account/saved-searches/${encodeURIComponent(id)}`;
  },
  savedSearchForDelete(id: string, version: number): `/${string}` {
    return `/api/account/saved-searches/${encodeURIComponent(
      id,
    )}?version=${version}`;
  },
  savedSearches(page = 1, perPage = 50): `/${string}` {
    return `/api/account/saved-searches?page=${page}&perPage=${perPage}`;
  },
});
