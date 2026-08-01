import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import { stringify } from "yaml";

const repositoryRoot = path.resolve(import.meta.dirname, "../..");
const contractPath = path.join(
  repositoryRoot,
  "packages/contracts/openapi.yaml",
);
const fixtureDirectory = path.join(
  repositoryRoot,
  "packages/contracts/fixtures",
);
const startMarker = "  # BEGIN GENERATED FIXTURE EXAMPLES";
const endMarker = "  # END GENERATED FIXTURE EXAMPLES";
const definitions = [
  ["HealthSuccess", "Process readiness", "health.success.json"],
  [
    "DatabaseHealthSuccess",
    "Optional account storage is ready",
    "database-health.success.json",
  ],
  [
    "AuthSessionAnonymous",
    "Authentication is configured without a current session",
    "auth-session.anonymous.json",
  ],
  [
    "AuthSessionAuthenticated",
    "Authenticated browser session with an in-memory CSRF token",
    "auth-session.authenticated.json",
  ],
  ["AuthLogoutSuccess", "Revoked browser session", "auth-logout.success.json"],
  [
    "AccountBookmarksSuccess",
    "Deterministic account-owned bookmark page",
    "account-bookmarks.success.json",
  ],
  [
    "AccountSavedSearchesSuccess",
    "Deterministic account-owned saved-search page",
    "account-saved-searches.success.json",
  ],
  [
    "AccountPreferencesSuccess",
    "Persisted account display preferences",
    "account-preferences.success.json",
  ],
  [
    "AccountExportSuccess",
    "Bounded privacy export",
    "account-export.success.json",
  ],
  [
    "AccountDeleteSuccess",
    "Irreversible account deletion confirmation",
    "account-delete.success.json",
  ],
  [
    "GitHubUserSuccess",
    "Normalized public GitHub profile",
    "github-user.success.json",
  ],
  [
    "ProfileAnalysisSuccess",
    "Bounded public profile evidence",
    "profile-analysis.success.json",
  ],
  [
    "IssueSearchEmpty",
    "Valid search with no eligible issues",
    "issue-search.empty.json",
  ],
  [
    "RepositoryDiscoveryPartial",
    "Successful bounded search with explicit upstream partial-data warnings",
    "repository-discovery.success.json",
  ],
  [
    "IssueDetailSuccess",
    "Analyzed issue with explainable recommendation evidence",
    "issue-detail.success.json",
  ],
  [
    "GitHubRateLimitError",
    "GitHub rejected the bounded upstream request at its rate limit",
    "rate-limit.error.json",
  ],
  [
    "InvalidRequestError",
    "A client value failed bounded validation",
    "invalid-request.error.json",
  ],
];

if (
  process.argv.length > 3 ||
  (process.argv[2] && process.argv[2] !== "--check")
) {
  console.error("usage: sync-openapi-examples.mjs [--check]");
  process.exit(64);
}

const examples = {};
for (const [name, summary, file] of definitions) {
  examples[name] = {
    summary,
    value: JSON.parse(
      await readFile(path.join(fixtureDirectory, file), "utf8"),
    ),
  };
}

const generatedYAML = stringify(
  { examples },
  {
    indent: 2,
    lineWidth: 0,
  },
)
  .trimEnd()
  .split("\n")
  .map((line) => `  ${line}`)
  .join("\n");
const generatedBlock = `${startMarker}\n${generatedYAML}\n${endMarker}\n`;
const source = await readFile(contractPath, "utf8");
const markerPattern =
  /  # BEGIN GENERATED FIXTURE EXAMPLES\n[\s\S]*?  # END GENERATED FIXTURE EXAMPLES\n/;

let synchronized;
if (markerPattern.test(source)) {
  synchronized = source.replace(markerPattern, generatedBlock);
} else {
  synchronized = source.replace(
    /^components:\n/m,
    `components:\n${generatedBlock}`,
  );
}

if (process.argv[2] === "--check") {
  if (synchronized !== source) {
    console.error(
      "OpenAPI fixture examples are stale; run: pnpm run contracts:examples",
    );
    process.exit(1);
  }
  console.log(`${definitions.length} generated OpenAPI examples are current.`);
} else {
  await writeFile(contractPath, synchronized);
  console.log(
    `Synchronized ${definitions.length} validated fixtures into OpenAPI examples.`,
  );
}
