# Security engineering

IssueScout assumes pull request content, GitHub API responses, repository files,
release inputs, and deployment inputs are untrusted. Secrets stay on the Go
server; the browser receives only normalized API responses.

## Automated controls

| Control                    | Scope                                                                  | Failure policy                  |
| -------------------------- | ---------------------------------------------------------------------- | ------------------------------- |
| CodeQL                     | Go and JavaScript/TypeScript with extended security queries            | Any analysis failure blocks     |
| Dependency review          | New pull request dependencies and licenses                             | HIGH or CRITICAL blocks         |
| Trivy repository scan      | Go/pnpm dependencies, secrets, workflows, and configuration            | Fixed HIGH or CRITICAL blocks   |
| Release secret surface     | Expanded archives, browser assets, binaries, environment/key patterns  | Any match blocks                |
| Trivy release scan         | Packaged release directory and embedded content                        | Fixed HIGH or CRITICAL blocks   |
| zizmor                     | GitHub Actions attack surface with the pedantic persona                | Medium+ / medium-confidence     |
| actionlint and policy      | Workflow syntax, immutable action SHAs, permissions, timeout, triggers | Any violation blocks            |
| golangci-lint and ESLint   | Security-aware static checks, type safety, resource handling           | Any configured violation blocks |
| Checksums and attestations | Six release archives, SPDX SBOM, build identity                        | Any mismatch blocks publication |

No vulnerability is silently ignored. A temporary exception requires a
dedicated issue containing the advisory, exploitability analysis, owner,
expiration date, compensating control, and removal plan. The default policy
ignores only findings that have no upstream fix.

## Workflow trust boundaries

```mermaid
flowchart LR
    Fork["Fork or pull request"] -->|read-only token| CI["CI and Security"]
    CI -->|no secrets| Evidence["Checks and annotations"]
    Tag["Protected release tag"] --> Build["Read-only build"]
    Build --> Gate["release environment"]
    Gate -->|job-scoped token + OIDC| Publish["Attestation and GitHub Release"]
    Operator["Authorized operator"] --> Promotion["staging / production environment"]
    Promotion -->|read-only release access| Verify["Checksum, provenance, scan, health"]
```

The repository forbids `pull_request_target`, inherited secrets, mutable action
tags, top-level write permissions, unbounded jobs, and unapproved job-level
write scopes. The allowlist is code-reviewed in
`scripts/ci/check-workflow-policy.mjs`. Forked pull requests cannot reach the
release or promotion workflows because both are manual/tag-only workflows and
protected environments are never entered from pull request events.

## Credentials and data

- Do not commit tokens, `.env` files, user data, API payload dumps, or private
  repository content.
- The workflow-provided `GITHUB_TOKEN` is job-scoped and receives only
  permissions listed in each workflow.
- A future hosting provider must use environment-scoped OIDC federation.
- The runtime GitHub token required for GraphQL issue search is accepted only
  by the backend process and must never be sent to browser code, logs, errors,
  analytics, cache keys, or fixtures.
- GitHub OAuth uses Authorization Code with PKCE `S256`, a random single-use
  state digest, an exact callback URL, and the explicit minimum `read:user`
  scope. The exchanged access token is used only for `GET /user`, then
  discarded without database, browser, cache, analytics, or log exposure.
- The OAuth flow cookie is AES-256-GCM authenticated, short-lived, versioned,
  HttpOnly, and paired with an atomically consumed database state hash.
  Callback query state is compared in constant time before any code exchange.
- Session and CSRF credentials contain 256 random bits. Only SHA-256 digests
  are persisted. HTTPS deployment cookies use `Secure`, `HttpOnly`,
  `SameSite=Lax`, `Path=/`, no Domain, and the `__Host-` prefix.
- Authenticated mutations require the session cookie, CSRF cookie, active
  server digest, and constant-time matching `X-CSRF-Token` header. Refresh
  atomically revokes the old session and rotates both browser credentials.
- OAuth return paths accept only bounded root-relative values. Callback
  redirects are assembled from the fixed `AUTH_FRONTEND_URL`; provider
  descriptions and caller-supplied origins are never reflected.
- Credentialed CORS echoes only an exact configured origin. Reverse-proxy
  forwarding headers are ignored unless `TRUSTED_PROXY_CIDRS` explicitly
  names the actual ingress.
- Public profile GraphQL queries filter owned, forked, and contributed
  repositories to `PUBLIC`; remove non-public star/contribution nodes; and
  never request restricted-contribution totals. Anonymous profile analysis is
  database-free and stores only a bounded in-memory cache entry.
- Logs use request IDs and bounded metadata; authorization headers and upstream
  response bodies are not logged.
- PostgreSQL is optional, TLS-only, pool-bounded, and reserved for authenticated
  account data. Anonymous handlers have no database repository dependency.
- Account routes derive ownership only from the server-validated principal.
  Every feature query includes `account_id`; a foreign resource UUID is masked
  as not found. Mutations require CSRF and optimistic versions, quota writes
  serialize per account, and stored bookmarks contain only normalized
  references rather than GitHub payloads.
- Privacy export excludes credentials, audit identifiers, upstream payloads,
  and anonymous activity. Confirmed account deletion cascades through all
  owned data and leaves only a content-free event without an account ID.
- Database URLs use redacting configuration values and never enter errors,
  fixtures, frontend assets, migration output, or release artifacts.
- Release archives reject `.env`, source-map, and private-key files plus
  credential-shaped GitHub, PostgreSQL/Neon, and AWS content.

The complete authentication threat model and lifecycle are in
[Optional GitHub authentication](authentication.md).
Account-specific ownership, quota, stale-reference, export, and deletion rules
are in [Authenticated account workspace](account-workspace.md).

## Incident response

For a suspected credential leak, malicious dependency, compromised action, or
release integrity failure:

1. disable the affected workflow or environment;
2. revoke the credential or trust relationship;
3. preserve the run, checksums, attestations, SBOM, dependency graph, and logs;
4. identify all releases and environments containing the affected digest;
5. roll back using a previously attested release;
6. patch or pin the dependency/action and run the complete CI and Security
   workflows;
7. document timeline, impact, root cause, containment, recovery, and preventive
   actions in an incident issue;
8. rotate credentials again if their exposure window cannot be proven closed.

Do not delete failed evidence during an investigation. Release inputs are kept
for 14 days and promotion evidence for 90 days; GitHub Releases and
attestations remain the long-term record.
