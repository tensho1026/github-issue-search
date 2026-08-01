# Interactive API reference

IssueScout ships its complete OpenAPI contract and Swagger UI inside the API
binary. A release archive needs no CDN, Node.js process, copied static
directory, or network access beyond the IssueScout API calls a reader chooses
to send.

## Open and download

Start the API through the normal native stack, then open:

- interactive Swagger UI: `http://127.0.0.1:8080/docs/`;
- machine-readable OpenAPI YAML: `http://127.0.0.1:8080/openapi.yaml`.

`/docs` permanently redirects to the canonical trailing-slash URL. Download
the exact contract served by the process with:

```sh
curl --fail-with-body \
  --output issuescout-openapi.yaml \
  http://127.0.0.1:8080/openapi.yaml
```

The YAML response has an `ETag`, an explicit
`application/yaml; charset=utf-8` media type, and a short revalidation cache
policy. Conditional requests can use `If-None-Match`. The index, IssueScout
bootstrap script, stylesheet, and allowlisted Swagger assets also have
explicit media and cache policies.

Set `API_DOCUMENTATION_ENABLED=false` to omit all three documentation route
shapes (`/docs`, `/docs/*asset`, and `/openapi.yaml`) from that API process.
The default is `true`. Disabling documentation does not alter an anonymous,
authentication, account, health, GitHub, or database dependency.

## Use the interactive operations

Swagger UI groups all 23 operations by product capability. Every operation
documents:

- its stable `operationId`, summary, and implementation-level behavior;
- accepted path, query, header, and JSON body values with bounds;
- one explicit success status and every reachable error status;
- required response `X-Request-ID` correlation;
- pagination, cache, GitHub failure, cancellation, and partial-data behavior;
- cookie and CSRF requirements for optional account operations.

The UI enables deep links, filtering, request durations, schema expansion,
request snippets, and **Try it out**. It does not persist authorization. A
try-out request receives a `docs_*` request ID unless the reader supplies one.

Anonymous profile, repository, issue, and process-health operations work
without GitHub sign-in. Live GitHub-backed operations still require the API
process to have its server-only `GITHUB_TOKEN`; the UI never receives it.

Account reads require an existing same-origin session cookie. Mutations also
require the current `X-CSRF-Token`, obtained from `GET /api/auth/session` and
entered through the Swagger authorization control. Browser policy prevents
JavaScript from constructing a Cookie header; the browser sends an existing
same-origin cookie because Swagger requests use credentials.

## Example catalog

Examples are generated from the same JSON fixtures validated by Ajv in CI.
They include:

- process and optional database readiness;
- public GitHub identity and bounded profile evidence;
- a valid issue search with no eligible result;
- repository discovery with explicit GitHub-window and enrichment warnings;
- complete issue recommendation evidence;
- anonymous and authenticated session states;
- account pages, preferences, export, logout, and deletion;
- validation failure and GitHub rate exhaustion.

The block between `BEGIN GENERATED FIXTURE EXAMPLES` and
`END GENERATED FIXTURE EXAMPLES` in
`packages/contracts/openapi.yaml` must not be hand-edited.

## Security and runtime isolation

Swagger UI 5.32.8 assets are supplied by the pinned
`github.com/swaggest/swgui` v1.8.9 Go dependency. Both projects use the
Apache-2.0 license. The handler exposes only an explicit asset allowlist and
does not enable the dependency's proxy feature or generated index.

The IssueScout index uses external same-origin script files, never inline
JavaScript. Documentation responses apply:

- `Content-Security-Policy` with `script-src 'self'`, `connect-src 'self'`,
  denied frames, objects, workers, forms, and base URL changes;
- `Cross-Origin-Opener-Policy: same-origin`;
- `Cross-Origin-Resource-Policy: same-origin`;
- denied camera, geolocation, microphone, payment, and USB permissions;
- `X-Content-Type-Options: nosniff`, frame denial, no-referrer, and no-index.

Swagger UI needs inline style attributes for its generated layout, so the CSP
allows inline styles but not inline scripts. Playwright records every
documentation request and rejects a non-loopback runtime dependency. Handler
tests reject unknown asset names and verify conditional, `HEAD`, CSP, cache,
content-type, and embedded-asset behavior.

## Update the contract

1. Edit `packages/contracts/openapi.yaml` outside the generated example block.
2. Add or update a schema-valid JSON document in
   `packages/contracts/fixtures/` and its manifest entry when an example
   changes.
3. Update `scripts/contracts/sync-openapi-examples.mjs` when the example
   catalog changes.
4. Run `pnpm run contracts:sync`.
5. Regenerate and retain frontend contract types.
6. Run `pnpm run contracts:check`.

`pnpm run contracts:examples:check` proves that embedded examples match their
fixtures. `pnpm run contracts:embed:check` proves the API binary's embedded
YAML is byte-identical to the single contract source. The complete contract
gate also runs Redocly, strict policy, fixture mutation, frontend type drift,
and Gin route drift checks.

Do not edit `apps/api/internal/documentation/openapi.yaml` or generated
TypeScript by hand. `pnpm run contracts:sync` regenerates the embedded YAML;
the web generator invoked by `pnpm run contracts:check` regenerates TypeScript.

## Release verification

The release build compiles the contract and all UI assets into every native API
binary. The packaged smoke test starts the extracted host binary and verifies:

- `/docs/` returns the IssueScout index and strict CSP;
- a Swagger stylesheet is available from the same process;
- `/openapi.yaml` returns OpenAPI 3.1 with the expected media type, cache
  policy, and `ETag`;
- the ordinary health, request-correlation, and graceful-shutdown checks still
  pass.

This proves the released documentation does not depend on a source checkout.
The release SBOM and vulnerability scan cover the pinned embedded dependency.

## Troubleshooting

If the UI shell loads but operations do not appear, fetch `/openapi.yaml`
directly and run `pnpm run contracts:lint`. A stale generated block or embedded
copy is repaired with `pnpm run contracts:sync`.

If **Try it out** returns `403`, use an origin in `ALLOWED_ORIGINS`. If an
account mutation returns `401` or `403`, bootstrap the same browser origin,
confirm the session is authenticated, and supply the current CSRF token.
GitHub `429`, upstream `502`, and deadline `504` responses are application
behavior documented by each operation, not Swagger asset failures.
