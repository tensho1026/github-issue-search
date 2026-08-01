# Configuration reference

IssueScout follows environment-only runtime configuration. Example files
contain names and safe defaults, never credentials. Commit no `.env` file.

## API environment

| Variable                                | Default                  | Bounds and purpose                                                    | Secret |
| --------------------------------------- | ------------------------ | --------------------------------------------------------------------- | ------ |
| `APP_ENV`                               | `development`            | One of `development`, `test`, `staging`, `production`                 | No     |
| `PORT`                                  | `8080`                   | TCP port 1–65535 for the API listener                                 | No     |
| `ALLOWED_ORIGINS`                       | `http://127.0.0.1:5173`  | Comma-separated absolute HTTP(S) browser origins; no wildcard         | No     |
| `GITHUB_API_BASE_URL`                   | `https://api.github.com` | Absolute HTTPS upstream URL; HTTP is accepted only for loopback tests | No     |
| `GITHUB_TOKEN`                          | empty                    | Server-only GitHub credential; required by GraphQL search             | Yes    |
| `GITHUB_REQUEST_TIMEOUT`                | `10s`                    | Positive duration, at most one minute                                 | No     |
| `GITHUB_API_MAX_CONCURRENCY`            | `5`                      | Bounded upstream fan-out, 1–20                                        | No     |
| `PROFILE_ANALYSIS_REPOSITORY_LIMIT`     | `20`                     | Repositories analyzed per profile, 1–20                               | No     |
| `PROFILE_ANALYSIS_CACHE_TTL`            | `30m`                    | Profile-analysis cache lifetime, positive and at most 24 hours        | No     |
| `PROFILE_ANALYSIS_CACHE_CAPACITY`       | `500`                    | Profile-analysis LRU entries, 1–10,000                                | No     |
| `REPOSITORY_DISCOVERY_RESULT_LIMIT`     | `50`                     | Upstream repository candidate window, 1–50                            | No     |
| `REPOSITORY_DISCOVERY_ENRICHMENT_LIMIT` | `10`                     | Batched enrichments, 1–20 and no more than result limit               | No     |
| `REPOSITORY_DISCOVERY_CACHE_TTL`        | `5m`                     | Repository cache lifetime, positive and at most 24 hours              | No     |
| `REPOSITORY_DISCOVERY_CACHE_CAPACITY`   | `1000`                   | Repository-discovery LRU entries, 1–10,000                            | No     |
| `ISSUE_SEARCH_RESULT_LIMIT`             | `50`                     | Upstream candidate window, 1–50                                       | No     |
| `ISSUE_SEARCH_CACHE_TTL`                | `5m`                     | Canonical candidate cache lifetime, positive and at most 24 hours     | No     |
| `ISSUE_SEARCH_CACHE_CAPACITY`           | `1000`                   | Candidate LRU entries, 1–10,000                                       | No     |
| `ISSUE_DETAIL_ANALYSIS_LIMIT`           | `20`                     | Enriched candidates, at least 1 and no more than search limit         | No     |
| `ISSUE_DETAIL_CACHE_TTL`                | `5m`                     | Detail snapshot cache lifetime, positive and at most 24 hours         | No     |
| `ISSUE_DETAIL_CACHE_CAPACITY`           | `500`                    | Detail LRU entries, 1–10,000                                          | No     |
| `MANIFEST_FILE_LIMIT`                   | `3`                      | Supported manifests read per repository, 1–10                         | No     |
| `DATABASE_URL`                          | empty                    | Optional TLS-only PostgreSQL URL for authenticated account data       | Yes    |
| `DATABASE_MAX_CONNECTIONS`              | `10`                     | Hard pool ceiling, 1–100                                              | No     |
| `DATABASE_MIN_CONNECTIONS`              | `0`                      | Warm pool floor, 0–maximum                                            | No     |
| `DATABASE_CONNECT_TIMEOUT`              | `5s`                     | Positive duration, at most one minute                                 | No     |
| `DATABASE_QUERY_TIMEOUT`                | `5s`                     | Query/statement/idle-transaction deadline, at most one minute         | No     |
| `DATABASE_MAX_CONNECTION_LIFETIME`      | `30m`                    | Positive duration, at most 24 hours                                   | No     |
| `DATABASE_MAX_CONNECTION_IDLE_TIME`     | `5m`                     | Positive duration, at most 24 hours                                   | No     |
| `DATABASE_HEALTH_CHECK_PERIOD`          | `30s`                    | Pool health cycle, positive and at most one hour                      | No     |
| `GITHUB_OAUTH_CLIENT_ID`                | empty                    | Required OAuth App ID when authentication is enabled; at most 255     | No     |
| `GITHUB_OAUTH_CLIENT_SECRET`            | empty                    | Required server-only OAuth App secret; at least 20 characters         | Yes    |
| `GITHUB_OAUTH_AUTHORIZE_URL`            | official GitHub URL      | Absolute HTTPS authorization endpoint; loopback allowed in tests      | No     |
| `GITHUB_OAUTH_TOKEN_URL`                | official GitHub URL      | Absolute HTTPS token endpoint; loopback allowed in tests              | No     |
| `GITHUB_OAUTH_CALLBACK_URL`             | empty                    | Exact absolute `/api/auth/github/callback` URL                        | No     |
| `AUTH_FRONTEND_URL`                     | empty                    | One fixed origin also present in `ALLOWED_ORIGINS`                    | No     |
| `AUTH_FLOW_ENCRYPTION_KEY`              | empty                    | Exactly 64 lowercase hex characters for AES-256-GCM                   | Yes    |
| `AUTH_STATE_TTL`                        | `10m`                    | Positive OAuth state/cookie lifetime, at most 15 minutes              | No     |
| `AUTH_SESSION_TTL`                      | `12h`                    | Positive server-session lifetime, at most seven days                  | No     |
| `AUTH_MAX_SESSIONS`                     | `10`                     | Active sessions retained per account, 1–50                            | No     |
| `AUTH_COOKIE_SECURE`                    | environment-dependent    | Defaults false in dev/test and true in staging/production             | No     |
| `TRUSTED_PROXY_CIDRS`                   | empty                    | Canonical comma-separated ingress CIDRs; empty trusts no proxy        | No     |
| `USE_GITHUB_API_MOCK`                   | `false`                  | Deterministic adapter; `true` is legal only with `APP_ENV=test`       | No     |

`apps/api/.env.example` is the copyable API reference. The process validates
all values before opening a listener. Configuration errors fail closed and log
only the variable name and validation reason.

## Authentication configuration group

Authentication is enabled only when all of these values are non-empty:

- `GITHUB_OAUTH_CLIENT_ID`;
- `GITHUB_OAUTH_CLIENT_SECRET`;
- `GITHUB_OAUTH_CALLBACK_URL`;
- `AUTH_FRONTEND_URL`;
- `AUTH_FLOW_ENCRYPTION_KEY`.

The group also requires `DATABASE_URL`. All values absent means
authentication is intentionally disabled; a partial group fails startup.
`GITHUB_OAUTH_CALLBACK_URL` must have exactly the
`/api/auth/github/callback` path and no query. `AUTH_FRONTEND_URL` must be one
origin already listed in `ALLOWED_ORIGINS`.

Development/test may use loopback HTTP only when `AUTH_COOKIE_SECURE=false`.
Staging and production require HTTPS and secure cookies. The authorization and
token endpoint overrides exist for GitHub Enterprise and deterministic
adapter tests; ordinary GitHub.com deployments keep their defaults.

Generate `AUTH_FLOW_ENCRYPTION_KEY` independently with:

```sh
openssl rand -hex 32
```

Never reuse the OAuth client secret, database password, runtime GitHub token,
or another encryption key. See [Optional GitHub authentication](authentication.md)
for registration, flow, Cookie, CSRF, proxy, and rotation behavior.

## Web and native supervisor environment

| Variable                   | Default                 | Purpose                                                  |
| -------------------------- | ----------------------- | -------------------------------------------------------- |
| `VITE_API_BASE_URL`        | `http://127.0.0.1:8080` | Public browser-visible IssueScout API origin             |
| `WEB_PORT`                 | `5173`                  | Native supervisor Vite port                              |
| `STACK_STARTUP_TIMEOUT_MS` | `60000`                 | Positive readiness deadline, including a cold Go compile |

`apps/web/.env.example` contains only browser-public values. Never place
`GITHUB_TOKEN`, a database URL, OAuth secret, or provider credential in a
`VITE_*` variable: Vite embeds those values into the static bundle.

`ALLOWED_ORIGINS`, `PORT`, and `APP_ENV` can also be set in the shell that runs
the native supervisor. Shell values take precedence over defaults.

## Fixed server deadlines

| Deadline                      |           Value | Reason                                     |
| ----------------------------- | --------------: | ------------------------------------------ |
| Read header                   |       5 seconds | Limits slow header delivery                |
| Request read/write            | 20 seconds each | Bounds client connections                  |
| Idle connection               |      60 seconds | Reuses healthy keep-alive connections      |
| Graceful shutdown             |      10 seconds | Allows in-flight requests to complete      |
| Health/profile-normal request |       5 seconds | Fast boundary work                         |
| Profile analysis              |      15 seconds | Bounded GitHub repository fan-out          |
| Repository discovery          |      15 seconds | One REST search and one GraphQL enrichment |
| Issue search                  |      15 seconds | Bounded discovery and enrichment           |
| Issue detail                  |      15 seconds | One bounded GraphQL snapshot and analysis  |

These are compiled process safety limits rather than deployment knobs. Change
them with focused server tests instead of adding unvalidated environment
surface.

## Cache and performance behavior

The four implemented caches are in-memory, capacity-bounded TTL/LRU adapters:

| Cache                | Default TTL | Default capacity | Canonical key                                      |
| -------------------- | ----------: | ---------------: | -------------------------------------------------- |
| Profile analysis     |  30 minutes |              500 | Validated lowercase username                       |
| Repository discovery |   5 minutes |             1000 | Normalized repository filters; excludes pagination |
| Issue search         |   5 minutes |             1000 | Normalized issue filters; excludes page and effort |
| Issue detail         |   5 minutes |              500 | Validated owner/repository/issue number            |

Equal misses are coalesced. Values are deep-copied at cache boundaries.
Restarting the API clears all anonymous data. See
[Architecture](architecture.md) for the staged request limits and
[Testing](testing.md) for latency/allocation budgets.

## Environment-specific safety

- Development may use a real server-only GitHub token.
- Test may opt into the deterministic adapter.
- Staging and production must reject the deterministic adapter.
- Anonymous routes never require or access database configuration.
- Database and OAuth secrets belong only to the API process and
  protected delivery environment, not examples, logs, frontend assets, or
  release manifests.

See [Authenticated PostgreSQL persistence](database.md) for TLS validation,
pool bounds, credential rotation, migrations, and least-privilege roles.
