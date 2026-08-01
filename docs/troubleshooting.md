# Troubleshooting

Start with the smallest reproducible command and keep the request ID or CI job
URL. Do not work around a failed safety gate by lowering a threshold.

## Local startup

### A port is already in use

`make dev` uses API port 8080 and web port 5173 by default. Find the owner,
stop only the known stale development process, or choose `PORT` and `WEB_PORT`.
Keep `VITE_API_BASE_URL` and `ALLOWED_ORIGINS` aligned.

### The stack never becomes ready

Run `make dev-api` and `make dev-web` separately to identify the child. Check:

- configuration error logs from the API;
- Node, pnpm, and Go prerequisite versions;
- `STACK_STARTUP_TIMEOUT_MS` if the first Go compile is unusually slow;
- a valid absolute `VITE_API_BASE_URL`;
- a matching CORS origin.

`pnpm run test:dev` verifies cleanup during startup and after readiness.

### Search reports an upstream error

Confirm `GITHUB_TOKEN` exists only in `apps/api/.env`, has read access to public
repository metadata, and has not expired. Profile REST behavior may work while
GraphQL search fails without a token. Use the response request ID to locate the
safe server log.

### CORS returns 403

Add the exact browser scheme, host, and port to `ALLOWED_ORIGINS`. Wildcards
and path components are intentionally invalid. Restart the API after changing
configuration.

### Authenticated storage is unavailable

Confirm the process still returns `200` from `/api/health`, then inspect
`/api/health/database`. `DATABASE_UNAVAILABLE` means the optional account
boundary is not configured or its bounded ping failed; anonymous features
remain healthy. Check only that `DATABASE_URL` contains a rotated TLS
credential, then run `pnpm run database:status`. Never print the value or paste
driver output containing connection parameters.

### GitHub sign-in reports `AUTH_UNAVAILABLE`

First call `GET /api/auth/session`. `configured: false` means the five required
OAuth values are all absent; this is healthy anonymous-only mode. If enabling
login, set the complete group, ensure `DATABASE_URL` is present, apply
migrations, and restart. Do not paste values into a report.

When `configured: true`, verify `/api/health/database` independently. A
temporary GitHub token-exchange or identity failure may return 502 while a
database operation returns 503. In both cases public routes remain usable.

### GitHub sign-in reports `INVALID_AUTH_STATE`

Restart from `/api/auth/github/start`. The flow may be expired, consumed,
modified, paired with a different browser Cookie jar, or returned with a state
that does not match the encrypted cookie. Do not retry a callback URL or
one-time code. Confirm the OAuth App callback exactly matches
`GITHUB_OAUTH_CALLBACK_URL`, including scheme, host, port, and path.

### Refresh or logout reports a CSRF or session error

Call `GET /api/auth/session` with browser credentials. If authenticated, use
the returned current token only in the `X-CSRF-Token` header and keep it in
memory. `CSRF_REJECTED` means the header, HttpOnly cookie, or server digest did
not match. `AUTHENTICATION_REQUIRED` means the session is missing, expired,
revoked, or rotated; bootstrap or sign in again.

### Login works directly but not through a reverse proxy

Leave `TRUSTED_PROXY_CIDRS` empty unless the API is actually behind an ingress.
If it is, list only the ingress's canonical CIDRs. Keep the registered callback
and `AUTH_FRONTEND_URL` as public HTTPS URLs, keep
`AUTH_COOKIE_SECURE=true`, and put the exact browser origin in
`ALLOWED_ORIGINS`. Never solve a proxy mismatch by accepting wildcard origins
or insecure production cookies.

### Migration verification reports drift

Stop deployment. Do not edit the migration table or rewrite an existing SQL
file. Compare the reported version with the reviewed catalog, restore the
original file if it changed, and create a new forward migration for the desired
schema change. Run `pnpm run migrations:check` and
`pnpm run database:verify` before retrying.

## Contract and generated code

### Generated frontend types differ

An OpenAPI change must regenerate and commit
`apps/web/src/shared/api/generated`. Run `pnpm run contracts:check` after
committing or staging the expected generated change. Never hand-edit generated
files.

### A fixture fails strict-envelope validation

Compare the fixture with its schema in
`packages/contracts/fixtures/manifest.json`. Unknown properties are expected to
fail. Update the OpenAPI contract first when a field is intentional.

### Route drift fails

Gin and OpenAPI disagree on method or path parameter spelling. Update both in
the same feature commit and add a handler/router test.

## Quality gates

### Go coverage or race check fails

Run `pnpm run coverage:api`, then the named package test with `-race`. Fix the
race or add behavioral tests; do not exclude a package or weaken the threshold.

### Fuzz smoke fails

Re-run the printed fuzz target with its seed. Preserve the generated regression
case as a deterministic test if it represents a valid failure.

### Performance budget fails

Run `pnpm run performance:api` three times on an otherwise idle machine.
Inspect algorithmic bounds and allocations. Budget changes require measured
evidence and explicit review.

### Frontend bundle budget fails

Run `pnpm run build:web` and `pnpm run bundle:check`. Prefer route-level loading,
reuse established UI primitives, and avoid importing broad libraries into the
shared initial chunk.

### A required GitHub check is pending

Inspect path-aware jobs first. `CI required` and `Security required` finish only
after applicable dependencies succeed or intentionally skip. A cancelled or
failed dependency makes the aggregate fail.

## Release artifacts

### GNU tar is missing on macOS

Install GNU tar and ensure `gtar` is on `PATH`. Deterministic archives must not
fall back to BSD tar because normalization flags differ.

### Independent builds differ

Run `pnpm run release:reproducibility v0.0.0-debug`. Differences commonly
mean an uncontrolled timestamp, build ID, file order, mode, owner, source map,
or environment-dependent frontend value entered the archive.

### Secret-surface scan fails

Treat it as a credential incident until disproved. Do not rename or encode the
value to bypass detection. Remove the source, rotate exposed credentials, and
rebuild from a clean revision.

## Escalation checklist

Include the failing command, revision, platform, safe request ID, expected and
actual behavior, and the smallest relevant log excerpt. Exclude `.env`
contents, GitHub tokens, database URLs, OAuth codes, private GitHub data, and
full upstream responses.
