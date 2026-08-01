# Known limitations and extension seams

IssueScout is intentionally conservative. These limits prevent an explainable
recommendation tool from implying certainty, collecting unnecessary data, or
creating unbounded GitHub work.

## Product limitations

- A recommendation is evidence for choosing work, not proof that the issue is
  still available or that a maintainer will accept a contribution.
- Difficulty and effort are deterministic estimates, not schedules.
- Claim detection sees only a bounded public comment sample and can miss work
  coordinated elsewhere.
- Maintainer response, review, merge, activity, CI, and contribution metrics
  are bounded samples with explicit status and confidence.
- Technology inference supports a documented manifest/keyword rule set. It
  does not compile code, clone repositories, execute dependency resolvers, or
  infer arbitrary frameworks.
- GitHub Search can report a larger upstream total than the at-most-50
  candidate window. Pagination is over eligible candidates in that window,
  not all GitHub matches.
- Deleted users, missing nullable fields, organization accounts, partial
  GraphQL results, and rate limits can make evidence unavailable.
- Anonymous cache data disappears on restart and is not shared between API
  instances.

The UI must keep showing estimate, sampled, truncated, unavailable, and warning
semantics. Do not replace them with confident prose.

## Delivered optional capabilities

The original MVP listed OAuth, database persistence, and bookmarks as future
work. They are now delivered behind a separate optional boundary:

- GitHub OAuth uses Authorization Code + PKCE and only `read:user`;
- PostgreSQL stores minimum public identity and hashed session/state material;
- bookmarks and saved searches store normalized references/filters, not
  GitHub payloads;
- preferences, bounded privacy export, and confirmed account deletion are
  supported.

These capabilities require deliberate production configuration. They do not
improve anonymous analysis accuracy and never run on public routes.

## Not implemented

| Capability                          | Why it remains out of scope                                                            | Extension seam                                                       |
| ----------------------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| AI issue analysis                   | Explainability, prompt injection, privacy, cost, and evaluation need a separate design | New analysis port with deterministic fallback and versioned evidence |
| Notifications                       | Requires consent, delivery provider, scheduling, retries, and unsubscribe semantics    | Account-owned notification port and worker                           |
| Automatic issue claiming/commenting | Material external mutation and maintainer etiquette risk                               | Explicit user-authorized GitHub write integration, never implicit    |
| Pull-request creation               | Requires repository write scopes and code generation security                          | Separate product/workflow with narrow installation permissions       |
| Background refresh                  | Requires queue, scheduler, idempotency, quotas, and stale-data rules                   | Provider-neutral job port                                            |
| Shared/team workspaces              | Ownership and authorization model is currently one account                             | Organization/team domain and row-level authorization                 |
| GitLab or other forges              | Domain is normalized but adapters/contracts remain GitHub-specific                     | Forge capability port plus source discriminator                      |
| Distributed cache                   | Current process-local behavior is simpler and privacy-preserving                       | Cache port adapter with TTL, capacity, encryption, and tenant review |
| Full-text/code analysis             | Cloning or executing untrusted repositories is excluded                                | Isolated static-analysis service with strict resource sandbox        |

## OAuth and Neon extension rules

If authentication gains more GitHub scopes:

1. open a dedicated security/product issue;
2. explain each scope and data retention purpose;
3. update consent, privacy export, deletion, threat model, and fixtures;
4. never reuse the short-lived identity token as a general GitHub credential;
5. preserve an OAuth-disabled zero-database startup.

If PostgreSQL gains a new account feature:

1. define account ownership and quota in domain/application code;
2. add a port before the PostgreSQL implementation;
3. create an append-only migration;
4. parameterize every query and mask foreign UUIDs as not found;
5. add export/deletion treatment;
6. prove anonymous routes still make zero database calls.

Neon is a compatible provider, not a domain dependency. No provider type,
branch identifier, or connection string belongs outside configuration and the
PostgreSQL adapter.

## Bookmark and collaboration evolution

Bookmark records deliberately contain only target type, repository owner/name,
optional issue number, timestamps, and optimistic version. To add notes, tags,
collections, or collaboration:

- define maximum lengths/counts and normalization;
- decide whether content enters privacy export and deletion;
- define conflict and version behavior;
- prevent notes from becoming a copy of untrusted issue bodies;
- preserve per-account ownership on every query;
- add accessible empty, partial, conflict, and recovery states.

## AI extension safety bar

An AI feature must not silently replace deterministic scoring. A future design
must include:

- an explicit opt-in and clear model/provider disclosure;
- bounded, redacted, public-only inputs;
- prompt-injection treatment for issue/repository text;
- no secret, Cookie, database value, or private repository input;
- latency, cost, rate, and output-size ceilings;
- versioned prompts/models and reproducible evaluation fixtures;
- typed citations back to evidence;
- safe timeout/failure fallback to the current rule engine;
- deletion/retention and regional processing decisions.

Until those conditions are satisfied, AI-generated difficulty, effort, or
recommendation claims are not part of IssueScout.

## Operational limitations

- The repository produces verified native archives but has no selected hosting
  provider adapter. The Deploy workflow validates promotion evidence and
  health after an external handoff; it does not pretend to deploy.
- Service metrics are not emitted by the application. Platforms may derive
  low-cardinality metrics from structured logs and route templates.
- There is no cross-instance cache coherence.
- Live performance depends on GitHub and the chosen host even though local
  production-shaped targets are executable.
- Pull-request dependency and vulnerability policies can block a release until
  an upstream fix or reviewed time-bounded exception exists.

Track a limitation change through an English issue with scope, acceptance
criteria, threat/performance impact, test plan, and documentation impact.
