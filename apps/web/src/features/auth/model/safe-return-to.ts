import { appRoutes } from "../../../shared/config/app-config";

const maximumReturnPathLength = 2048;

const exactProductRoutes = new Set<string>([
  appRoutes.home,
  appRoutes.repositories,
  appRoutes.search,
  appRoutes.workspace,
]);

function isProductRoute(pathname: string): boolean {
  if (exactProductRoutes.has(pathname)) {
    return true;
  }
  return (
    /^\/profiles\/[^/]+$/u.test(pathname) ||
    /^\/issues\/[^/]+\/[^/]+\/[1-9]\d*$/u.test(pathname)
  );
}

/**
 * Returns a bounded product-local path suitable for the OAuth flow cookie.
 * Absolute URLs, protocol-relative paths, fragments, backslashes, unsupported
 * application routes, and callback status markers are rejected or removed.
 */
export function safeReturnTo(
  candidate: string,
  origin = globalThis.location.origin,
): string {
  if (
    candidate.length === 0 ||
    candidate.length > maximumReturnPathLength ||
    !candidate.startsWith("/") ||
    candidate.startsWith("//") ||
    candidate.includes("\\") ||
    candidate.includes("#")
  ) {
    return appRoutes.home;
  }

  try {
    const target = new URL(candidate, origin);
    if (target.origin !== origin || !isProductRoute(target.pathname)) {
      return appRoutes.home;
    }
    target.searchParams.delete("auth");
    const query = target.searchParams.toString();
    return `${target.pathname}${query ? `?${query}` : ""}`;
  } catch {
    return appRoutes.home;
  }
}

export function currentSafeReturnTo(): string {
  return safeReturnTo(
    `${globalThis.location.pathname}${globalThis.location.search}`,
  );
}
