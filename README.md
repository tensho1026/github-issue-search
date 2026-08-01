# IssueScout

[![CI](https://github.com/tensho1026/github-issue-search/actions/workflows/ci.yml/badge.svg)](https://github.com/tensho1026/github-issue-search/actions/workflows/ci.yml)
[![Security](https://github.com/tensho1026/github-issue-search/actions/workflows/security.yml/badge.svg)](https://github.com/tensho1026/github-issue-search/actions/workflows/security.yml)

IssueScout helps developers find open-source issues they can realistically
complete. It compares a GitHub user's public technology profile with issue
requirements, estimated effort, and repository health.

The anonymous core is intentionally stateless: the API reads public GitHub data
on demand and may cache it in memory, but it does not require a database or
GitHub OAuth. Optional account features use GitHub OAuth with PKCE, rotating
server sessions, and a separate TLS-only PostgreSQL boundary. Database or OAuth
failure never disables anonymous profile, repository, or issue discovery.

## Quick start

The deterministic stack needs no credential:

```sh
corepack enable
corepack prepare --activate
make install
make dev-smoke
```

For interactive development, create ignored environment files and start both
native processes:

```sh
cp apps/api/.env.example apps/api/.env
cp apps/web/.env.example apps/web/.env
make dev
```

- Web: `http://127.0.0.1:5173`
- API process health: `http://127.0.0.1:8080/api/health`
- Optional database readiness: `http://127.0.0.1:8080/api/health/database`

Press Ctrl-C once; the supervisor gracefully stops both process trees.

## Repository layout

```text
.
├── apps/
│   ├── api/              # Go + Gin HTTP API
│   └── web/              # React + TypeScript web application
├── docs/                 # Architecture and engineering decisions
├── http/                 # HTTPYAC requests grouped by capability
├── packages/             # OpenAPI contract, fixtures, and generated types
├── scripts/              # Reusable CI, database, release, and dev automation
├── go.work               # Go workspace
├── package.json          # Cross-application commands
└── pnpm-workspace.yaml   # JavaScript workspace
```

Start with the [engineering handbook](docs/README.md). See
[the architecture guide](docs/architecture.md) for dependency rules and
future extension seams, [the frontend guide](docs/frontend.md) for the
accessible profile journey and state ownership, [the test strategy](docs/testing.md)
for deterministic validation, [the CI guide](docs/ci.md) for quality gates,
[secure delivery](docs/delivery.md) for Docker-free releases, and
[security engineering](docs/security.md) for trust boundaries and incident
response. The [account workspace guide](docs/account-workspace.md) defines
bookmarks, saved searches, preferences, privacy export, and deletion. The
[issue recommendation guide](docs/issue-recommendations.md)
documents the score, sampling, warnings, cache, and deterministic ranking.

## Prerequisites

- Node.js 22.22 or newer (Node 24 LTS is recommended)
- pnpm 10
- Go 1.25
- A server-only GitHub personal access token for live GraphQL-powered routes
- PostgreSQL 14+ only when manually verifying optional account persistence
- GNU tar only when building reproducible release archives on macOS

The repository pins the pnpm release in `package.json`. Corepack can provision
that version:

```sh
corepack enable
corepack prepare --activate
```

## Environment variables

Examples contain no credential. Store local server secrets only in the ignored
`apps/api/.env`; never put them in a `VITE_*` variable.

| Variable                     | Required  | Purpose                                                             |
| ---------------------------- | --------- | ------------------------------------------------------------------- |
| `GITHUB_TOKEN`               | Live use  | Server-only public GitHub REST/GraphQL access                       |
| `DATABASE_URL`               | Auth only | Rotated TLS PostgreSQL URL for authenticated account data           |
| `GITHUB_OAUTH_CLIENT_ID`     | Auth only | GitHub OAuth App public client identifier                           |
| `GITHUB_OAUTH_CLIENT_SECRET` | Auth only | Server-only OAuth App secret                                        |
| `GITHUB_OAUTH_CALLBACK_URL`  | Auth only | Exact API callback registered in GitHub                             |
| `AUTH_FRONTEND_URL`          | Auth only | Fixed frontend origin used after a validated callback               |
| `AUTH_FLOW_ENCRYPTION_KEY`   | Auth only | Dedicated 32-byte hex key for the encrypted short-lived flow cookie |
| `AUTH_COOKIE_SECURE`         | No        | Must be `true` outside loopback development/test                    |
| `TRUSTED_PROXY_CIDRS`        | No        | Explicit reverse-proxy CIDRs; empty trusts no proxy                 |
| `ALLOWED_ORIGINS`            | No        | Exact credentialed browser origins; defaults to local Vite          |
| `PORT`                       | No        | API listener; defaults to `8080`                                    |
| `VITE_API_BASE_URL`          | No        | Browser-visible API origin; defaults to local API                   |

`DATABASE_URL` must use `sslmode=require` or `sslmode=verify-full`. A
connection string exposed in chat, an issue, a screenshot, or terminal output
must be rotated before use. The complete bounded configuration surface is in
[the configuration reference](docs/configuration.md).

### Optional local GitHub sign-in

Anonymous development needs no OAuth App. To exercise login, bookmarks, saved
searches, preferences, export, and deletion, first create a GitHub OAuth App
with:

- Homepage URL: `http://127.0.0.1:5173`
- Authorization callback URL:
  `http://127.0.0.1:8080/api/auth/github/callback`

Then set `DATABASE_URL` and all five required authentication values together in
the ignored `apps/api/.env`:

```dotenv
GITHUB_OAUTH_CLIENT_ID=replace-with-oauth-app-client-id
GITHUB_OAUTH_CLIENT_SECRET=replace-with-oauth-app-secret
GITHUB_OAUTH_CALLBACK_URL=http://127.0.0.1:8080/api/auth/github/callback
AUTH_FRONTEND_URL=http://127.0.0.1:5173
AUTH_FLOW_ENCRYPTION_KEY=replace-with-output-from-openssl-rand
AUTH_COOKIE_SECURE=false
```

Generate the last value:

```sh
openssl rand -hex 32
```

Apply migrations and start the stack:

```sh
pnpm run database:migrate
make dev
```

The API requests only `read:user`, retains only the minimum public GitHub
identity, and discards GitHub's access token after `GET /user`. A complete
authentication setup makes
`http://127.0.0.1:8080/api/auth/github/start?returnTo=/` the login entry point.
If the five required values are all absent, authentication stays disabled and
`GET /api/auth/session` returns an anonymous `configured: false` response
without querying PostgreSQL. See the
[authentication guide](docs/authentication.md) for callback, Cookie, PKCE,
CSRF, rotation, proxy, and production HTTPS details.

After signing in, use the tracked
[`http/account-workspace.http`](http/account-workspace.http) requests with
fresh credentials stored only in a private HTTPYAC environment. The deletion
request contains a deliberately invalid confirmation by default.
The web application exposes the same optional features at
`http://127.0.0.1:5173/workspace`. Public result cards explain account-only
save actions before sign-in and preserve the current validated route through
OAuth; they never force a login or store credentials in browser storage.

## Essential commands

| Command                     | Purpose                                               |
| --------------------------- | ----------------------------------------------------- |
| `make dev`                  | Start API and web with readiness and safe cleanup     |
| `make dev-smoke`            | Prove the deterministic credential-free stack         |
| `make check`                | Format, lint, typecheck, unit test, and build         |
| `pnpm run quality:strict`   | Run the complete local pre-PR quality policy          |
| `pnpm run e2e`              | Test production web against the compiled API          |
| `pnpm run database:status`  | Inspect safe migration state and checksums            |
| `pnpm run database:migrate` | Apply pending forward-only migrations                 |
| `pnpm run database:verify`  | Require a complete, checksum-matching database schema |
| `pnpm run migrations:check` | Enforce SQL safety and append-only migration history  |

Database commands load `apps/api/.env` without printing it. Leave
`DATABASE_URL` empty for anonymous-only development. See
[the database guide](docs/database.md) before enabling it.

The API can search a bounded GitHub issue candidate window after startup:

```sh
curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{"username":"octocat","languages":["Go"],"maximumEffort":"half_day"}' \
  'http://127.0.0.1:8080/api/issues/search?page=1&perPage=20'
```

Omitted filters use the MVP defaults: at least 10 stars, updated within 180
days, maximum preliminary difficulty 3, non-archived repositories, English
allowed, and the `good first issue` or `help wanted` labels. See the
[versioned OpenAPI contract](packages/contracts/openapi.yaml) for all request,
response, pagination, exclusion, cache-header, and error details. Issue search
uses GitHub's authenticated GraphQL API, so the API process requires a
server-only `GITHUB_TOKEN` for this route. Browser callers remain anonymous and
the token is never returned to them. The optional `maximumEffort` filter is
applied to the ranked analysis before server pagination.

Inspect the same recommendation with complete evidence and bounded activity
samples:

```sh
curl --fail-with-body \
  'http://127.0.0.1:8080/api/issues/golang/go/1?skills=Go'
```

## Quality commands

```sh
make format-check
make lint
make typecheck
make test
make build
```

`make check` runs the complete local gate in the same order expected by CI.
Generated output is ignored and can be removed with `make clean`.

The strict pull request pipelines additionally enforce coverage, fuzz smoke
runs, Go performance and web bundle budgets, OpenAPI fixture and route drift,
architecture, workflow security, dependency and secret scans, contribution
metadata, and built-stack E2E checks. See
[the CI guide](docs/ci.md) for local equivalents, quality budgets, artifact
retention, and branch protection.

Performance-sensitive code is developed against explicit limits: at most 20
repositories per profile, 50 search candidates, 20 detailed candidates, three
manifest reads per repository, and five concurrent GitHub requests by default.
See the issue recommendation guide for the staged analysis and caching model.

## Current status

The repository is being delivered through the scoped GitHub issues in the
[MVP implementation backlog](https://github.com/tensho1026/github-issue-search/issues).
Features, recommendation logic, and production UI continue to land through
their own issue-linked branches. CI, security, and Docker-free release
automation are maintained as executable repository policy.
